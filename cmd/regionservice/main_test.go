package main

import (
	"testing"

	"github.com/swayrider/swlib/app"
)

// TestNewGrpcConfigEnablesAuth guards against the regression where the gRPC
// server was built with app.NoInterceptor, leaving every RegionService
// endpoint public despite the auth declarations in internal/server/server.go.
func TestNewGrpcConfigEnablesAuth(t *testing.T) {
	cfg := newGrpcConfig(app.New("regionservice"))

	if cfg.Interceptors&app.AuthInterceptor == 0 {
		t.Error("expected AuthInterceptor to be enabled in the gRPC config")
	}
	if cfg.JWTPublicKeysFn == nil {
		t.Error("expected a non-nil JWTPublicKeysFn when AuthInterceptor is enabled")
	}
	if cfg.Interceptors&app.RateLimitInterceptor == 0 {
		t.Error("expected RateLimitInterceptor to be enabled in the gRPC config")
	}
}
