package grpc

import (
	log "github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewInternalError(err error, explanation string) error {
	log.WithError(err).Error(explanation)

	return status.Errorf(codes.Internal, "internal error")
}

func NewUnauthenticatedError(err error, explanation string) error {
	log.WithError(err).Debug(explanation)

	return status.Errorf(codes.Unauthenticated, explanation)
}
