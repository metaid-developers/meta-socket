package mvc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// buildMetaIDWitnessTx creates a mock SegWit transaction with MetaID witness data.
// The witness stack encodes: OP_FALSE OP_IF "metaid" "init" "/" "0" "0" "text/plain" "hello"
func buildMetaIDWitnessTx(t *testing.T) *wire.MsgTx {
	t.Helper()

	// Build a witness script: OP_FALSE OP_IF <protocol_id> OP_ENDIF
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_FALSE)
	builder.AddOp(txscript.OP_IF)
	builder.AddData([]byte("metaid"))
	builder.AddOp(txscript.OP_ENDIF)

	witnessScript, err := builder.Script()
	if err != nil {
		t.Fatalf("failed to build witness script: %v", err)
	}

	// Build a valid P2WSH output script (OP_0 + SHA256 of witness script).
	witnessProg := sha256Sum(witnessScript)
	pkScript := make([]byte, 2+len(witnessProg))
	pkScript[0] = txscript.OP_0
	pkScript[1] = byte(len(witnessProg))
	copy(pkScript[2:], witnessProg)

	tx := wire.NewMsgTx(2)

	outpoint := wire.NewOutPoint(&chainhash.Hash{}, 0)
	txIn := wire.NewTxIn(outpoint, nil, nil)
	txIn.Sequence = wire.MaxTxInSequenceNum
	tx.AddTxIn(txIn)

	witnessData := [][]byte{
		[]byte("dummy-sig"),
		[]byte("dummy-pubkey"),
		[]byte("metaid"),
		[]byte("init"),
		[]byte("/"),
		[]byte("0"),
		[]byte("0"),
		[]byte("text/plain"),
		[]byte("hello metaid world"),
	}
	fullWitness := append([][]byte{witnessScript}, witnessData...)
	tx.TxIn[0].Witness = fullWitness

	txOut := wire.NewTxOut(0, pkScript)
	tx.AddTxOut(txOut)

	return tx
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func TestCatchPinsByTx_BasicMetaID_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	tx := buildMetaIDWitnessTx(t)

	pins := idx.catchPinsByTx(tx, 100, 1234567890,
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		0)

	if len(pins) == 0 {
		t.Fatal("catchPinsByTx returned no pins for MetaID witness transaction")
	}

	pin := pins[0]
	if pin.ChainName != "mvc" {
		t.Errorf("expected chainName 'mvc', got %q", pin.ChainName)
	}
	if pin.Operation == "" {
		t.Error("expected non-empty operation")
	}
	if pin.Path == "" {
		t.Error("expected non-empty path")
	}
	if pin.ContentBody == nil || len(pin.ContentBody) == 0 {
		t.Error("expected non-empty content body")
	}
	t.Logf("chain=%s op=%s path=%q body=%q",
		pin.ChainName, pin.Operation, pin.Path, string(pin.ContentBody))
}

