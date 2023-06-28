package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	fhttp "github.com/ri-nat/foundation/http"
)

func TestWithAuthenticationDetails(t *testing.T) {
	// Create a mock authentication handler
	mockAuthHandler := func(token string) (*AuthenticationResult, error) {
		switch token {
		case "valid_token":
			return &AuthenticationResult{IsAuthenticated: true, UserID: "user_id"}, nil
		case "invalid_token":
			return &AuthenticationResult{IsAuthenticated: false, UserID: ""}, nil
		default:
			return nil, errors.New("server error")
		}
	}

	// Create a mock handler
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xAuthenticated := ""
		xUserID := ""

		token := r.Header.Get(fhttp.HeaderAuthorization)
		switch token {
		case "valid_token":
			xAuthenticated = "true"
			xUserID = "user_id"
		case "invalid_token":
			xAuthenticated = "false"
			xUserID = ""
		}

		if r.Header.Get("X-Authenticated") != xAuthenticated {
			t.Errorf("Expected X-Authenticated header to be %s, but got %s", xAuthenticated, r.Header.Get("X-Authenticated"))
		}
		if r.Header.Get("X-User-Id") != xUserID {
			t.Errorf("Expected X-User-Id header to be %s, but got %s", xUserID, r.Header.Get("X-User-Id"))
		}

		// Write a response
		w.WriteHeader(http.StatusOK)
	})

	//
	// Test valid token
	//
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(fhttp.HeaderAuthorization, "valid_token")

	// Create a recorder to capture the response
	recorder := httptest.NewRecorder()

	// Call the middleware with the mock handler and authentication handler
	WithAuthenticationDetails(mockHandler, mockAuthHandler).ServeHTTP(recorder, req)

	// Check that the response code is OK
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, recorder.Code)
	}

	//
	// Test invalid token
	//
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(fhttp.HeaderAuthorization, "invalid_token")

	// Call the middleware with the mock handler and authentication handler
	WithAuthenticationDetails(mockHandler, mockAuthHandler).ServeHTTP(recorder, req)

	// Check that the response code is Internal Server Error
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestWithAuthentication(t *testing.T) {
	// Create a mock handler
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write a response
		w.WriteHeader(http.StatusOK)
	})

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// Test with authenticated request
	req.Header.Set(fhttp.HeaderXAuthenticated, "true")
	recorder := httptest.NewRecorder()
	WithAuthentication(mockHandler, []string{}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, recorder.Code)
	}

	// Test with unauthenticated request
	req.Header.Del(fhttp.HeaderXAuthenticated)
	recorder = httptest.NewRecorder()
	WithAuthentication(mockHandler, []string{}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, but got %d", http.StatusUnauthorized, recorder.Code)
	}

	// Test `except` option
	req.URL.Path = "/signup"
	recorder = httptest.NewRecorder()
	WithAuthentication(mockHandler, []string{"/signup"}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, recorder.Code)
	}
}
