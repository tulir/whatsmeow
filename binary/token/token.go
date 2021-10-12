package token

import "fmt"

func getSingleTokenMD(i int) (string, error) {
	if i < 3 || i >= len(mdSingleByteTokens) {
		return "", fmt.Errorf("index out of single byte token bounds %d", i)
	}

	return mdSingleByteTokens[i], nil
}
func getSingleTokenWeb(i int) (string, error) {
	if i < 3 || i >= len(webSingleByteTokens) {
		return "", fmt.Errorf("index out of single byte token bounds %d", i)
	}

	return webSingleByteTokens[i], nil
}

func GetSingleToken(i int, md bool) (string, error) {
	if md {
		return getSingleTokenMD(i)
	} else {
		return getSingleTokenWeb(i)
	}
}

func getDoubleTokenMD(index1, index2 int) (string, error) {
	if index1 < 0 || index1 >= len(mdDoubleByteTokens) {
		return "", fmt.Errorf("index out of double byte token bounds %d-%d", index1, index2)
	} else if index2 < 0 || index2 >= len(mdDoubleByteTokens[index1]) {
		return "", fmt.Errorf("index out of double byte token index %d bounds %d", index1, index2)
	}

	return mdDoubleByteTokens[index1][index2], nil
}

func GetDoubleToken(index1, index2 int, md bool) (string, error) {
	if md {
		return getDoubleTokenMD(index1, index2)
	} else {
		return "", fmt.Errorf("web doesn't have double byte tokens")
	}
}

func SingleTokenCount(md bool) int {
	if md {
		return len(mdSingleByteTokens)
	} else {
		return len(webSingleByteTokens)
	}
}

func IndexOfSingleToken(token string, md bool) (val byte, ok bool) {
	if md {
		val, ok = mdSingleByteTokenIndex[token]
	} else {
		val, ok = webSingleByteTokenIndex[token]
	}
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
	PackedMax     = 254
	SingleByteMax = 256
)
