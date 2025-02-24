package server

import (
	"context"

	fliptotel "go.flipt.io/flipt/internal/server/otel"
	"go.flipt.io/flipt/internal/storage"
	flipt "go.flipt.io/flipt/rpc/flipt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	empty "google.golang.org/protobuf/types/known/emptypb"
)

// GetFlag gets a flag
func (s *Server) GetFlag(ctx context.Context, r *flipt.GetFlagRequest) (*flipt.Flag, error) {
	s.logger.Debug("get flag", zap.Stringer("request", r))

	if r.NamespaceKey == "" {
		r.NamespaceKey = storage.DefaultNamespace
	}

	flag, err := s.store.GetFlag(ctx, r.NamespaceKey, r.Key)

	spanAttrs := []attribute.KeyValue{
		fliptotel.AttributeNamespace.String(r.NamespaceKey),
		fliptotel.AttributeFlag.String(r.Key),
	}

	if flag != nil {
		spanAttrs = append(spanAttrs, fliptotel.AttributeFlagEnabled.Bool(flag.Enabled))
	}

	// add otel attributes to span
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(spanAttrs...)

	s.logger.Debug("get flag", zap.Stringer("response", flag))
	return flag, err
}

// ListFlags lists all flags
func (s *Server) ListFlags(ctx context.Context, r *flipt.ListFlagRequest) (*flipt.FlagList, error) {
	s.logger.Debug("list flags", zap.Stringer("request", r))

	if r.Offset < 0 {
		r.Offset = 0
	}

	opts := []storage.QueryOption{storage.WithLimit(uint64(r.Limit))}

	if r.PageToken != "" {
		opts = append(opts, storage.WithPageToken(r.PageToken))
	} else if r.Offset >= 0 {
		// TODO: deprecate
		opts = append(opts, storage.WithOffset(uint64(r.Offset)))
	}

	results, err := s.store.ListFlags(ctx, r.NamespaceKey, opts...)
	if err != nil {
		return nil, err
	}

	resp := flipt.FlagList{
		Flags: results.Results,
	}

	total, err := s.store.CountFlags(ctx, r.NamespaceKey)
	if err != nil {
		return nil, err
	}

	resp.TotalCount = int32(total)
	resp.NextPageToken = results.NextPageToken

	s.logger.Debug("list flags", zap.Stringer("response", &resp))
	return &resp, nil
}

// CreateFlag creates a flag
func (s *Server) CreateFlag(ctx context.Context, r *flipt.CreateFlagRequest) (*flipt.Flag, error) {
	s.logger.Debug("create flag", zap.Stringer("request", r))
	flag, err := s.store.CreateFlag(ctx, r)
	s.logger.Debug("create flag", zap.Stringer("response", flag))
	return flag, err
}

// UpdateFlag updates an existing flag
func (s *Server) UpdateFlag(ctx context.Context, r *flipt.UpdateFlagRequest) (*flipt.Flag, error) {
	s.logger.Debug("update flag", zap.Stringer("request", r))
	flag, err := s.store.UpdateFlag(ctx, r)
	s.logger.Debug("update flag", zap.Stringer("response", flag))
	return flag, err
}

// DeleteFlag deletes a flag
func (s *Server) DeleteFlag(ctx context.Context, r *flipt.DeleteFlagRequest) (*empty.Empty, error) {
	s.logger.Debug("delete flag", zap.Stringer("request", r))
	if err := s.store.DeleteFlag(ctx, r); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// CreateVariant creates a variant
func (s *Server) CreateVariant(ctx context.Context, r *flipt.CreateVariantRequest) (*flipt.Variant, error) {
	s.logger.Debug("create variant", zap.Stringer("request", r))
	variant, err := s.store.CreateVariant(ctx, r)
	s.logger.Debug("create variant", zap.Stringer("response", variant))
	return variant, err
}

// UpdateVariant updates an existing variant
func (s *Server) UpdateVariant(ctx context.Context, r *flipt.UpdateVariantRequest) (*flipt.Variant, error) {
	s.logger.Debug("update variant", zap.Stringer("request", r))
	variant, err := s.store.UpdateVariant(ctx, r)
	s.logger.Debug("update variant", zap.Stringer("response", variant))
	return variant, err
}

// DeleteVariant deletes a variant
func (s *Server) DeleteVariant(ctx context.Context, r *flipt.DeleteVariantRequest) (*empty.Empty, error) {
	s.logger.Debug("delete variant", zap.Stringer("request", r))
	if err := s.store.DeleteVariant(ctx, r); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
