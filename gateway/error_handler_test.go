package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	fctx "github.com/ri-nat/foundation/context"
)

func TestErrorHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost", nil)
	if err != nil {
		t.Fatalf("could not create http request: %v", err)
	}

	w := httptest.NewRecorder()
	mux := runtime.NewServeMux()
	marshaler := &runtime.JSONPb{}

	internalError := status.Error(codes.Internal, "private error")
	ctx := context.Background()
	req = req.WithContext(fctx.WithLogger(ctx, logrus.NewEntry(logrus.New())))

	ErrorHandler(ctx, mux, marshaler, w, req, internalError)

	resp := w.Result()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	// Get the response body
	body, _ := io.ReadAll(resp.Body)

	// Check if the response body contains the expected error message
	if !strings.Contains(string(body), http.StatusText(http.StatusInternalServerError)) {
		t.Errorf("Expected response body to contain public error message, got %s", string(body))
	}

	// Check that the response body not contains the private error message
	if strings.Contains(string(body), internalError.Error()) {
		t.Errorf("Expected response body to contain private error message, got %s", string(body))
	}
}
