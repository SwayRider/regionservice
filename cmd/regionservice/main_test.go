package main

import (
	"testing"

	"github.com/swayrider/regionservice/internal/index"
	"github.com/swayrider/swlib/app"
	"github.com/swayrider/swlib/jwtkeys"
	log "github.com/swayrider/swlib/logger"
	"google.golang.org/grpc"
)

// TestNewGrpcConfigEnablesAuth guards against the regression where the gRPC
// server was built with app.NoInterceptor, leaving every RegionService
// endpoint public despite the auth declarations in internal/server/server.go.
func TestNewGrpcConfigEnablesAuth(t *testing.T) {
	cfg := newGrpcConfig(app.New("regionservice"), jwtkeys.New(log.New()))

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

// TestGrpcRegionRegistrar verifies the RegionService gRPC server is actually
// registered on the registrar.
func TestGrpcRegionRegistrar(t *testing.T) {
	srv := grpc.NewServer()
	a := app.New("regionservice").
		WithAppData("RegionIndex", index.NewRegionIndex()).
		WithAppData("BorderIndex", index.NewBorderIndex())

	grpcRegionRegistrar(srv, a)

	if _, ok := srv.GetServiceInfo()["region.v1.RegionService"]; !ok {
		t.Fatalf("expected region.v1.RegionService to be registered, got %v", srv.GetServiceInfo())
	}
}

// TestGrpcHealthRegistrar verifies the HealthService gRPC server is actually
// registered on the registrar.
func TestGrpcHealthRegistrar(t *testing.T) {
	srv := grpc.NewServer()
	a := app.New("regionservice")

	grpcHealthRegistrar(srv, a)

	if _, ok := srv.GetServiceInfo()["health.v1.HealthService"]; !ok {
		t.Fatalf("expected health.v1.HealthService to be registered, got %v", srv.GetServiceInfo())
	}
}
