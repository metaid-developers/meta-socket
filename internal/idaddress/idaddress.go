package idaddress

import (
	"errors"
	"fmt"
	"strings"
)

const (
	hrp            = "id"
	separator      = "1"
	checksumLength = 6
	charset        = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
)

type addressVersion byte

const (
	versionP2PKH  addressVersion = 0
	versionP2SH   addressVersion = 1
	versionP2WPKH addressVersion = 2
	versionP2WSH  addressVersion = 3
	versionP2TR   addressVersion = 5
)

var versionChar = map[addressVersion]string{
	versionP2PKH:  "q",
	versionP2SH:   "p",
	versionP2WPKH: "z",
	versionP2WSH:  "r",
	versionP2TR:   "t",
}

var charsetMap map[byte]int

func init() {
	charsetMap = make(map[byte]int)
	for i := 0; i < len(charset); i++ {
		charsetMap[charset[i]] = i
	}
}

func EncodeIDAddress(version addressVersion, data []byte) (string, error) {
	versionSymbol, ok := versionChar[version]
	if !ok {
		return "", fmt.Errorf("invalid version: %d", version)
	}
	if err := validateDataLength(version, data); err != nil {
		return "", err
	}

	converted, err := convertBits(data, 8, 5, true)
	if err != nil {
		return "", err
	}

	hrpWithVersion := hrp + versionSymbol
	checksum := createChecksum(expandHRP(hrpWithVersion), converted)
	combined := append(converted, checksum...)

	encoded := make([]byte, len(combined))
	for i, value := range combined {
		encoded[i] = charset[value]
	}

	return hrpWithVersion + separator + string(encoded), nil
}

func ValidateIDAddress(addr string) bool {
	addr = strings.ToLower(strings.TrimSpace(addr))
	if !strings.HasPrefix(addr, hrp) {
		return false
	}

	idx := strings.LastIndex(addr, separator)
	if idx <= len(hrp) || idx+1 >= len(addr) {
		return false
	}

	versionSymbol := addr[len(hrp):idx]
	if len(versionSymbol) != 1 {
		return false
	}
	for _, symbol := range versionChar {
		if symbol == versionSymbol {
			return true
		}
	}
	return false
}

func convertBits(data []byte, fromBits, toBits int, pad bool) ([]int, error) {
	acc := 0
	bits := 0
	ret := make([]int, 0, len(data)*fromBits/toBits+1)
	maxv := (1 << toBits) - 1
	maxAcc := (1 << (fromBits + toBits - 1)) - 1

	for _, value := range data {
		if int(value)>>fromBits != 0 {
			return nil, errors.New("invalid data value")
		}
		acc = ((acc << fromBits) | int(value)) & maxAcc
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, (acc>>bits)&maxv)
		}
	}

	if pad {
		if bits > 0 {
			ret = append(ret, (acc<<(toBits-bits))&maxv)
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, errors.New("invalid padding")
	}

	return ret, nil
}

func expandHRP(value string) []int {
	ret := make([]int, len(value)*2+1)
	for i, char := range value {
		ret[i] = int(char) >> 5
		ret[i+len(value)+1] = int(char) & 31
	}
	ret[len(value)] = 0
	return ret
}

func polymod(values []int) int {
	generators := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, value := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ value
		for i := 0; i < 5; i++ {
			if (top>>i)&1 == 1 {
				chk ^= generators[i]
			}
		}
	}
	return chk
}

func createChecksum(hrpExpanded []int, data []int) []int {
	values := append(hrpExpanded, data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	mod := polymod(values) ^ 1
	checksum := make([]int, checksumLength)
	for i := 0; i < checksumLength; i++ {
		checksum[i] = (mod >> (5 * (5 - i))) & 31
	}
	return checksum
}

func validateDataLength(version addressVersion, data []byte) error {
	switch version {
	case versionP2PKH, versionP2SH, versionP2WPKH:
		if len(data) != 20 {
			return fmt.Errorf("invalid data length for version %d: expected 20, got %d", version, len(data))
		}
	case versionP2WSH, versionP2TR:
		if len(data) != 32 {
			return fmt.Errorf("invalid data length for version %d: expected 32, got %d", version, len(data))
		}
	default:
		return fmt.Errorf("unsupported version: %d", version)
	}
	return nil
}
