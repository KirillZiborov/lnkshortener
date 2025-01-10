// Package grpcapi provides functionality for handling gRPC communication with the URL shortener service.
// It includes interfaces and structs for defining gRPC services and servers, as well as methods
// for interacting with the URL shortener service.
package grpcapi

import (
	"github.com/KirillZiborov/lnkshortener/internal/grpcapi/proto"
	"github.com/KirillZiborov/lnkshortener/internal/logic"
)

// GRPCShortenerServer supports all neccessary server methods.
type GRPCShortenerServer struct {
	proto.UnimplementedShortenerServiceServer
	svc *logic.ShortenerService
}

// NewGRPCShortenerServer creates a new instance of the GRPCShortenerServer struct with the provided service.
// It initializes the service field of the GRPCShortenerServer struct with the given
// service instance and returns a pointer to the newly created GRPCShortenerServer instance.
func NewGRPCShortenerServer(svc *logic.ShortenerService) *GRPCShortenerServer {
	return &GRPCShortenerServer{svc: svc}
}
