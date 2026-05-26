package idaddress

import (
	"strings"
	"testing"
)

func TestEncodeGlobalMetaId(t *testing.T) {
	// Test: EncodeGlobalMetaId should generate "id"-prefixed addresses.
	// Use a well-known BTC P2PKH address for the test.
	btcAddr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	result := EncodeGlobalMetaId(btcAddr, "btc")
	if result == "" {
		t.Fatal("EncodeGlobalMetaId returned empty for valid BTC P2PKH address")
	}
	if !strings.HasPrefix(result, "id") {
		t.Errorf("expected result to start with 'id', got %q", result)
	}
	t.Logf("EncodeGlobalMetaId(%q, btc) = %q", btcAddr, result)
}

func TestEncodeGlobalMetaId_EmptyAddress(t *testing.T) {
	result := EncodeGlobalMetaId("", "btc")
	if result != "" {
		t.Errorf("expected empty result for empty address, got %q", result)
	}

	result = EncodeGlobalMetaId("errorAddr", "btc")
	if result != "" {
		t.Errorf("expected empty result for errorAddr, got %q", result)
	}
}

func TestDecodeGlobalMetaId(t *testing.T) {
	// First encode a BTC address, then decode it back.
	btcAddr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	globalMetaId := EncodeGlobalMetaId(btcAddr, "btc")
	if globalMetaId == "" {
		t.Fatal("EncodeGlobalMetaId returned empty")
	}

	metaid, chainName, err := DecodeGlobalMetaId(globalMetaId)
	if err != nil {
		t.Fatalf("DecodeGlobalMetaId failed: %v", err)
	}
	if metaid == "" {
		t.Error("decoded metaid is empty")
	}
	if chainName == "" {
		t.Error("decoded chainName is empty")
	}
	t.Logf("round-trip: %s -> %s -> (%s, %s)", btcAddr, globalMetaId, metaid, chainName)
}

func TestDecodeGlobalMetaId_Empty(t *testing.T) {
	_, _, err := DecodeGlobalMetaId("")
	if err == nil {
		t.Error("expected error for empty GlobalMetaId")
	}
}

func TestEncodeGlobalMetaId_Dogecoin(t *testing.T) {
	dogeAddr := "DFo712BpLysLsoF6kSjTN6pPmZXxibtWcG"
	result := EncodeGlobalMetaId(dogeAddr, "doge")
	if result == "" {
		t.Fatal("EncodeGlobalMetaId returned empty for valid DOGE address")
	}
	if !strings.HasPrefix(result, "id") {
		t.Errorf("expected result to start with 'id', got %q", result)
	}
	t.Logf("EncodeGlobalMetaId(%q, doge) = %q", dogeAddr, result)
}

func TestValidateIDAddress(t *testing.T) {
	btcAddr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	globalMetaId := EncodeGlobalMetaId(btcAddr, "btc")
	if globalMetaId == "" {
		t.Fatal("EncodeGlobalMetaId returned empty")
	}
	if !ValidateIDAddress(globalMetaId) {
		t.Errorf("ValidateIDAddress returned false for valid ID address: %q", globalMetaId)
	}
	if ValidateIDAddress("invalid123") {
		t.Error("ValidateIDAddress returned true for invalid address")
	}
}

func TestEncodeDecodeIDAddress_Roundtrip(t *testing.T) {
	// Test P2PKH round-trip.
	data := make([]byte, 20)
	for i := range data {
		data[i] = byte(i + 1)
	}

	addr, err := EncodeIDAddress(VersionP2PKH, data)
	if err != nil {
		t.Fatalf("EncodeIDAddress failed: %v", err)
	}
	t.Logf("P2PKH ID address: %s", addr)

	info, err := DecodeIDAddress(addr)
	if err != nil {
		t.Fatalf("DecodeIDAddress failed: %v", err)
	}
	if info.Version != VersionP2PKH {
		t.Errorf("expected version P2PKH(0), got %d", info.Version)
	}
	if len(info.Data) != 20 {
		t.Errorf("expected data length 20, got %d", len(info.Data))
	}
	for i, b := range info.Data {
		if b != data[i] {
			t.Errorf("data mismatch at index %d: expected %d, got %d", i, data[i], b)
		}
	}
}

