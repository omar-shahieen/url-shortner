// Package handler contains HTTP handlers for the URL shortener API.
package handler

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/model"
	"github.com/omar-shahieen/url-shortner/internal/service"
)

//go:embed preview.html
var previewHTML []byte

// Handler exposes the URL shortener over HTTP.
type Handler struct {
	service *service.Service
}

// New returns the application's HTTP router.
func New(service *service.Service) http.Handler {
	handler := &Handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handler.preview)
	mux.HandleFunc("POST /api/shorten", handler.shorten)
	mux.HandleFunc("GET /api/stats/{code}", handler.stats)
	mux.HandleFunc("GET /{code}", handler.redirect)
	return mux
}

func (h *Handler) preview(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(previewHTML)
}

type shortenRequest struct {
	OriginalURL string     `json:"originalUrl"`
	CustomAlias string     `json:"customAlias"`
	ExpiresAt   *time.Time `json:"expiresAt"`
}

func (h *Handler) shorten(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var request shortenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	url, err := h.service.Shorten(r.Context(), request.OriginalURL, request.CustomAlias, request.ExpiresAt)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, url)
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	url, err := h.service.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	http.Redirect(w, r, url.OriginalURL, http.StatusFound)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	url, err := h.service.Stats(r.Context(), r.PathValue("code"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, url)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		writeError(w, http.StatusNotFound, "short URL not found")
	case errors.Is(err, model.ErrExpired):
		writeError(w, http.StatusGone, "short URL has expired")
	case errors.Is(err, model.ErrAliasTaken):
		writeError(w, http.StatusConflict, "custom alias is already taken")
	case errors.Is(err, model.ErrInvalidAlias), errors.Is(err, model.ErrInvalidURL):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
