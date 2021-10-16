package token

import "fmt"

func GetSingleToken(i int) (string, error) {
	if i < 3 || i >= len(SingleByteTokens) {
		return "", fmt.Errorf("index out of single byte token bounds %d", i)
	}

	return SingleByteTokens[i], nil
}

func GetDoubleToken(index1, index2 int) (string, error) {
	if index1 < 0 || index1 >= len(DoubleByteTokens) {
		return "", fmt.Errorf("index out of double byte token bounds %d-%d", index1, index2)
	} else if index2 < 0 || index2 >= len(DoubleByteTokens[index1]) {
		return "", fmt.Errorf("index out of double byte token index %d bounds %d", index1, index2)
	}

	return DoubleByteTokens[index1][index2], nil
}

func IndexOfSingleToken(token string) (val byte, ok bool) {
	val, ok = mdSingleByteTokenIndex[token]
	return
}

func IndexOfDoubleByteToken(token string) (byte, byte, bool) {
	val, ok := mdDoubleByteTokenIndex[token]
	return val.dictionary, val.index, ok
}

const (
	ListEmpty   = 0
	StreamEnd   = 2
	Dictionary0 = 236
	Dictionary1 = 237
	Dictionary2 = 238
	Dictionary3 = 239
	ADJID       = 247
	List8       = 248
	List16      = 249
	JIDPair     = 250
	Hex8        = 251
	Binary8     = 252
	Binary20    = 253
	Binary32    = 254
	Nibble8     = 255
)

const (
	PackedMax     = 127
	SingleByteMax = 256
)
