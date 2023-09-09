package foundation

import (
	"context"
	"testing"

	fhttp "github.com/ri-nat/foundation/http"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func mockContextWithScope(scope string) context.Context {
	md := metadata.Pairs(fhttp.HeaderXScope, scope)
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestCheckScopePresenceFuncs(t *testing.T) {
	tests := []struct {
		name           string
		functionToTest string // either "all" or "any"
		contextScopes  string
		requiredScopes []string
		shouldError    bool
	}{
		{"All scopes are present", "all", "read write", []string{"read", "write"}, false},
		{"Missing some scopes", "all", "read", []string{"read", "write"}, true},
		{"At least one scope is present", "any", "read", []string{"read", "write"}, false},
		{"No scopes are present", "any", "delete", []string{"read", "write"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mockContextWithScope(tt.contextScopes)
			var err *PermissionDeniedError

			switch tt.functionToTest {
			case "all":
				err = CheckAllScopesPresence(ctx, tt.requiredScopes...)
			case "any":
				err = CheckAnyScopePresence(ctx, tt.requiredScopes...)
			}

			if tt.shouldError {
				assert.NotNil(t, err)
				assert.IsType(t, &PermissionDeniedError{}, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
