package foundation

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const (
	// Alphabet is the default alphabet used for string generation.
	Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
)

// GenerateRandomString generates a random string of length n. Panics if an error occurs.
func GenerateRandomString(n int) string {
	var sb strings.Builder
	maxValue := big.NewInt(int64(len(Alphabet)))

	for i := 0; i < n; i++ {
		randomInt, err := rand.Int(rand.Reader, maxValue)
		if err != nil {
			panic(err)
		}

		sb.WriteByte(Alphabet[randomInt.Int64()])
	}

	return sb.String()
}
