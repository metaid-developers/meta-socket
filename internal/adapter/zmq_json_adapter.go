package adapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/metaid-developers/meta-socket/internal/idaddress"
	zmq "github.com/pebbe/zmq4"
)

const defaultProtocolIDHex = "6d6574616964"

var dogeMainNetParams = chaincfg.Params{
	Name:             "doge-mainnet",
	Net:              wire.BitcoinNet(0xc0c0c0c0),
	PubKeyHashAddrID: 0x1e, // D...
	ScriptHashAddrID: 0x16, // 9...
	PrivateKeyID:     0x9e,
	Bech32HRPSegwit:  "doge",
}

type JSONZMQAdapter struct {
	chain      string
	endpoint   string
	topic      string
	protocolID []byte
}

func NewJSONZMQAdapter(chain, endpoint, topic string) *JSONZMQAdapter {
	normalizedTopic := strings.TrimSpace(topic)
	if normalizedTopic == "" {
		normalizedTopic = "rawtx"
	}

	protocolID, err := hex.DecodeString(defaultProtocolIDHex)
	if err != nil || len(protocolID) == 0 {
		protocolID = []byte("metaid")
	}

	return &JSONZMQAdapter{
		chain:      strings.ToLower(strings.TrimSpace(chain)),
		endpoint:   strings.TrimSpace(endpoint),
		topic:      normalizedTopic,
		protocolID: protocolID,
	}
}

func (a *JSONZMQAdapter) Name() string {
	return a.chain
}

func (a *JSONZMQAdapter) Run(ctx context.Context, out chan<- MempoolMessage) error {
	if strings.TrimSpace(a.endpoint) == "" {
		return fmt.Errorf("chain=%s endpoint is empty", a.chain)
	}
	if out == nil {
		return errors.New("output channel is nil")
	}

	sub, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		return fmt.Errorf("chain=%s create zmq socket failed: %w", a.chain, err)
	}
	defer sub.Close()

	if err := sub.Connect(a.endpoint); err != nil {
		return fmt.Errorf("chain=%s connect zmq endpoint failed: %w", a.chain, err)
	}
	if err := sub.SetSubscribe(a.topic); err != nil {
		return fmt.Errorf("chain=%s subscribe topic failed: %w", a.chain, err)
	}
	if err := sub.SetRcvtimeo(1 * time.Second); err != nil {
		return fmt.Errorf("chain=%s set rcv timeout failed: %w", a.chain, err)
	}

	log.Printf("[ZMQ] adapter started: chain=%s endpoint=%s topic=%s", a.chain, a.endpoint, a.topic)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		parts, err := sub.RecvMessageBytes(0)
		if err != nil {
			// Timeout is expected to allow context cancellation checks.
			continue
		}
		if len(parts) == 0 {
			continue
		}

		msg, ok := a.decodeFrames(parts)
		if !ok {
			continue
		}

		select {
		case <-ctx.Done():
			return nil
		case out <- msg:
		}
	}
}

func (a *JSONZMQAdapter) decodeFrames(parts [][]byte) (MempoolMessage, bool) {
	if msg, ok := a.decodeJSONFrames(parts); ok {
		return msg, true
	}

	rawTxPayload, ok := a.pickRawTxPayload(parts)
	if !ok {
		return MempoolMessage{}, false
	}
	return a.decodeRawTxPayload(rawTxPayload)
}

func (a *JSONZMQAdapter) decodeJSONFrames(parts [][]byte) (MempoolMessage, bool) {
	// Keep compatibility with the old JSON ZMQ pipeline (`pinList + tx`).
	for i := len(parts) - 1; i >= 0; i-- {
		if msg, ok := a.decodeJSONPayload(parts[i]); ok {
			return msg, true
		}
	}
	return MempoolMessage{}, false
}

func (a *JSONZMQAdapter) decodeJSONPayload(payload []byte) (MempoolMessage, bool) {
	var msg MempoolMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return MempoolMessage{}, false
	}
	if msg.Chain == "" {
		msg.Chain = a.chain
	}
	if msg.PinList == nil {
		msg.PinList = make([]*PinRecord, 0)
	}
	return msg, true
}

