package routers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omar-shahieen/url-shortner/internal/handler"
	"github.com/omar-shahieen/url-shortner/internal/repository/inmemory"
	"github.com/omar-shahieen/url-shortner/internal/service"
)

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		healthCheck HealthChecker
		wantStatus int
	}{
		{name: "database available", healthCheck: healthChecker{} , wantStatus: http.StatusOK},
		{name: "database unavailable", healthCheck: failingHealthChecker{}, wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := New(handler.New(service.New(inmemory.New())), tt.healthCheck)
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Errorf("GET /health status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}

type healthChecker struct{}

func (healthChecker) PingContext(context.Context) error {
	return nil
}

type failingHealthChecker struct{}

func (failingHealthChecker) PingContext(context.Context) error {
	return errors.New("database unavailable")
}
