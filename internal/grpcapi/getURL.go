package grpcapi

import (
	"context"
	"errors"

	"github.com/KirillZiborov/lnkshortener/internal/grpcapi/interceptors"
	"github.com/KirillZiborov/lnkshortener/internal/grpcapi/proto"
	"github.com/KirillZiborov/lnkshortener/internal/logic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetOriginalURL is the gRPC equivalent of the HTTP GetHandler from package handlers.
func (s *GRPCShortenerServer) GetOriginalURL(ctx context.Context, req *proto.GetOriginalURLRequest) (*proto.GetOriginalURLResponse, error) {
	shortID := req.GetShortId()
	if shortID == "" {
		return nil, status.Error(codes.InvalidArgument, "no short_id provided")
	}

	// Call to GetShortURL from logic.
	originalURL, err := s.svc.GetShortURL(ctx, shortID)
	if errors.Is(err, logic.ErrURLNotFound) {
		return nil, status.Error(codes.NotFound, "URL not found")
	} else if errors.Is(err, logic.ErrURLDeleted) {
		return nil, status.Error(codes.FailedPrecondition, "URL is deleted")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
	}

	return &proto.GetOriginalURLResponse{
		OriginalUrl: originalURL,
	}, nil
}

// GetUserURLs is the gRPC equivalent of the HTTP GetUserURLsHandler from package handlers.
func (s *GRPCShortenerServer) GetUserURLs(ctx context.Context, req *proto.GetUserURLsRequest) (*proto.GetUserURLsResponse, error) {
	// Get userID from context (using interceptor).
	userID, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing userID in context")
	}

	// Call to GetUserURLs from logic.
	records, err := s.svc.GetUserURLs(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get a list of user's URLs")
	}

	// Prepare response.
	var respRecords []*proto.URLRecord
	for _, r := range records {
		respRecords = append(respRecords, &proto.URLRecord{
			ShortUrl:    r.ShortURL,
			OriginalUrl: r.OriginalURL,
		})
	}

	return &proto.GetUserURLsResponse{Records: respRecords}, nil
}

// GetStats is the gRPC equivalent of the HTTP GetStatsHandler from package handlers.
func (s *GRPCShortenerServer) GetStats(ctx context.Context, req *proto.GetStatsRequest) (*proto.GetStatsResponse, error) {
	// Get a client IP from context (using interceptor).
	clientIP := interceptors.GetClientIPFromContext(ctx)
	if clientIP == "" {
		return nil, status.Error(codes.PermissionDenied, "No client IP")
	}

	// Call to CheckTrustedSubnet from logic.
	if err := s.svc.CheckTrustedSubnet(clientIP); err != nil {
		switch {
		case errors.Is(err, logic.ErrNoTrustedSubnet),
			errors.Is(err, logic.ErrIPNotInSubnet),
			errors.Is(err, logic.ErrNoClientIP):
			return nil, status.Error(codes.PermissionDenied, "Forbidden")
		default:
			return nil, status.Errorf(codes.Internal, "Invalid trusted subnet: %v", err)
		}
	}

	// Call to GetStats from logic.
	urls, users, err := s.svc.GetStats()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get stats: %v", err)
	}

	return &proto.GetStatsResponse{
		Urls:  int64(urls),
		Users: int64(users),
	}, nil
}
