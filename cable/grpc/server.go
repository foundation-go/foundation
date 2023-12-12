package cable_grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	pb "github.com/foundation-go/foundation/cable/grpc/proto"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	IsAuthenticatedKey = "isAuthenticated"
	UserIDKey          = "userID"
)

const (
	CmdMessage     = "message"
	CmdPing        = "ping"
	CmdSubscribe   = "subscribe"
	CmdUnsubscribe = "unsubscribe"

	WelcomeMessage                     = `{"type": "welcome"}`
	ConnectionUnauthorizedMessage      = `{"type":"disconnect", "reason":"unauthorized", "reconnect": false}`
	ConfirmSubscriptionMessageTemplate = `{"identifier": "%s", "type": "confirm_subscription"}`
	RejectSubscriptionMessageTemplate  = `{"identifier": "%s", "type": "reject_subscription"}`
)

// Channel represents a single communication path, supporting authorization and multiple streams subscription
type Channel interface {
	// Authorize checks if the user is authorized to subscribe to the channel,
	// based on the provided user ID and channel identifier.
	Authorize(ctx context.Context, userID string, ident map[string]string) error
	// GetStreams returns a list of streams that the user should be subscribed to
	// or unsubscribe from, based on the provided user ID and channel identifier.
	GetStreams(ctx context.Context, userID string, ident map[string]string) []string
}

// AuthenticationFunc is used to authenticate a user based on the provided access token.
type AuthenticationFunc func(ctx context.Context, accessToken string) (userID string, err error)

// Server encapsulates the AnyCable RPC server functionalities.
type Server struct {
	pb.UnimplementedRPCServer

	Channels map[string]Channel

	WithAuthentication bool
	AuthenticationFunc AuthenticationFunc

	Logger *logrus.Entry
}

// ConnIdentifier is used to identify a specific connection through its AccessToken.
type ConnIdentifier struct {
	AccessToken string
}

func (s *Server) Connect(ctx context.Context, in *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	s.Logger.WithField("url", in.Env.Url).Debug("Connect received")

	unauthenticatedResp := &pb.ConnectionResponse{
		Status:        pb.Status_FAILURE,
		ErrorMsg:      "Unauthenticated",
		Transmissions: []string{ConnectionUnauthorizedMessage},
	}

	var accessToken string
	cState := in.Env.Cstate
	if cState == nil {
		cState = make(map[string]string)
	}

	if s.WithAuthentication {
		// Parse URL from in.Env.Url and get `accessToken` from it
		parsedURL, err := url.Parse(in.Env.Url)
		if err != nil {
			return nil, err
		}

		accessToken = parsedURL.Query().Get("accessToken")
		if accessToken == "" {
			return unauthenticatedResp, nil
		}

		userID, err := s.AuthenticationFunc(ctx, accessToken)
		if err != nil {
			s.Logger.WithError(err).Error("Authentication failed")

			return unauthenticatedResp, nil
		}

		cState[UserIDKey] = userID
		cState[IsAuthenticatedKey] = "true"
	} else {
		// We need an `accessToken` to identify the connection, so we generate a random one
		accessToken = fmt.Sprintf("foundationGuest-%s", uuid.New().String())
		cState[IsAuthenticatedKey] = "false"
	}

	ident := &ConnIdentifier{
		AccessToken: accessToken,
	}
	identJSON, err := json.Marshal(ident)
	if err != nil {
		return nil, err
	}

	resp := &pb.ConnectionResponse{
		Status:      pb.Status_SUCCESS,
		Identifiers: string(identJSON),
		Env: &pb.EnvResponse{
			Cstate: cState,
			Istate: in.Env.Istate,
		},
		Transmissions: []string{WelcomeMessage},
	}

	return resp, nil
}

func (s *Server) Command(ctx context.Context, in *pb.CommandMessage) (*pb.CommandResponse, error) {
	s.Logger.WithField("command", in.Command).Debug("Command received")

	resp := &pb.CommandResponse{
		Status: pb.Status_SUCCESS,
		Env: &pb.EnvResponse{
			Cstate: in.Env.Cstate,
			Istate: in.Env.Istate,
		},
	}

	ident, err := s.decodeIdentifier(in.Identifier)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to parse identifier")

		return &pb.CommandResponse{
			Status:   pb.Status_FAILURE,
			ErrorMsg: "Wrong identifier format",
		}, nil
	}

	switch in.Command {
	case CmdPing:
		return resp, nil
	case CmdMessage: // Just skip for now
		return resp, nil
	case CmdSubscribe:
		escapedIdentifier := strconv.Quote(in.Identifier)
		escapedIdentifier = escapedIdentifier[1 : len(escapedIdentifier)-1]

		ch, err := s.channelFromIdent(ident)
		if err != nil {
			s.Logger.WithError(err).Error("Channel not found")

			return &pb.CommandResponse{
				Status:        pb.Status_FAILURE,
				ErrorMsg:      "Channel not found",
				Transmissions: []string{fmt.Sprintf(RejectSubscriptionMessageTemplate, escapedIdentifier)},
			}, nil
		}

		if err = ch.Authorize(ctx, in.Env.Cstate[UserIDKey], ident); err != nil {
			s.Logger.WithError(err).Debug("Authorization failed")

			return &pb.CommandResponse{
				Status:        pb.Status_FAILURE,
				ErrorMsg:      "Permission denied",
				Transmissions: []string{fmt.Sprintf(RejectSubscriptionMessageTemplate, escapedIdentifier)},
			}, nil
		}

		resp.Streams = ch.GetStreams(ctx, in.Env.Cstate[UserIDKey], ident)
		resp.Transmissions = []string{fmt.Sprintf(ConfirmSubscriptionMessageTemplate, escapedIdentifier)}

		return resp, nil
	case CmdUnsubscribe:
		ch, err := s.channelFromIdent(ident)
		if err != nil {
			s.Logger.WithError(err).Error("Channel not found")

			return &pb.CommandResponse{
				Status:   pb.Status_FAILURE,
				ErrorMsg: err.Error(),
			}, nil
		}

		resp.StoppedStreams = ch.GetStreams(ctx, in.Env.Cstate[UserIDKey], ident)

		return resp, nil
	default:
		s.Logger.WithField("command", in.Command).Error("Unknown command")

		return &pb.CommandResponse{
			Status:   pb.Status_ERROR,
			ErrorMsg: fmt.Sprintf("Unknown command: %s", in.Command),
		}, nil
	}
}

func (s *Server) Disconnect(ctx context.Context, in *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	s.Logger.Debug("Disconnect received")

	return &pb.DisconnectResponse{
		Status: pb.Status_SUCCESS,
	}, nil
}

func (s *Server) channelFromIdent(ident map[string]string) (Channel, error) {
	ch, ok := s.Channels[ident["channel"]]
	if !ok {
		return nil, fmt.Errorf("unknown channel: %s", ident["channel"])
	}

	return ch, nil
}

func (s *Server) decodeIdentifier(ident string) (map[string]string, error) {
	identJSON := []byte(ident)
	identObj := map[string]string{}
	err := json.Unmarshal(identJSON, &identObj)
	if err != nil {
		return nil, err
	}

	return identObj, nil
}
