package cable_grpc

import (
	"context"

	fhydra "github.com/ri-nat/foundation/hydra"
)

func HydraAuthenticationFunc(ctx context.Context, accessToken string) (userID string, err error) {
	result, err := fhydra.IntrospectedOAuth2Token(ctx, accessToken)
	if err != nil || !result.Active {
		return "", err
	}

	return *result.Sub, nil
}
