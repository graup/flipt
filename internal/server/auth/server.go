package auth

import (
	"context"
	"fmt"
	"strings"

	"go.flipt.io/flipt/internal/server/audit"
	"go.flipt.io/flipt/internal/storage"
	storageauth "go.flipt.io/flipt/internal/storage/auth"
	"go.flipt.io/flipt/rpc/flipt/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const ipKey = "x-forwarded-for"

var _ auth.AuthenticationServiceServer = &Server{}

// Actor represents some metadata from the context for the audit event.
type Actor map[string]string

func ActorFromContext(ctx context.Context) Actor {
	var (
		actor          = Actor{}
		authentication = "none"
	)

	md, _ := metadata.FromIncomingContext(ctx)
	if len(md[ipKey]) > 0 {
		actor["ip"] = md[ipKey][0]
	}

	auth := GetAuthenticationFrom(ctx)
	if auth != nil {
		authentication = strings.ToLower(strings.TrimPrefix(auth.Method.String(), "METHOD_"))
		for k, v := range auth.Metadata {
			actor[k] = v
		}
	}

	actor["authentication"] = authentication
	return actor
}

// Server is the core AuthenticationServiceServer implementations.
//
// It is the service which presents all Authentications created in the backing auth store.
type Server struct {
	logger *zap.Logger
	store  storageauth.Store

	enableAuditLogging bool

	auth.UnimplementedAuthenticationServiceServer
}

type Option func(*Server)

// WithAuditLoggingEnabled sets the option for enabling audit logging for the auth server.
func WithAuditLoggingEnabled(enabled bool) Option {
	return func(s *Server) {
		s.enableAuditLogging = enabled
	}
}

func NewServer(logger *zap.Logger, store storageauth.Store, opts ...Option) *Server {
	s := &Server{
		logger: logger,
		store:  store,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// RegisterGRPC registers the server as an Server on the provided grpc server.
func (s *Server) RegisterGRPC(server *grpc.Server) {
	auth.RegisterAuthenticationServiceServer(server, s)
}

// GetAuthenticationSelf returns the Authentication which was derived from the request context.
func (s *Server) GetAuthenticationSelf(ctx context.Context, _ *emptypb.Empty) (*auth.Authentication, error) {
	if auth := GetAuthenticationFrom(ctx); auth != nil {
		s.logger.Debug("GetAuthentication", zap.String("id", auth.Id))

		return auth, nil
	}

	return nil, errUnauthenticated
}

// GetAuthentication returns the Authentication identified by the supplied id.
func (s *Server) GetAuthentication(ctx context.Context, r *auth.GetAuthenticationRequest) (*auth.Authentication, error) {
	return s.store.GetAuthenticationByID(ctx, r.Id)
}

// ListAuthentications produces a set of authentications for the provided method filter and pagination parameters.
func (s *Server) ListAuthentications(ctx context.Context, r *auth.ListAuthenticationsRequest) (*auth.ListAuthenticationsResponse, error) {
	req := &storage.ListRequest[storageauth.ListAuthenticationsPredicate]{
		QueryParams: storage.QueryParams{
			Limit:     uint64(r.Limit),
			PageToken: r.PageToken,
		},
	}

	if r.Method != auth.Method_METHOD_NONE {
		req.Predicate.Method = &r.Method
	}

	results, err := s.store.ListAuthentications(ctx, req)
	if err != nil {
		s.logger.Error("listing authentication", zap.Error(err))

		return nil, fmt.Errorf("listing authentications: %w", err)
	}

	return &auth.ListAuthenticationsResponse{
		Authentications: results.Results,
		NextPageToken:   results.NextPageToken,
	}, nil
}

// DeleteAuthentication deletes the authentication with the supplied ID.
func (s *Server) DeleteAuthentication(ctx context.Context, req *auth.DeleteAuthenticationRequest) (*emptypb.Empty, error) {
	s.logger.Debug("DeleteAuthentication", zap.String("id", req.Id))

	if s.enableAuditLogging {
		actor := ActorFromContext(ctx)

		a, err := s.GetAuthentication(ctx, &auth.GetAuthenticationRequest{
			Id: req.Id,
		})
		if err != nil {
			s.logger.Error("failed to get authentication for audit events", zap.Error(err))
			return nil, err
		}
		if a.Method == auth.Method_METHOD_TOKEN {
			event := audit.NewEvent(audit.TokenType, audit.Delete, actor, a.Metadata)
			event.AddToSpan(ctx)
		}
	}

	return &emptypb.Empty{}, s.store.DeleteAuthentications(ctx, storageauth.Delete(storageauth.WithID(req.Id)))
}

// ExpireAuthenticationSelf expires the Authentication which was derived from the request context.
// If no expire_at is provided, the current time is used. This is useful for logging out a user.
// If the expire_at is greater than the current expiry time, the expiry time is extended.
func (s *Server) ExpireAuthenticationSelf(ctx context.Context, req *auth.ExpireAuthenticationSelfRequest) (*emptypb.Empty, error) {
	if auth := GetAuthenticationFrom(ctx); auth != nil {
		s.logger.Debug("ExpireAuthentication", zap.String("id", auth.Id))

		if req.ExpiresAt == nil || !req.ExpiresAt.IsValid() {
			req.ExpiresAt = timestamppb.Now()
		}

		return &emptypb.Empty{}, s.store.ExpireAuthenticationByID(ctx, auth.Id, req.ExpiresAt)
	}

	return nil, errUnauthenticated
}
