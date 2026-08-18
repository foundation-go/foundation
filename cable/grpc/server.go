package cable_grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/foundation-go/foundation/cable/grpc/proto"
)

const (
	IsAuthenticatedKey = "isAuthenticated"
	UserIDKey          = "userID"
)

const (
	// AccessTokenQueryParam is the query parameter the access token is read
	// from when no header carries it.
	//
	// A token in a URL is a token in every access log, proxy log and browser
	// history along the way. Prefer sending it as a header; this remains
	// supported because browser WebSocket clients cannot set headers.
	AccessTokenQueryParam = "accessToken"

	// AuthorizationHeader and AccessTokenHeader are the headers checked, in
	// this order, before falling back to the query parameter.
	AuthorizationHeader = "authorization"
	AccessTokenHeader   = "x-access-token"
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

// ConnIdentifier identifies a specific connection.
//
// It used to hold the raw access token. AnyCable stores the identifier for the
// life of the connection and hands it back with every command, so the token sat
// in that state for no reason: nothing ever read it back, it only had to be
// unique per connection. A random ID does that without keeping a credential
// around.
type ConnIdentifier struct {
	ConnectionID string `json:"connectionId"`
}

func (s *Server) Connect(ctx context.Context, in *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	if in.GetEnv() == nil {
		return nil, status.Error(codes.InvalidArgument, "connection request has no env")
	}

	// The URL carries the access token as a query parameter, so it is logged
	// with that parameter removed. It used to be logged whole, which put every
	// access token into the debug log.
	s.Logger.WithField("url", redactAccessToken(in.Env.Url)).Debug("Connect received")

	unauthenticatedResp := &pb.ConnectionResponse{
		Status:        pb.Status_FAILURE,
		ErrorMsg:      "Unauthenticated",
		Transmissions: []string{ConnectionUnauthorizedMessage},
	}

	cState := in.Env.Cstate
	if cState == nil {
		cState = make(map[string]string)
	}

	if s.WithAuthentication {
		if s.AuthenticationFunc == nil {
			// Failing closed rather than dereferencing nil: a server configured
			// to authenticate but given nothing to authenticate with must not
			// let connections through, and must not crash either.
			s.Logger.Error("WithAuthentication is enabled but AuthenticationFunc is not set")

			return unauthenticatedResp, nil
		}

		accessToken := accessTokenFrom(in.Env)
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
		cState[IsAuthenticatedKey] = "false"
	}

	ident := &ConnIdentifier{
		ConnectionID: uuid.New().String(),
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
	if in.GetEnv() == nil {
		return nil, status.Error(codes.InvalidArgument, "command has no env")
	}

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

// accessTokenFrom extracts the access token from a connection environment,
// preferring a header over the query parameter.
func accessTokenFrom(env *pb.Env) string {
	for _, header := range []string{AuthorizationHeader, AccessTokenHeader} {
		if value := headerValue(env.GetHeaders(), header); value != "" {
			return strings.TrimPrefix(value, "Bearer ")
		}
	}

	parsedURL, err := url.Parse(env.GetUrl())
	if err != nil {
		return ""
	}

	return parsedURL.Query().Get(AccessTokenQueryParam)
}

// headerValue looks a header up case-insensitively; AnyCable does not
// canonicalise the map it forwards.
func headerValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}

	return ""
}

// redactAccessToken removes the access token from a URL so it can be logged.
func redactAccessToken(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "<unparsable url>"
	}

	query := parsedURL.Query()
	if query.Has(AccessTokenQueryParam) {
		query.Set(AccessTokenQueryParam, "[REDACTED]")
		parsedURL.RawQuery = query.Encode()
	}

	return parsedURL.String()
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