func TestCatchPinsByTx_InfoNamePin_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_FALSE)
	builder.AddOp(txscript.OP_IF)
	builder.AddData([]byte("metaid"))
	builder.AddOp(txscript.OP_ENDIF)
	witnessScript, _ := builder.Script()

	tx := wire.NewMsgTx(2)
	outpoint := wire.NewOutPoint(&chainhash.Hash{}, 0)
	txIn := wire.NewTxIn(outpoint, nil, nil)
	txIn.Sequence = wire.MaxTxInSequenceNum
	tx.AddTxIn(txIn)

	witnessData := [][]byte{
		[]byte("sig"), []byte("pubkey"),
		[]byte("metaid"),
		[]byte("create"),
		[]byte("/info/name"),
		[]byte("0"), []byte("0"),
		[]byte("text/plain"),
		[]byte("Alice"),
	}
	tx.TxIn[0].Witness = append([][]byte{witnessScript}, witnessData...)

	witnessProg := sha256Sum(witnessScript)
	pkScript := make([]byte, 2+len(witnessProg))
	pkScript[0] = txscript.OP_0
	pkScript[1] = byte(len(witnessProg))
	copy(pkScript[2:], witnessProg)
	tx.AddTxOut(wire.NewTxOut(0, pkScript))

	pins := idx.catchPinsByTx(tx, 200, 1234567890, "00", "00", 0)

	if len(pins) == 0 {
		t.Fatal("catchPinsByTx returned no pins for /info/name path")
	}

	pin := pins[0]
	if pin.Path != "/info/name" {
		t.Errorf("expected path '/info/name', got %q", pin.Path)
	}
	if pin.Operation != "create" {
		t.Errorf("expected operation 'create', got %q", pin.Operation)
	}
	if string(pin.ContentBody) != "Alice" {
		t.Errorf("expected contentBody 'Alice', got %q", string(pin.ContentBody))
	}
	t.Logf("path=%s op=%s body=%s", pin.Path, pin.Operation, string(pin.ContentBody))
}

func TestCatchPinsByTx_NonWitness_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	tx := wire.NewMsgTx(1)
	outpoint := wire.NewOutPoint(&chainhash.Hash{}, 0)
	tx.AddTxIn(wire.NewTxIn(outpoint, nil, nil))
	tx.AddTxOut(wire.NewTxOut(0, []byte{}))

	pins := idx.catchPinsByTx(tx, 100, 1234567890, "00", "00", 0)
	if len(pins) != 0 {
		t.Errorf("expected 0 pins for non-witness tx, got %d", len(pins))
	}
}

func TestCatchPinsByTx_OPReturnUsesMANHash_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	tx := wire.NewMsgTx(10)
	outpoint := wire.NewOutPoint(&chainhash.Hash{}, 0)
	txIn := wire.NewTxIn(outpoint, []byte{txscript.OP_TRUE}, nil)
	txIn.Sequence = wire.MaxTxInSequenceNum
	tx.AddTxIn(txIn)

	addr, err := btcutil.NewAddressPubKeyHash(bytes.Repeat([]byte{0x11}, 20), &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	ownerScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}
	tx.AddTxOut(wire.NewTxOut(1, ownerScript))

	metaScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("metaid")).
		AddData([]byte("create")).
		AddData([]byte("/protocols/skill-service")).
		AddData([]byte("0")).
		AddData([]byte("1.0.0")).
		AddData([]byte("application/json")).
		AddData([]byte(`{"serviceName":"music-search-service","displayName":"帮你搜索Web2 音乐"}`)).
		Script()
	if err != nil {
		t.Fatal(err)
	}
	tx.AddTxOut(wire.NewTxOut(0, metaScript))

	legacyHash := tx.TxHash().String()
	expectedHash := manStyleMVCNewHash(t, tx)
	if expectedHash == legacyHash {
		t.Fatalf("test transaction should expose MAN hash difference; both were %s", expectedHash)
	}

	pins := idx.catchPinsByTx(tx, 167270, 1775180512, "00", "00", 14)
	if len(pins) != 1 {
		t.Fatalf("expected 1 OP_RETURN pin, got %d", len(pins))
	}
	pin := pins[0]
	if pin.GenesisTransaction != expectedHash {
		t.Fatalf("genesisTransaction: got %s want MAN hash %s (legacy %s)",
			pin.GenesisTransaction, expectedHash, legacyHash)
	}
	if pin.Id != expectedHash+"i0" {
		t.Fatalf("id: got %s want %si0", pin.Id, expectedHash)
	}
}

