package gateway

import (
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestMain silences the standard logger. Several paths under test deliberately
// log errors — an authentication backend failing, a recovered panic — and their
// output would otherwise drown the test results.
func TestMain(m *testing.M) {
	logrus.SetOutput(io.Discard)

	os.Exit(m.Run())
}
