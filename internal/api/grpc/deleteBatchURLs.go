package grpcapi

import (
	"context"

	"github.com/KirillZiborov/lnkshortener/internal/api/grpc/interceptors"
	"github.com/KirillZiborov/lnkshortener/internal/api/grpc/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BatchDelete is the gRPC equivalent of the HTTP BatchDeleteHandler from package handlers.
func (s *GRPCShortenerServer) BatchDelete(ctx context.Context, req *proto.BatchDeleteRequest) (*proto.BatchDeleteResponse, error) {
	// Get userID from context (using interceptor).
	userID, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing userID in context")
	}

	shortIDs := req.GetShortIds()
	if len(shortIDs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Empty short_ids array")
	}

	// Call to BatchDeleteAsync from app.
	s.svc.BatchDeleteAsync(userID, shortIDs)

	return &proto.BatchDeleteResponse{}, nil
}
