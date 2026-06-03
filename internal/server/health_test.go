package server

import (
	"context"
	"testing"

	healthv1 "github.com/swayrider/protos/health/v1"
)

func TestPing(t *testing.T) {
	h := newTestHealthServer()
	resp, err := h.Ping(context.Background(), &healthv1.PingRequest{})
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Ping returned nil response")
	}
}
