package foundation

import (
	"context"
	"strings"
	"testing"

	fctx "github.com/ri-nat/foundation/context"
	ferr "github.com/ri-nat/foundation/errors"
	"github.com/stretchr/testify/assert"
)

func mockContextWithScope(scope string) context.Context {
	return fctx.WithScopes(context.Background(), fctx.Oauth2Scopes(strings.Split(scope, " ")))
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
			var err *ferr.PermissionDeniedError

			switch tt.functionToTest {
			case "all":
				err = CheckAllScopesPresence(ctx, tt.requiredScopes...)
			case "any":
				err = CheckAnyScopePresence(ctx, tt.requiredScopes...)
			}

			if tt.shouldError {
				assert.NotNil(t, err)
				assert.IsType(t, &ferr.PermissionDeniedError{}, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
