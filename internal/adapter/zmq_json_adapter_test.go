package adapter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
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
