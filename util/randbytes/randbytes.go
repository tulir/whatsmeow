package randbytes

import (
	"crypto/rand"
	"fmt"
)

func Make(length int) []byte {
	random := make([]byte, length)
	_, err := rand.Read(random)
	if err != nil {
		panic(fmt.Errorf("failed to get random bytes: %w", err))
	}
	return random
}