func TestCatchPinsByTx_EmptyWitness_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	tx := wire.NewMsgTx(2)
	outpoint := wire.NewOutPoint(&chainhash.Hash{}, 0)
	txIn := wire.NewTxIn(outpoint, nil, nil)
	txIn.Sequence = wire.MaxTxInSequenceNum
	txIn.Witness = [][]byte{}
	tx.AddTxIn(txIn)
	tx.AddTxOut(wire.NewTxOut(0, []byte{}))

	pins := idx.catchPinsByTx(tx, 100, 1234567890, "00", "00", 0)
	if len(pins) != 0 {
		t.Errorf("expected 0 pins for empty witness, got %d", len(pins))
	}
}

func manStyleMVCNewHash(t *testing.T, tx *wire.MsgTx) string {
	t.Helper()

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	if len(raw) < 10 {
		t.Fatalf("raw transaction too short: %d", len(raw))
	}

	index := 0
	version := raw[index : index+4]
	index += 4

	vinCount, n := decodeMANVarIntForTest(t, raw[index:])
	index += n
	if vinCount == 0 {
		t.Fatal("test transaction has no inputs")
	}

	type txInParts struct {
		txid      []byte
		vout      []byte
		scriptSig []byte
		sequence  []byte
	}
	inputs := make([]txInParts, 0, vinCount)
	for i := 0; i < vinCount; i++ {
		if index+36 > len(raw) {
			t.Fatal("truncated input")
		}
		in := txInParts{
			txid: raw[index : index+32],
			vout: raw[index+32 : index+36],
		}
		index += 36
		scriptLen, size := decodeMANVarIntForTest(t, raw[index:])
		index += size
		if index+scriptLen+4 > len(raw) {
			t.Fatal("truncated input script")
		}
		in.scriptSig = raw[index : index+scriptLen]
		index += scriptLen
		in.sequence = raw[index : index+4]
		index += 4
		inputs = append(inputs, in)
	}

	voutCount, n := decodeMANVarIntForTest(t, raw[index:])
	index += n
	if voutCount == 0 {
		t.Fatal("test transaction has no outputs")
	}

	type txOutParts struct {
		amount []byte
		script []byte
	}
	outputs := make([]txOutParts, 0, voutCount)
	for i := 0; i < voutCount; i++ {
		if index+8 > len(raw) {
			t.Fatal("truncated output")
		}
		out := txOutParts{amount: raw[index : index+8]}
		index += 8
		scriptLen, size := decodeMANVarIntForTest(t, raw[index:])
		index += size
		if index+scriptLen > len(raw) {
			t.Fatal("truncated output script")
		}
		out.script = raw[index : index+scriptLen]
		index += scriptLen
		outputs = append(outputs, out)
	}
	if index+4 != len(raw) {
		t.Fatalf("unexpected raw transaction tail: index=%d len=%d", index, len(raw))
	}
	lockTime := raw[index : index+4]

	if binary.LittleEndian.Uint32(version) < 10 {
		return tx.TxHash().String()
	}

	var inputBytes, inputScriptHashes, outputBytes []byte
	for _, in := range inputs {
		inputBytes = append(inputBytes, in.txid...)
		inputBytes = append(inputBytes, in.vout...)
		inputBytes = append(inputBytes, in.sequence...)
		inputScriptHashes = append(inputScriptHashes, sha256Once(in.scriptSig)...)
	}
	for _, out := range outputs {
		outputBytes = append(outputBytes, out.amount...)
		outputBytes = append(outputBytes, sha256Once(out.script)...)
	}

	var manRaw []byte
	manRaw = append(manRaw, version...)
	manRaw = append(manRaw, lockTime...)
	manRaw = append(manRaw, uint32LEForTest(uint32(vinCount))...)
	manRaw = append(manRaw, uint32LEForTest(uint32(voutCount))...)
	manRaw = append(manRaw, sha256Once(inputBytes)...)
	manRaw = append(manRaw, sha256Once(inputScriptHashes)...)
	manRaw = append(manRaw, sha256Once(outputBytes)...)
	return doubleSHA256ReversedHexForTest(manRaw)
}

