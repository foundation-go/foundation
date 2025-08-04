package gateway

import (
	"net/http"
	"os"
	"strconv"
)

// WithSwagger returns a middleware that serves a swagger JSON file at /api.swagger.json
// The swagger content is loaded into memory from the file path for efficient serving.
func WithSwagger(swaggerFilePath string) (func(http.Handler) http.Handler, error) {
	// Load the swagger file into memory
	swaggerContent, err := os.ReadFile(swaggerFilePath)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only handle the swagger endpoint
			if r.URL.Path == "/api.swagger.json" && r.Method == http.MethodGet {
				// Set appropriate headers
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", strconv.Itoa(len(swaggerContent)))

				// Disable caching
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")

				// Write the swagger content
				w.Write(swaggerContent)
				return
			}

			// Pass through to the next handler
			next.ServeHTTP(w, r)
		})
	}, nil
}

