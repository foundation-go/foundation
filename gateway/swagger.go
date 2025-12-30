package gateway

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
)

// SwaggerEndpoint defines a mapping between a URL path and a swagger file.
type SwaggerEndpoint struct {
	URLPath  string //e.g., "/api.swagger.json"
	FilePath string //e.g., "api.swagger.json"
}

// WithSwagger returns a middleware that serves swagger JSON files at specified URL paths.
// The swagger content is loaded into memory from the file paths for efficient serving.
func WithSwagger(endpoints []SwaggerEndpoint) (func(http.Handler) http.Handler, error) {
	swaggerContents := make(map[string][]byte, len(endpoints))

	for _, endpoint := range endpoints {
		content, err := os.ReadFile(endpoint.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load swagger file %s: %w", endpoint.FilePath, err)
		}

		swaggerContents[endpoint.URLPath] = content
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if content, ok := swaggerContents[r.URL.Path]; ok && r.Method == http.MethodGet {
				// Set appropriate headers
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", strconv.Itoa(len(content)))

				// Disable caching
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")

				// Write the swagger content
				w.Write(content)
				return
			}

			// Pass through to the next handler
			next.ServeHTTP(w, r)
		})
	}, nil
}