func (a *JSONZMQAdapter) pickRawTxPayload(parts [][]byte) ([]byte, bool) {
	if len(parts) == 0 {
		return nil, false
	}

	normalizedTopic := strings.ToLower(strings.TrimSpace(a.topic))
	if len(parts) >= 2 {
		first := strings.ToLower(strings.TrimSpace(string(parts[0])))
		if first == normalizedTopic || first == "" {
			return parts[1], true
		}
		// Common rawtx frame shape: [topic, tx-bytes, sequence]
		return parts[1], true
	}

	single := bytes.TrimSpace(parts[0])
	if len(single) == 0 {
		return nil, false
	}
	if strings.EqualFold(string(single), normalizedTopic) {
		return nil, false
	}
	if normalizedTopic != "" {
		topicBytes := []byte(normalizedTopic)
		lower := bytes.ToLower(single)
		if bytes.HasPrefix(lower, topicBytes) {
			rest := bytes.TrimSpace(single[len(topicBytes):])
			if len(rest) == 0 {
				return nil, false
			}
			return rest, true
		}
	}
	return single, true
}

func (a *JSONZMQAdapter) decodeRawTxPayload(payload []byte) (MempoolMessage, bool) {
	if len(payload) < 10 {
		return MempoolMessage{}, false
	}

	tx := wire.NewMsgTx(1)
	if err := tx.Deserialize(bytes.NewReader(payload)); err != nil {
		return MempoolMessage{}, false
	}

	pinList := a.extractPinRecordsFromTx(tx)
	if len(pinList) == 0 {
		return MempoolMessage{}, false
	}

	return MempoolMessage{
		Chain:   a.chain,
		PinList: pinList,
	}, true
}

type parsedPin struct {
	Path        string
	ContentBody []byte
}

func (a *JSONZMQAdapter) extractPinRecordsFromTx(tx *wire.MsgTx) []*PinRecord {
	if tx == nil {
		return nil
	}

	txHash := normalizedTxHashForChain(a.chain, tx)

	pins := make([]*PinRecord, 0)

	appendPin := func(parsed *parsedPin, pinIndex int, ownerAddress string) {
		if parsed == nil {
			return
		}
		_, path := splitHostAndPath(parsed.Path)
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		if pinIndex < 0 {
			pinIndex = 0
		}

		createMetaID := metaIDFromAddress(ownerAddress)
		globalMetaID := globalMetaIDFromAddress(ownerAddress)
		if globalMetaID == "" {
			// Keep compatibility with previous fallback behavior when conversion fails.
			globalMetaID = createMetaID
		}

		pinID := fmt.Sprintf("%si%d", txHash, pinIndex)

		pins = append(pins, &PinRecord{
			ID:            pinID,
			Path:          path,
			MetaID:        createMetaID,
			CreateMetaID:  createMetaID,
			GlobalMetaID:  globalMetaID,
			CreateAddress: ownerAddress,
			ChainName:     a.chain,
			ContentBody:   json.RawMessage(parsed.ContentBody),
		})
	}

	// Legacy behavior: once OP_RETURN pin is found, it takes priority and short-circuits.
	for _, out := range tx.TxOut {
		if parsed := parseOpReturnPin(out.PkScript, a.protocolID); parsed != nil {
			ownerAddress, ownerOutIdx := resolveOpReturnOwner(a.chain, tx)
			if ownerAddress == "" {
				ownerAddress, ownerOutIdx = resolveInputOwner(a.chain, tx, 0)
			}
			appendPin(parsed, ownerOutIdx, ownerAddress)
			return pins
		}
	}

	// Legacy MVC parser only accepts OP_RETURN inscriptions.
	if strings.EqualFold(a.chain, "mvc") {
		return pins
	}

	if strings.EqualFold(a.chain, "doge") {
		// Dogecoin supports inscription payloads in ScriptSig / redeem-script flows.
		for inIdx, in := range tx.TxIn {
			if in == nil || len(in.SignatureScript) == 0 {
				continue
			}

			parsed := parseDogeDirectScriptSigPin(in.SignatureScript)
			if parsed == nil {
				parsed = parseDogeRedeemScriptPin(lastPushData(in.SignatureScript), a.protocolID)
			}
			if parsed != nil {
				ownerAddress, ownerOutIdx := resolveInputOwner(a.chain, tx, inIdx)
				appendPin(parsed, ownerOutIdx, ownerAddress)
			}
		}
	} else {
		// BTC-like witness inscription path.
		for inIdx, in := range tx.TxIn {
			if script := pickWitnessScript(in); len(script) > 0 {
				if parsed := parseWitnessPin(script, a.protocolID); parsed != nil {
					ownerAddress, ownerOutIdx := resolveInputOwner(a.chain, tx, inIdx)
					appendPin(parsed, ownerOutIdx, ownerAddress)
				}
			}
		}
	}

	return pins
}

func normalizedTxHashForChain(chain string, tx *wire.MsgTx) string {
	if tx == nil {
		return ""
	}
	if strings.EqualFold(chain, "mvc") {
		return mvcNormalizedTxHash(tx)
	}
	return tx.TxHash().String()
}

