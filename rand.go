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

// GenerateRandomString generates a random string of length n.
func GenerateRandomString(n int) (string, FoundationError) {
	var sb strings.Builder
	max := big.NewInt(int64(len(Alphabet)))

	for i := 0; i < n; i++ {
		randomInt, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", NewInternalError(err, "failed to generate random string")
		}

		sb.WriteByte(Alphabet[randomInt.Int64()])
	}

	return sb.String(), nil
}
