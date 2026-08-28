package service

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/model"
	"github.com/omar-shahieen/url-shortner/internal/repository"
)

const maxCodeGenerationAttempts = 10

// Service coordinates URL creation and resolution.
type Service struct {
	repository     repository.Repository
	generator      Generator
	aliasValidator AliasValidator
	salt           atomic.Uint64
	now            func() time.Time
}

// New returns a Service backed by repository.
func New(repository repository.Repository) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

// Shorten creates a short URL. A non-empty customAlias is used verbatim after
// validation; otherwise a generated code is retried on collision.
func (s *Service) Shorten(ctx context.Context, originalURL, customAlias string, expiresAt *time.Time) (*model.URL, error) {
	url := &model.URL{
		OriginalURL:   originalURL,
		CreatedAt:     s.now(),
		ExpiresAt:     cloneTime(expiresAt),
		IsCustomAlias: customAlias != "",
	}
	if err := url.Validate(); err != nil {
		return nil, err
	}

	if customAlias != "" {
		if err := s.aliasValidator.Validate(customAlias); err != nil {
			return nil, err
		}

		url.Code = customAlias
		return s.saveCustomAlias(ctx, url)
	}

	for range maxCodeGenerationAttempts {
		url.Code = s.generator.Generate(originalURL, s.salt.Add(1)-1)
		exists, err := s.repository.Exists(ctx, url.Code)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}

		if err := s.repository.Save(ctx, url); err != nil {
			if errors.Is(err, model.ErrAliasTaken) {
				continue
			}
			return nil, err
		}

		return cloneURL(url), nil
	}

	return nil, model.ErrCodeGenerationExhausted
}

func (s *Service) saveCustomAlias(ctx context.Context, url *model.URL) (*model.URL, error) {
	exists, err := s.repository.Exists(ctx, url.Code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, model.ErrAliasTaken
	}

	if err := s.repository.Save(ctx, url); err != nil {
		if errors.Is(err, model.ErrAliasTaken) {
			return nil, model.ErrAliasTaken
		}
		return nil, err
	}

	return cloneURL(url), nil
}

// Resolve returns an active URL and records its click. Expired URLs are never
// counted as clicks.
func (s *Service) Resolve(ctx context.Context, code string) (*model.URL, error) {
	url, err := s.repository.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if url.IsExpired() {
		return nil, model.ErrExpired
	}

	if err := s.repository.IncrementClicks(ctx, code); err != nil {
		return nil, err
	}
	url.ClickCount++
	return url, nil
}

func cloneURL(url *model.URL) *model.URL {
	clone := *url
	clone.ExpiresAt = cloneTime(url.ExpiresAt)
	return &clone
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	clone := *value
	return &clone
}
