package foundation

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const (
	// Alphabet is the default alphabet used for string generation.
	Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// GenerateRandomString generates a random string of length n.
func GenerateRandomString(n int) (string, error) {
	var sb strings.Builder
	max := big.NewInt(int64(len(Alphabet)))

	for i := 0; i < n; i++ {
		randomInt, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		sb.WriteByte(Alphabet[randomInt.Int64()])
	}

	return sb.String(), nil
}
