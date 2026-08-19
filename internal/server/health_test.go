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

func TestCheck(t *testing.T) {
	tests := []struct {
		component  string
		wantStatus healthv1.HealthResponse_ServiceStatus
	}{
		{"region", healthv1.HealthResponse_UP},
		{"REGION", healthv1.HealthResponse_UP},
		{"Region", healthv1.HealthResponse_UP},
		{"health", healthv1.HealthResponse_UP},
		{"HEALTH", healthv1.HealthResponse_UP},
		{"Health", healthv1.HealthResponse_UP},
		{"", healthv1.HealthResponse_UP},
		{"unknown", healthv1.HealthResponse_UNKNOWN},
		{"geodata", healthv1.HealthResponse_UNKNOWN},
		{"database", healthv1.HealthResponse_UNKNOWN},
	}

	h := newTestHealthServer()
	for _, tt := range tests {
		t.Run(tt.component, func(t *testing.T) {
			resp, err := h.Check(context.Background(), &healthv1.HealthRequest{
				Component: tt.component,
			})
			if err != nil {
				t.Fatalf("Check(%q) returned error: %v", tt.component, err)
			}
			if resp.Status != tt.wantStatus {
				t.Errorf("Check(%q).Status = %v, want %v", tt.component, resp.Status, tt.wantStatus)
			}
		})
	}
}
