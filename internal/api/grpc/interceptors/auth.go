// Package interceptors provides gRPC interceptors for handling authentication and authorization.
package interceptors

import (
	"context"
	"log"
	"time"

	"github.com/KirillZiborov/lnkshortener/internal/api/http/auth"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// contextKey defines a type of a key for storing user ID value in context.
// Defined to avoid staticcheck warnings.
type contextKey string

const (
	// metadataKey is the key in context where we store userID.
	metadataKey contextKey = "userID"

	// cookieHeader is the name of metadata that simulates cookie.
	cookieHeader = "cookie"
)

// AuthInterceptor is a gRPC interceptor that handles users authentification.
// It emulates HTTP cookie-based JWT app.
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract metadata from incoming context.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			// If no metadata extracted then generate new token.
			return createNewToken(ctx, handler, req)
		}

		cookies := md.Get(cookieHeader)
		if len(cookies) == 0 {
			// If no cookie in metadata then generate new token.
			return createNewToken(ctx, handler, req)
		}

		// Parse and validate cookie from metadata.
		userID := auth.GetUserID(cookies[0])
		if userID == "" {
			return nil, status.Errorf(codes.Unauthenticated, "Invalid token in %s", cookieHeader)
		}

		// Put userID in context.
		newCtx := context.WithValue(ctx, metadataKey, userID)

		// Call next handler.
		resp, err := handler(newCtx, req)
		return resp, err
	}
}

// createNewToken generates a new token, sets userID in context, calls the handler, and returns token in response metadata.
func createNewToken(ctx context.Context, handler grpc.UnaryHandler, req interface{}) (interface{}, error) {
	tokenStr, userID, err := generateToken("")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
	}
	newCtx := context.WithValue(ctx, metadataKey, userID)

	// Call the method.
	resp, err := handler(newCtx, req)
	if err != nil {
		return nil, err
	}

	// After method returns, we set the cookie in response metadata.
	sendMD := metadata.Pairs(cookieHeader, tokenStr)
	if err := grpc.SetHeader(newCtx, sendMD); err != nil {
		log.Printf("Failed to set response header: %v", err)
	}

	return resp, nil
}

// generateToken is analogue for GenerateToken function from package auth.
// It creates a new JWT token for a given userID.
// If the provided userID is empty, it generates a new UUID for the user.
// The function returns the signed JWT token string and userID or an error if the process fails.
func generateToken(userID string) (string, string, error) {
	if userID == "" {
		userID = uuid.New().String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(auth.TokenExp)),
		},
		UserID: userID,
	})
	tokenString, err := token.SignedString([]byte(auth.SecretKey))
	if err != nil {
		return "", "", err
	}
	return tokenString, userID, nil
}

// GetUserIDFromContext extracts userID from context in gRPC methods.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	val := ctx.Value(metadataKey)
	userID, ok := val.(string)
	return userID, ok
}
