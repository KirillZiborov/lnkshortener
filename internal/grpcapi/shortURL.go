package grpcapi

import (
	"context"
	"errors"

	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/grpcapi/interceptors"
	"github.com/KirillZiborov/lnkshortener/internal/grpcapi/proto"
	"github.com/KirillZiborov/lnkshortener/internal/logic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateURL is the gRPC equivalent of the HTTP PostHandler from package handlers.
func (s *GRPCShortenerServer) CreateURL(ctx context.Context, req *proto.CreateURLRequest) (*proto.CreateURLResponse, error) {
	if req.OriginalUrl == "" {
		return nil, status.Error(codes.InvalidArgument, "original_url is empty")
	}

	// Get userID from context (using interceptor).
	userID, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no userID in context")
	}

	// Call CreateShortURL from logic.
	shortURL, err := s.svc.CreateShortURL(ctx, req.OriginalUrl, userID)
	if errors.Is(err, database.ErrorDuplicate) {
		return &proto.CreateURLResponse{ShortUrl: shortURL}, status.Error(codes.AlreadyExists, "URL already exists")
	} else if err != nil {
		return nil, status.Error(codes.Internal, "Failed to save URL")
	}

	return &proto.CreateURLResponse{ShortUrl: shortURL}, nil
}

// BatchShorten is the gRPC equivalent of the HTTP BatchShortenHandler from package handlers.
func (s *GRPCShortenerServer) BatchShorten(ctx context.Context, req *proto.BatchShortenRequest) (*proto.BatchShortenResponse, error) {
	// Get userID from context (using interceptor).
	userID, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no userID in context")
	}

	// Convert input to internal logic structure BatchReq.
	var requests []logic.BatchReq
	for _, item := range req.GetItems() {
		requests = append(requests, logic.BatchReq{
			CorrelationID: item.CorrelationId,
			OriginalURL:   item.OriginalUrl,
		})
	}

	// Call to BatchShorten from logic.
	results, err := s.svc.BatchShorten(ctx, userID, requests)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "BatchShorten error: %v", err)
	}

	// Prepare response.
	respItems := make([]*proto.BatchShortenResponse_Item, 0, len(results))
	for _, r := range results {
		respItems = append(respItems, &proto.BatchShortenResponse_Item{
			CorrelationId: r.CorrelationID,
			ShortUrl:      r.ShortURL,
		})
	}

	return &proto.BatchShortenResponse{
		Items: respItems,
	}, nil
}
