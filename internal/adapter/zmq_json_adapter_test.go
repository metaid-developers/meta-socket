package adapter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestJSONZMQAdapterDecodeFramesJSON(t *testing.T) {
	adapter := NewJSONZMQAdapter("mvc", "tcp://127.0.0.1:15555", "rawtx")

	raw := []byte(`{"chain":"mvc","pinList":[{"id":"p1","path":"/protocols/simplemsg","metaid":"m1"}]}`)
	msg, ok := adapter.decodeFrames([][]byte{raw})
	if !ok {
		t.Fatalf("expected json frame to decode")
	}
	if msg.Chain != "mvc" {
		t.Fatalf("expected chain mvc, got %s", msg.Chain)
	}
	if len(msg.PinList) != 1 {
		t.Fatalf("expected one pin, got %d", len(msg.PinList))
	}
	if msg.PinList[0].Path != "/protocols/simplemsg" {
		t.Fatalf("unexpected path: %s", msg.PinList[0].Path)
	}
}

func TestJSONZMQAdapterDecodeFramesRawTx(t *testing.T) {
	adapter := NewJSONZMQAdapter("mvc", "tcp://127.0.0.1:15555", "rawtx")

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{})

	payload := map[string]interface{}{
		"groupId": "g1",
		"content": "hello",
		"metaId":  "meta_sender",
	}
	contentBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}

	pinScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("metaid")).
		AddData([]byte("create")).
		AddData([]byte("/protocols/simplegroupchat")).
		AddData([]byte("0")).
		AddData([]byte("0")).
		AddData([]byte("application/json")).
		AddData(contentBody).
		Script()
	if err != nil {
		t.Fatalf("build op_return script: %v", err)
	}
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: pinScript})

	var rawTx []byte
	{
		buf := make([]byte, 0, 512)
		writer := makeByteSliceWriter(&buf)
		if err := tx.Serialize(writer); err != nil {
			t.Fatalf("serialize tx: %v", err)
		}
		rawTx = buf
	}

	msg, ok := adapter.decodeFrames([][]byte{
		[]byte("rawtx"),
		rawTx,
		[]byte{0x01, 0x00, 0x00, 0x00},
	})
	if !ok {
		t.Fatalf("expected rawtx frames to decode")
	}
	if msg.Chain != "mvc" {
		t.Fatalf("expected chain mvc, got %s", msg.Chain)
	}
	if len(msg.PinList) != 1 {
		t.Fatalf("expected one pin from rawtx, got %d", len(msg.PinList))
	}
	if msg.PinList[0].Path != "/protocols/simplegroupchat" {
		t.Fatalf("unexpected pin path: %s", msg.PinList[0].Path)
	}
	if len(msg.PinList[0].ContentBody) == 0 {
		t.Fatalf("expected content body to be set")
	}
}

func TestJSONZMQAdapterDecodeFramesRawTxDogeDirectScriptSig(t *testing.T) {
	adapter := NewJSONZMQAdapter("doge", "tcp://127.0.0.1:15533", "rawtx")

	tx := wire.NewMsgTx(1)
	directScript, err := txscript.NewScriptBuilder().
		AddData([]byte("metaid")).
		AddData([]byte("create")).
		AddData([]byte("text/plain")).
		AddData([]byte("0")).
		AddData([]byte("0")).
		AddData([]byte("doge_sender:/protocols/simplemsg")).
		AddData([]byte("hello_doge")).
		Script()
	if err != nil {
		t.Fatalf("build doge direct script: %v", err)
	}
	tx.AddTxIn(&wire.TxIn{SignatureScript: directScript})

	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		PkScript: mustP2PKHScriptFromHash(t, bytes.Repeat([]byte{0x11}, 20), chainAddressParams("doge")),
	})

	msg, ok := adapter.decodeFrames([][]byte{
		[]byte("rawtx"),
		mustSerializeTx(t, tx),
		[]byte{0x01, 0x00, 0x00, 0x00},
	})
	if !ok {
		t.Fatalf("expected doge direct scriptsig frame to decode")
	}
	if len(msg.PinList) != 1 {
		t.Fatalf("expected one pin, got %d", len(msg.PinList))
	}
	if msg.PinList[0].Path != "/protocols/simplemsg" {
		t.Fatalf("unexpected doge path: %s", msg.PinList[0].Path)
	}
	if !strings.HasPrefix(msg.PinList[0].CreateAddress, "D") {
		t.Fatalf("expected doge address prefix D, got %s", msg.PinList[0].CreateAddress)
	}
}

