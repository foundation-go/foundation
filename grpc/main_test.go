package grpc

import (
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestMain silences the standard logger. The recovery interceptors deliberately
// log a panic with its stack, and that output would otherwise drown the test
// results.
func TestMain(m *testing.M) {
	logrus.SetOutput(io.Discard)

	os.Exit(m.Run())
}