func decodeMANVarIntForTest(t *testing.T, buf []byte) (int, int) {
	t.Helper()
	if len(buf) == 0 {
		t.Fatal("empty varint")
	}
	switch buf[0] {
	case 0xfd:
		if len(buf) < 3 {
			t.Fatal("short uint16 varint")
		}
		return int(buf[2])*int(math.Pow(256, 1)) + int(buf[1]), 3
	case 0xfe:
		if len(buf) < 5 {
			t.Fatal("short uint32 varint")
		}
		return int(buf[4])*int(math.Pow(256, 3)) +
			int(buf[3])*int(math.Pow(256, 2)) +
			int(buf[2])*int(math.Pow(256, 1)) +
			int(buf[1]), 5
	case 0xff:
		if len(buf) < 9 {
			t.Fatal("short uint64 varint")
		}
		return int(buf[8])*int(math.Pow(256, 7)) +
			int(buf[7])*int(math.Pow(256, 6)) +
			int(buf[6])*int(math.Pow(256, 5)) +
			int(buf[5])*int(math.Pow(256, 4)) +
			int(buf[4])*int(math.Pow(256, 3)) +
			int(buf[3])*int(math.Pow(256, 2)) +
			int(buf[2])*int(math.Pow(256, 1)) +
			int(buf[1]), 9
	default:
		return int(buf[0]), 1
	}
}

func uint32LEForTest(v uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, v)
	return out
}

func sha256Once(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func doubleSHA256ReversedHexForTest(data []byte) string {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	out := make([]byte, len(second))
	copy(out, second[:])
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return hex.EncodeToString(out)
}

func TestGetAddress_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	scriptHex := "4104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac"
	scriptBytes, err := hex.DecodeString(scriptHex)
	if err != nil {
		t.Fatalf("failed to decode hex script: %v", err)
	}

	addr := idx.GetAddress(scriptBytes)
	if addr == "" {
		t.Error("GetAddress returned empty for valid P2PK script")
	}
	t.Logf("address from P2PK script: %s", addr)
}

func TestCatchPinsByTx_InitPin_MVC(t *testing.T) {
	idx := &Indexer{
		chainParams: &chaincfg.MainNetParams,
	}

	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_FALSE)
	builder.AddOp(txscript.OP_IF)
	builder.AddData([]byte("metaid"))
	builder.AddOp(txscript.OP_ENDIF)
	witnessScript, _ := builder.Script()

	tx := wire.NewMsgTx(2)
	outpoint := wire.NewOutPoint(&chainhash.Hash{}, 0)
	txIn := wire.NewTxIn(outpoint, nil, nil)
	txIn.Sequence = wire.MaxTxInSequenceNum
	tx.AddTxIn(txIn)

	witnessData := [][]byte{
		[]byte("sig"), []byte("pubkey"),
		[]byte("metaid"),
		[]byte("init"),
		[]byte("/"),
		[]byte("0"), []byte("0"),
		[]byte("text/plain"),
		[]byte(""),
	}
	tx.TxIn[0].Witness = append([][]byte{witnessScript}, witnessData...)

	witnessProg := sha256Sum(witnessScript)
	pkScript := make([]byte, 2+len(witnessProg))
	pkScript[0] = txscript.OP_0
	pkScript[1] = byte(len(witnessProg))
	copy(pkScript[2:], witnessProg)
	tx.AddTxOut(wire.NewTxOut(0, pkScript))

	pins := idx.catchPinsByTx(tx, 300, 1234567890, "00", "00", 0)
	if len(pins) == 0 {
		t.Fatal("catchPinsByTx returned no pins for init path '/'")
	}

	pin := pins[0]
	if pin.Path != "/" {
		t.Errorf("expected path '/', got %q", pin.Path)
	}
	if pin.Operation != "init" {
		t.Errorf("expected operation 'init', got %q", pin.Operation)
	}
	t.Logf("init pin: chain=%s path=%s op=%s", pin.ChainName, pin.Path, pin.Operation)
}