func mvcNormalizedTxHash(tx *wire.MsgTx) string {
	if tx == nil {
		return ""
	}
	value, err := mvcGetNewHash(tx)
	if err != nil {
		return tx.TxHash().String()
	}
	return value
}

func resolveOpReturnOwner(chain string, tx *wire.MsgTx) (string, int) {
	if tx == nil {
		return "", 0
	}

	params := chainAddressParams(chain)
	if strings.EqualFold(chain, "mvc") {
		// Legacy MVC op-return ownership resolves to first standard output.
		for i, out := range tx.TxOut {
			class, addresses, _, _ := txscript.ExtractPkScriptAddrs(out.PkScript, params)
			if class != txscript.NullDataTy && class != txscript.NonStandardTy && len(addresses) > 0 {
				return addresses[0].String(), i
			}
		}
		return "", 0
	}

	// Legacy BTC/DOGE op-return ownership resolves to the last standard output.
	ownerAddress := ""
	ownerIndex := 0
	for i, out := range tx.TxOut {
		class, addresses, _, _ := txscript.ExtractPkScriptAddrs(out.PkScript, params)
		if class != txscript.NonStandardTy && len(addresses) > 0 {
			ownerAddress = addresses[0].String()
			ownerIndex = i
		}
	}
	return ownerAddress, ownerIndex
}

func resolveInputOwner(chain string, tx *wire.MsgTx, inIdx int) (string, int) {
	if tx == nil || len(tx.TxOut) == 0 {
		return "", 0
	}

	params := chainAddressParams(chain)
	if len(tx.TxIn) == 1 || len(tx.TxOut) == 1 || inIdx <= 0 {
		if address := extractAddress(tx.TxOut[0].PkScript, params); address != "" {
			return address, 0
		}
		return "", 0
	}

	// Legacy GetPinOwner depends on prevout values; here we keep deterministic index-aware fallback.
	candidate := inIdx
	if candidate >= len(tx.TxOut) {
		candidate = len(tx.TxOut) - 1
	}
	if address := extractAddress(tx.TxOut[candidate].PkScript, params); address != "" {
		return address, candidate
	}

	for i, out := range tx.TxOut {
		if address := extractAddress(out.PkScript, params); address != "" {
			return address, i
		}
	}
	return "", 0
}

func extractAddress(pkScript []byte, params *chaincfg.Params) string {
	_, addresses, _, _ := txscript.ExtractPkScriptAddrs(pkScript, params)
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0].String()
}

func chainAddressParams(chain string) *chaincfg.Params {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "doge":
		return &dogeMainNetParams
	default:
		// BTC and MVC both currently use bitcoin-compatible address semantics.
		return &chaincfg.MainNetParams
	}
}

func metaIDFromAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(address))
	return hex.EncodeToString(sum[:])
}

func globalMetaIDFromAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	value, err := idaddress.ConvertFromBitcoin(address)
	if err != nil {
		return ""
	}
	return value
}

func splitHostAndPath(raw string) (string, string) {
	items := strings.SplitN(raw, ":", 2)
	if len(items) == 2 {
		return items[0], items[1]
	}
	return "", raw
}

func pickWitnessScript(input *wire.TxIn) []byte {
	if input == nil || len(input.Witness) <= 1 {
		return nil
	}
	last := input.Witness[len(input.Witness)-1]
	if len(last) > 0 && last[0] == txscript.TaprootAnnexTag {
		return last
	}
	return input.Witness[len(input.Witness)-2]
}

func lastPushData(script []byte) []byte {
	if len(script) == 0 {
		return nil
	}
	tokenizer := txscript.MakeScriptTokenizer(0, script)
	var last []byte
	for tokenizer.Next() {
		data := tokenizer.Data()
		if len(data) == 0 {
			continue
		}
		last = data
	}
	if tokenizer.Err() != nil || len(last) == 0 {
		return nil
	}
	item := make([]byte, len(last))
	copy(item, last)
	return item
}

func parseOpReturnPin(script []byte, protocolID []byte) *parsedPin {
	tokenizer := txscript.MakeScriptTokenizer(0, script)
	for tokenizer.Next() {
		if tokenizer.Opcode() != txscript.OP_RETURN {
			continue
		}
		if !tokenizer.Next() {
			return nil
		}
		if !matchProtocol(tokenizer.Data(), protocolID) {
			continue
		}

		fields, ok := collectPinFields(&tokenizer, false)
		if !ok {
			return nil
		}
		return buildParsedPin(fields)
	}
	return nil
}

