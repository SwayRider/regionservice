// auth_test.go verifies that the gRPC AuthInterceptor enforces the endpoint
// profiles declared in server.go: region endpoints require a user JWT or a
// service token with the "region:query" scope, while Ping is public.

package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"testing"
	"time"

	jwt5 "github.com/golang-jwt/jwt/v5"
	geo "github.com/swayrider/protos/common_types/geo"
	healthv1 "github.com/swayrider/protos/health/v1"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/swlib/grpc/interceptors"
	"github.com/swayrider/swlib/jwt"
	log "github.com/swayrider/swlib/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// newAuthTestServer boots an in-memory gRPC server with the real
// AuthInterceptor and returns clients for the region and health services.
// When publicKeyPEM is empty, the key resolver fails token verification.
func newAuthTestServer(t *testing.T, publicKeyPEM string) (
	regionv1.RegionServiceClient, healthv1.HealthServiceClient,
) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)

	getKeys := func() ([]string, error) {
		if publicKeyPEM == "" {
			return nil, fmt.Errorf("no public keys")
		}
		return []string{publicKeyPEM}, nil
	}

	gs := grpc.NewServer(grpc.UnaryInterceptor(
		interceptors.AuthInterceptor(getKeys, log.New())))

	regionv1.RegisterRegionServiceServer(gs, newTestRegionServer(
		&mockRegionQuerier{}, &mockBorderQuerier{}))
	healthv1.RegisterHealthServiceServer(gs, newTestHealthServer())

	go func() {
		_ = gs.Serve(lis)
	}()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return regionv1.NewRegionServiceClient(conn),
		healthv1.NewHealthServiceClient(conn)
}

// TestAuthInterceptorRejectsUnauthenticatedRegionCall verifies that a
// region:query endpoint rejects a call without any token.
func TestAuthInterceptorRejectsUnauthenticatedRegionCall(t *testing.T) {
	regionClient, _ := newAuthTestServer(t, "")

	_, err := regionClient.SearchPoint(context.Background(), &regionv1.SearchPointRequest{
		Location: &geo.Coordinate{Lat: 41.0, Lon: 2.0},
	})
	if err == nil {
		t.Fatal("expected unauthenticated error, got nil")
	}
	if st, _ := status.FromError(err); st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

// TestAuthInterceptorAllowsPublicPing verifies that the Ping endpoint remains
// public and does not require a token.
func TestAuthInterceptorAllowsPublicPing(t *testing.T) {
	_, healthClient := newAuthTestServer(t, "")

	resp, err := healthClient.Ping(context.Background(), &healthv1.PingRequest{})
	if err != nil {
		t.Fatalf("public Ping failed: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil PingResponse")
	}
}

// TestAuthInterceptorAllowsServiceTokenWithRegionQueryScope verifies that a
// service token holding the required scope can call a protected endpoint.
func TestAuthInterceptorAllowsServiceTokenWithRegionQueryScope(t *testing.T) {
	privateKeyPEM, publicKeyPEM := testRSAKeyPair(t)

	_, token, _, err := jwt.GenerateToken(
		"test-service",
		nil,
		jwt.NewSwayRiderServiceClaims(jwt5.ClaimStrings{"region:query"}),
		privateKeyPEM,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to generate service token: %v", err)
	}

	regionClient, _ := newAuthTestServer(t, publicKeyPEM)

	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"authorization", "Bearer "+string(token))

	_, err = regionClient.SearchPoint(ctx, &regionv1.SearchPointRequest{
		Location: &geo.Coordinate{Lat: 41.0, Lon: 2.0},
	})
	if err != nil {
		t.Fatalf("authorized service call failed: %v", err)
	}
}

// TestAuthInterceptorRejectsServiceTokenWithoutRequiredScope verifies that a
// service token missing the required scope is rejected.
func TestAuthInterceptorRejectsServiceTokenWithoutRequiredScope(t *testing.T) {
	privateKeyPEM, publicKeyPEM := testRSAKeyPair(t)

	_, token, _, err := jwt.GenerateToken(
		"test-service",
		nil,
		jwt.NewSwayRiderServiceClaims(jwt5.ClaimStrings{"some:other-scope"}),
		privateKeyPEM,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to generate service token: %v", err)
	}

	regionClient, _ := newAuthTestServer(t, publicKeyPEM)

	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"authorization", "Bearer "+string(token))

	_, err = regionClient.SearchPoint(ctx, &regionv1.SearchPointRequest{
		Location: &geo.Coordinate{Lat: 41.0, Lon: 2.0},
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
	if st, _ := status.FromError(err); st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

// testRSAKeyPair generates an ephemeral RSA keypair and returns the private
// and public keys in PEM encoding, matching the authservice key format.
func testRSAKeyPair(t *testing.T) (privateKeyPEM, publicKeyPEM string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	publicKeyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}))

	return privateKeyPEM, publicKeyPEM
}
