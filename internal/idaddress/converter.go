package idaddress

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil/base58"
)

func base58Decode(input string) ([]byte, error) {
	decoded := base58.Decode(input)
	if len(decoded) == 0 && len(input) > 0 {
		return nil, errors.New("invalid base58 string")
	}
	return decoded, nil
}

func base58CheckDecode(input string) (version byte, payload []byte, err error) {
	decoded, err := base58Decode(input)
	if err != nil {
		return 0, nil, err
	}
	if len(decoded) < 5 {
		return 0, nil, errors.New("decoded data too short")
	}

	data := decoded[:len(decoded)-4]
	checksum := decoded[len(decoded)-4:]
	expected := doubleSHA256(data)
	for i := 0; i < 4; i++ {
		if checksum[i] != expected[i] {
			return 0, nil, errors.New("checksum mismatch")
		}
	}
	return data[0], data[1:], nil
}

func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

func ConvertFromBitcoin(address string) (string, error) {
	version, payload, err := base58CheckDecode(address)
	if err == nil {
		return convertFromLegacyBitcoin(version, payload)
	}

	hrp, witnessVersion, program, _, err := Bech32Decode(address)
	if err == nil {
		return convertFromSegWitBitcoin(hrp, witnessVersion, program)
	}

	return "", fmt.Errorf("unsupported address format: %s", address)
}

func convertFromLegacyBitcoin(version byte, payload []byte) (string, error) {
	switch version {
	case 0x00, 0x6f, 0x1e:
		return EncodeIDAddress(versionP2PKH, payload)
	case 0x05, 0xc4, 0x16:
		return EncodeIDAddress(versionP2SH, payload)
	default:
		return "", fmt.Errorf("unsupported version byte: 0x%02x", version)
	}
}

func convertFromSegWitBitcoin(hrp string, witnessVersion byte, program []byte) (string, error) {
	if hrp != "bc" && hrp != "tb" {
		return "", fmt.Errorf("unsupported network: %s", hrp)
	}

	switch witnessVersion {
	case 0:
		if len(program) == 20 {
			return EncodeIDAddress(versionP2WPKH, program)
		}
		if len(program) == 32 {
			return EncodeIDAddress(versionP2WSH, program)
		}
		return "", fmt.Errorf("invalid witness v0 program length: %d", len(program))
	case 1:
		if len(program) == 32 {
			return EncodeIDAddress(versionP2TR, program)
		}
		return "", fmt.Errorf("invalid witness v1 program length: %d", len(program))
	default:
		return "", fmt.Errorf("unsupported witness version: %d", witnessVersion)
	}
}