func parseWitnessPin(script []byte, protocolID []byte) *parsedPin {
	tokenizer := txscript.MakeScriptTokenizer(0, script)
	for tokenizer.Next() {
		if tokenizer.Opcode() != txscript.OP_FALSE {
			continue
		}
		if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_IF {
			return nil
		}
		if !tokenizer.Next() || !matchProtocol(tokenizer.Data(), protocolID) {
			return nil
		}

		fields, ok := collectPinFields(&tokenizer, true)
		if !ok {
			return nil
		}
		return buildParsedPin(fields)
	}
	return nil
}

func parseDogeRedeemScriptPin(redeemScript []byte, protocolID []byte) *parsedPin {
	if len(redeemScript) == 0 {
		return nil
	}
	tokenizer := txscript.MakeScriptTokenizer(0, redeemScript)

	// Expected prefix: <pubkey> OP_CHECKSIGVERIFY OP_FALSE OP_IF
	if !tokenizer.Next() {
		return nil
	}
	if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_CHECKSIGVERIFY {
		return nil
	}
	if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_FALSE {
		return nil
	}
	if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_IF {
		return nil
	}
	if !tokenizer.Next() || !matchProtocol(tokenizer.Data(), protocolID) {
		return nil
	}

	fields, ok := collectPinFields(&tokenizer, true)
	if !ok {
		return nil
	}
	return buildParsedPin(fields)
}

func parseDogeDirectScriptSigPin(scriptSig []byte) *parsedPin {
	if len(scriptSig) < 7 {
		return nil
	}

	tokenizer := txscript.MakeScriptTokenizer(0, scriptSig)
	infoList := make([][]byte, 0, 10)
	for tokenizer.Next() {
		data := tokenizer.Data()
		if len(data) == 0 {
			continue
		}
		item := make([]byte, len(data))
		copy(item, data)
		infoList = append(infoList, item)
	}
	if tokenizer.Err() != nil {
		return nil
	}
	if len(infoList) < 6 {
		return nil
	}
	if !strings.EqualFold(string(infoList[0]), "metaid") {
		return nil
	}

	operation := strings.ToLower(string(infoList[1]))
	if operation == "init" {
		return &parsedPin{Path: "/"}
	}
	if operation != "create" && operation != "modify" && operation != "revoke" {
		return nil
	}
	if operation != "revoke" && len(infoList) < 7 {
		return nil
	}

	path := "/info"
	if len(infoList) > 5 {
		field := strings.TrimSpace(string(infoList[5]))
		if field != "" {
			parts := strings.SplitN(field, ":", 2)
			if len(parts) == 2 {
				path = parts[1]
			} else {
				path = field
			}
		}
	}

	body := make([]byte, 0, 128)
	for i := 6; i < len(infoList); i++ {
		data := infoList[i]
		// Signature and pubkey tails are not part of content body.
		if len(data) >= 70 && len(data) <= 73 && data[0] == 0x30 {
			break
		}
		if (len(data) == 33 || len(data) == 65) && (data[0] == 0x02 || data[0] == 0x03 || data[0] == 0x04) {
			break
		}
		body = append(body, data...)
	}

	return &parsedPin{
		Path:        strings.ToLower(strings.TrimSpace(path)),
		ContentBody: body,
	}
}

func collectPinFields(tokenizer *txscript.ScriptTokenizer, stopAtEndIf bool) ([][]byte, bool) {
	fields := make([][]byte, 0, 8)
	for tokenizer.Next() {
		if stopAtEndIf && tokenizer.Opcode() == txscript.OP_ENDIF {
			return fields, true
		}

		data := tokenizer.Data()
		if len(data) > 520 {
			return nil, false
		}
		item := make([]byte, len(data))
		copy(item, data)
		fields = append(fields, item)
	}

	if tokenizer.Err() != nil {
		return nil, false
	}
	return fields, !stopAtEndIf
}

func buildParsedPin(fields [][]byte) *parsedPin {
	if len(fields) < 1 {
		return nil
	}

	operation := strings.ToLower(string(fields[0]))
	if operation == "init" {
		return &parsedPin{Path: "/"}
	}
	if operation != "revoke" && len(fields) < 6 {
		return nil
	}
	if operation == "revoke" && len(fields) < 5 {
		return nil
	}

	path := strings.ToLower(strings.TrimSpace(string(fields[1])))
	if path == "" {
		return nil
	}

	body := make([]byte, 0, 128)
	for i := 5; i < len(fields); i++ {
		body = append(body, fields[i]...)
	}
	return &parsedPin{
		Path:        path,
		ContentBody: body,
	}
}

func matchProtocol(data []byte, protocolID []byte) bool {
	if len(data) == 0 {
		return false
	}
	if len(protocolID) > 0 && bytes.EqualFold(data, protocolID) {
		return true
	}
	return strings.EqualFold(string(data), "metaid")
}