func TestEncodeIDAddress_InvalidVersion(t *testing.T) {
	_, err := EncodeIDAddress(AddressVersion(99), []byte{})
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestDecodeIDAddress_InvalidAddress(t *testing.T) {
	_, err := DecodeIDAddress("not-an-id-address")
	if err == nil {
		t.Error("expected error for invalid address")
	}

	_, err = DecodeIDAddress("idx1invalid")
	if err == nil {
		t.Error("expected error for invalid version character 'x'")
	}
}

func TestConvertFromBitcoin_P2PKH(t *testing.T) {
	btcAddr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	idAddr, err := ConvertFromBitcoin(btcAddr)
	if err != nil {
		t.Fatalf("ConvertFromBitcoin(P2PKH) failed: %v", err)
	}
	if !strings.HasPrefix(idAddr, "id") {
		t.Errorf("expected id-prefixed result, got %q", idAddr)
	}
	t.Logf("BTC %s -> ID %s", btcAddr, idAddr)
}

func TestConvertFromBitcoin_P2WPKH(t *testing.T) {
	// A known Bech32 mainnet address.
	btcAddr := "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
	idAddr, err := ConvertFromBitcoin(btcAddr)
	if err != nil {
		t.Fatalf("ConvertFromBitcoin(P2WPKH) failed: %v", err)
	}
	if !strings.HasPrefix(idAddr, "id") {
		t.Errorf("expected id-prefixed result, got %q", idAddr)
	}
	t.Logf("BTC %s -> ID %s", btcAddr, idAddr)
}

func TestConvertToBitcoin(t *testing.T) {
	btcAddr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	idAddr, err := ConvertFromBitcoin(btcAddr)
	if err != nil {
		t.Fatalf("ConvertFromBitcoin failed: %v", err)
	}

	recovered, err := ConvertToBitcoin(idAddr, "mainnet")
	if err != nil {
		t.Fatalf("ConvertToBitcoin failed: %v", err)
	}
	if recovered != btcAddr {
		t.Errorf("round-trip mismatch: expected %q, got %q", btcAddr, recovered)
	}
	t.Logf("round-trip BTC P2PKH: %s -> %s -> %s", btcAddr, idAddr, recovered)
}

func TestBech32Decode_Valid(t *testing.T) {
	addr := "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
	hrp, version, program, _, err := Bech32Decode(addr)
	if err != nil {
		t.Fatalf("Bech32Decode failed: %v", err)
	}
	if hrp != "bc" {
		t.Errorf("expected hrp 'bc', got %q", hrp)
	}
	if version != 0 {
		t.Errorf("expected version 0, got %d", version)
	}
	if len(program) != 20 {
		t.Errorf("expected program length 20, got %d", len(program))
	}
	t.Logf("Bech32: hrp=%s version=%d program_len=%d", hrp, version, len(program))
}

func TestBase58CheckDecode_Valid(t *testing.T) {
	addr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	version, payload, err := Base58CheckDecode(addr)
	if err != nil {
		t.Fatalf("Base58CheckDecode failed: %v", err)
	}
	if version != 0x00 {
		t.Errorf("expected version 0x00, got 0x%02x", version)
	}
	if len(payload) != 20 {
		t.Errorf("expected payload length 20, got %d", len(payload))
	}
	t.Logf("Base58Check: version=0x%02x payload_len=%d", version, len(payload))
}

func TestConvertFromBitcoin_Invalid(t *testing.T) {
	_, err := ConvertFromBitcoin("invalid-address")
	if err == nil {
		t.Error("expected error for invalid address")
	}

	_, err = ConvertFromBitcoin("")
	if err == nil {
		t.Error("expected error for empty address")
	}
}