func TestJSONZMQAdapterDecodeFramesRawTxDogeRedeemScript(t *testing.T) {
	adapter := NewJSONZMQAdapter("doge", "tcp://127.0.0.1:15533", "rawtx")

	tx := wire.NewMsgTx(1)
	redeemScript, err := txscript.NewScriptBuilder().
		AddData(bytes.Repeat([]byte{0x02}, 33)).
		AddOp(txscript.OP_CHECKSIGVERIFY).
		AddOp(txscript.OP_FALSE).
		AddOp(txscript.OP_IF).
		AddData([]byte("metaid")).
		AddData([]byte("create")).
		AddData([]byte("/protocols/simplegroupchat")).
		AddData([]byte("0")).
		AddData([]byte("0")).
		AddData([]byte("application/json")).
		AddData([]byte(`{"groupId":"g1","content":"hello"}`)).
		AddOp(txscript.OP_ENDIF).
		Script()
	if err != nil {
		t.Fatalf("build redeem script: %v", err)
	}

	signatureScript, err := txscript.NewScriptBuilder().
		AddData(bytes.Repeat([]byte{0x30}, 71)).
		AddData(redeemScript).
		Script()
	if err != nil {
		t.Fatalf("build signature script: %v", err)
	}
	tx.AddTxIn(&wire.TxIn{SignatureScript: signatureScript})
	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		PkScript: mustP2PKHScriptFromHash(t, bytes.Repeat([]byte{0x22}, 20), chainAddressParams("doge")),
	})

	msg, ok := adapter.decodeFrames([][]byte{
		[]byte("rawtx"),
		mustSerializeTx(t, tx),
		[]byte{0x01, 0x00, 0x00, 0x00},
	})
	if !ok {
		t.Fatalf("expected doge redeem scriptsig frame to decode")
	}
	if len(msg.PinList) != 1 {
		t.Fatalf("expected one pin, got %d", len(msg.PinList))
	}
	if msg.PinList[0].Path != "/protocols/simplegroupchat" {
		t.Fatalf("unexpected doge path: %s", msg.PinList[0].Path)
	}
}

func TestMVCNormalizedTxHashVersion10MatchesLegacyRule(t *testing.T) {
	tx := wire.NewMsgTx(10)
	inHash1, err := chainhash.NewHashFromStr("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	if err != nil {
		t.Fatalf("parse hash1: %v", err)
	}
	inHash2, err := chainhash.NewHashFromStr("202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f")
	if err != nil {
		t.Fatalf("parse hash2: %v", err)
	}
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *inHash1, Index: 1},
		SignatureScript:  []byte{0x51, 0x21, 0x02, 0xab, 0xcd},
		Sequence:         0xfffffffe,
	})
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *inHash2, Index: 2},
		SignatureScript:  []byte{0x00, 0x63, 0x6d, 0x65, 0x74, 0x61, 0x69, 0x64},
		Sequence:         0xfffffffd,
	})
	tx.AddTxOut(&wire.TxOut{Value: 12345, PkScript: []byte{0x51}})
	tx.AddTxOut(&wire.TxOut{Value: 67890, PkScript: []byte{0x6a, 0x02, 0x6d, 0x65}})
	tx.LockTime = 9

	got := normalizedTxHashForChain("mvc", tx)
	const legacyWant = "696850ef759535843631d317d655b225a5d4f0cdcb25f32a3b02a4dea2e67b2f"
	if got != legacyWant {
		t.Fatalf("mvc normalized hash mismatch: got=%s want=%s", got, legacyWant)
	}
	if got == tx.TxHash().String() {
		t.Fatalf("expected mvc normalized hash to differ from standard tx hash for version>=10")
	}
}

