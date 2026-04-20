package idaddress

import (
	"errors"
	"strings"
)

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

type Bech32Encoding int

const (
	Bech32  Bech32Encoding = 1
	Bech32m Bech32Encoding = 2
)

var bech32CharsetMap = map[rune]int{
	'q': 0, 'p': 1, 'z': 2, 'r': 3, 'y': 4, '9': 5, 'x': 6, '8': 7,
	'g': 8, 'f': 9, '2': 10, 't': 11, 'v': 12, 'd': 13, 'w': 14, '0': 15,
	's': 16, '3': 17, 'j': 18, 'n': 19, '5': 20, '4': 21, 'k': 22, 'h': 23,
	'c': 24, 'e': 25, '6': 26, 'm': 27, 'u': 28, 'a': 29, '7': 30, 'l': 31,
}

func Bech32Decode(addr string) (hrp string, version byte, program []byte, encoding Bech32Encoding, err error) {
	addr = strings.ToLower(strings.TrimSpace(addr))

	pos := strings.LastIndex(addr, "1")
	if pos < 1 || pos+7 > len(addr) || len(addr) > 90 {
		return "", 0, nil, 0, errors.New("invalid bech32 address format")
	}

	hrp = addr[:pos]
	data := addr[pos+1:]

	decoded := make([]int, 0, len(data))
	for _, char := range data {
		value, ok := bech32CharsetMap[char]
		if !ok {
			return "", 0, nil, 0, errors.New("invalid bech32 character")
		}
		decoded = append(decoded, value)
	}

	encoding = Bech32m
	if !verifyBech32Checksum(hrp, decoded, Bech32m) {
		encoding = Bech32
		if !verifyBech32Checksum(hrp, decoded, Bech32) {
			return "", 0, nil, 0, errors.New("invalid bech32 checksum")
		}
	}

	decoded = decoded[:len(decoded)-6]
	if len(decoded) < 1 {
		return "", 0, nil, 0, errors.New("invalid bech32 data length")
	}

	version = byte(decoded[0])
	program, err = convertBits8to5(toByteArray(decoded[1:]), 5, 8, false)
	if err != nil {
		return "", 0, nil, 0, err
	}

	if len(program) < 2 || len(program) > 40 {
		return "", 0, nil, 0, errors.New("invalid witness program length")
	}
	if version == 0 && encoding != Bech32 {
		return "", 0, nil, 0, errors.New("witness version 0 must use bech32")
	}
	if version != 0 && encoding != Bech32m {
		return "", 0, nil, 0, errors.New("witness version 1+ must use bech32m")
	}

	return hrp, version, program, encoding, nil
}

func bech32Polymod(values []int) int {
	generators := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, value := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ value
		for i := 0; i < 5; i++ {
			if (top>>uint(i))&1 == 1 {
				chk ^= generators[i]
			}
		}
	}
	return chk
}

func bech32ExpandHRP(value string) []int {
	result := make([]int, 0, len(value)*2+1)
	for _, char := range value {
		result = append(result, int(char>>5))
	}
	result = append(result, 0)
	for _, char := range value {
		result = append(result, int(char&31))
	}
	return result
}

func verifyBech32Checksum(hrp string, data []int, encoding Bech32Encoding) bool {
	values := append(bech32ExpandHRP(hrp), data...)
	polymod := bech32Polymod(values)
	if encoding == Bech32 {
		return polymod == 1
	}
	return polymod == 0x2bc830a3
}

func convertBits8to5(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	acc := 0
	bits := uint(0)
	result := make([]byte, 0, len(data)*int(fromBits)/int(toBits)+1)
	maxv := (1 << toBits) - 1

	for _, value := range data {
		acc = (acc << fromBits) | int(value)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			result = append(result, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			result = append(result, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, errors.New("invalid padding")
	}

	return result, nil
}

func toByteArray(values []int) []byte {
	result := make([]byte, len(values))
	for i, value := range values {
		result[i] = byte(value)
	}
	return result
}
