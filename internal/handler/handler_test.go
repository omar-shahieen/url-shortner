package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omar-shahieen/url-shortner/internal/handler"
	"github.com/omar-shahieen/url-shortner/internal/middleware"
	"github.com/omar-shahieen/url-shortner/internal/model"
	"github.com/omar-shahieen/url-shortner/internal/repository/inmemory"
	"github.com/omar-shahieen/url-shortner/internal/routers"
	"github.com/omar-shahieen/url-shortner/internal/service"
)

func TestShortenRedirectAndStats(t *testing.T) {
	router := newRouter()
	body := []byte(`{"originalUrl":"https://example.com/articles/go","customAlias":"go-guide"}`)
	shortenRequest := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	shortenResponse := httptest.NewRecorder()

	router.ServeHTTP(shortenResponse, shortenRequest)
	if shortenResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/shorten status = %d, want %d; body = %s", shortenResponse.Code, http.StatusCreated, shortenResponse.Body)
	}

	var shortened model.URL
	if err := json.NewDecoder(shortenResponse.Body).Decode(&shortened); err != nil {
		t.Fatalf("decode shortened URL: %v", err)
	}
	if shortened.Code != "go-guide" || !shortened.IsCustomAlias {
		t.Errorf("POST /api/shorten response = %#v", shortened)
	}

	redirectRequest := httptest.NewRequest(http.MethodGet, "/"+shortened.Code, nil)
	redirectResponse := httptest.NewRecorder()
	router.ServeHTTP(redirectResponse, redirectRequest)
	if redirectResponse.Code != http.StatusFound {
		t.Fatalf("GET /{code} status = %d, want %d", redirectResponse.Code, http.StatusFound)
	}
	if location := redirectResponse.Header().Get("Location"); location != shortened.OriginalURL {
		t.Errorf("GET /{code} Location = %q, want %q", location, shortened.OriginalURL)
	}

	statsRequest := httptest.NewRequest(http.MethodGet, "/api/stats/"+shortened.Code, nil)
	statsResponse := httptest.NewRecorder()
	router.ServeHTTP(statsResponse, statsRequest)
	if statsResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/stats/{code} status = %d, want %d", statsResponse.Code, http.StatusOK)
	}

	var stats model.URL
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats response: %v", err)
	}
	if stats.ClickCount != 1 {
		t.Errorf("stats click count = %d, want 1", stats.ClickCount)
	}
}

func TestShortenRejectsInvalidRequest(t *testing.T) {
	router := newRouter()

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{`},
		{name: "invalid destination URL", body: `{"originalUrl":"not-a-url"}`},
		{name: "unknown field", body: `{"originalUrl":"https://example.com","unexpected":true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(tt.body))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Errorf("POST /api/shorten status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body)
			}
		})
	}
}

func TestRedirectUnknownCode(t *testing.T) {
	router := newRouter()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Errorf("GET /missing status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestPreviewPage(t *testing.T) {
	router := newRouter()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("GET / Content-Type = %q, want text/html", contentType)
	}
	if !strings.Contains(response.Body.String(), "URL Shortener") {
		t.Error("GET / response does not contain preview page content")
	}
}

type healthyChecker struct{}

func (healthyChecker) PingContext(context.Context) error {
	return nil
}

func newRouter() http.Handler {
	rl := middleware.NewRateLimiter(1000, 1000) // generous limits for tests
	return routers.New(handler.New(service.New(inmemory.New())), healthyChecker{}, rl)
}