func TestJSONZMQAdapterRawTxOpReturnPriorityShortCircuit(t *testing.T) {
	adapter := NewJSONZMQAdapter("btc", "tcp://127.0.0.1:15555", "rawtx")

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		Witness: wire.TxWitness{
			[]byte{0x01},
			mustWitnessPinScript(t, "/protocols/simplemsg", []byte(`{"from":"m1","to":"m2","content":"hi"}`)),
		},
	})

	opReturnScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("metaid")).
		AddData([]byte("create")).
		AddData([]byte("/protocols/simplegroupchat")).
		AddData([]byte("0")).
		AddData([]byte("0")).
		AddData([]byte("application/json")).
		AddData([]byte(`{"groupId":"g1","content":"from-op-return"}`)).
		Script()
	if err != nil {
		t.Fatalf("build op_return: %v", err)
	}
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: opReturnScript})
	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		PkScript: mustP2PKHScriptFromHash(t, bytes.Repeat([]byte{0x31}, 20), chainAddressParams("btc")),
	})

	msg, ok := adapter.decodeFrames([][]byte{
		[]byte("rawtx"),
		mustSerializeTx(t, tx),
		[]byte{0x01, 0x00, 0x00, 0x00},
	})
	if !ok {
		t.Fatalf("expected frame to decode")
	}
	if len(msg.PinList) != 1 {
		t.Fatalf("expected one pin due to op_return short-circuit, got %d", len(msg.PinList))
	}
	pin := msg.PinList[0]
	if pin.Path != "/protocols/simplegroupchat" {
		t.Fatalf("expected op_return pin path, got %s", pin.Path)
	}
	if !strings.HasSuffix(pin.ID, "i1") {
		t.Fatalf("expected op_return owner outIdx to be used in pin id, got %s", pin.ID)
	}
	if !strings.HasPrefix(pin.GlobalMetaID, "id") {
		t.Fatalf("expected converted globalMetaId, got %s", pin.GlobalMetaID)
	}
}

func TestResolveOpReturnOwnerCompatibilityRules(t *testing.T) {
	tx := wire.NewMsgTx(1)
	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		PkScript: mustP2PKHScriptFromHash(t, bytes.Repeat([]byte{0x11}, 20), chainAddressParams("btc")),
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    2000,
		PkScript: mustP2PKHScriptFromHash(t, bytes.Repeat([]byte{0x22}, 20), chainAddressParams("btc")),
	})

	btcOwner, btcIdx := resolveOpReturnOwner("btc", tx)
	if btcOwner == "" || btcIdx != 1 {
		t.Fatalf("expected btc op_return owner from last standard output, got owner=%s idx=%d", btcOwner, btcIdx)
	}

	mvcOwner, mvcIdx := resolveOpReturnOwner("mvc", tx)
	if mvcOwner == "" || mvcIdx != 0 {
		t.Fatalf("expected mvc op_return owner from first standard output, got owner=%s idx=%d", mvcOwner, mvcIdx)
	}
}

type byteSliceWriter struct {
	dst *[]byte
}

func makeByteSliceWriter(dst *[]byte) *byteSliceWriter {
	return &byteSliceWriter{dst: dst}
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	*w.dst = append(*w.dst, p...)
	return len(p), nil
}

func mustSerializeTx(t *testing.T, tx *wire.MsgTx) []byte {
	t.Helper()
	buf := make([]byte, 0, 1024)
	writer := makeByteSliceWriter(&buf)
	if err := tx.Serialize(writer); err != nil {
		t.Fatalf("serialize tx: %v", err)
	}
	return buf
}

func mustP2PKHScriptFromHash(t *testing.T, hash160 []byte, params *chaincfg.Params) []byte {
	t.Helper()
	addr, err := btcutil.NewAddressPubKeyHash(hash160, params)
	if err != nil {
		t.Fatalf("new address from hash160: %v", err)
	}
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("pay-to-addr script: %v", err)
	}
	return script
}

func mustWitnessPinScript(t *testing.T, path string, body []byte) []byte {
	t.Helper()
	script, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_FALSE).
		AddOp(txscript.OP_IF).
		AddData([]byte("metaid")).
		AddData([]byte("create")).
		AddData([]byte(path)).
		AddData([]byte("0")).
		AddData([]byte("0")).
		AddData([]byte("application/json")).
		AddData(body).
		AddOp(txscript.OP_ENDIF).
		Script()
	if err != nil {
		t.Fatalf("build witness script: %v", err)
	}
	return script
}
