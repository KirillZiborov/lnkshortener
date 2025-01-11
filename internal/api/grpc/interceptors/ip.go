package interceptors

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// CtxClientIPKey defines a type of a key for storing IP value in context.
// Defined to avoid staticcheck warnings.
type ctxKey string

// CtxClientIPKey is the key in context where we store client IP.
const CtxClientIPKey ctxKey = "clientIP"

// IPInterceptor extracts an IP address from metadata or from peer.Address.
// It saves the found IP to the context.
func IPInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		var clientIP string

		// Extract x-real-ip from metadata.
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			xRealIPHeader := md.Get("x-real-ip")
			if len(xRealIPHeader) > 0 {
				clientIP = strings.TrimSpace(xRealIPHeader[0])
			}
		}

		// Extratct IP from peer.Addr if it is not found in metadata.
		if clientIP == "" {
			if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
				clientIP = p.Addr.String()
			}
		}

		// Save IP to the context.
		newCtx := context.WithValue(ctx, CtxClientIPKey, clientIP)

		// Call next handler.
		return handler(newCtx, req)
	}
}

// GetClientIPFromContext extracts IP from context in gRPC methods.
func GetClientIPFromContext(ctx context.Context) string {
	val := ctx.Value(CtxClientIPKey)
	if ip, ok := val.(string); ok {
		return ip
	}
	return ""
}
