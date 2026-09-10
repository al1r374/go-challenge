// Package service implements the Estimation Service (ES) domain logic.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/store"
)

// Estimation is the core ES API: Add membership and Count unique users.
// Segment modes come from config (ES_SEGMENTS), not from this interface.
type Estimation interface {
	Add(ctx context.Context, userID, segment string) error
	Count(ctx context.Context, segment string) (int64, error)
	Mode(segment string) config.Mode
}

// Service routes Add/Count to Exact or Approximate stores based on config.
type Service struct {
	cfg    config.Provider
	exact  store.ExactStore
	approx store.ApproximateStore
}

// New wires config and storage backends.
func New(cfg config.Provider, exact store.ExactStore, approx store.ApproximateStore) *Service {
	return &Service{cfg: cfg, exact: exact, approx: approx}
}

// Add records (userID, segment) using the segment's configured mode.
func (s *Service) Add(ctx context.Context, userID, segment string) error {
	if userID == "" || segment == "" {
		return fmt.Errorf("user_id and segment are required")
	}
	mode := s.cfg.Mode(segment)
	start := time.Now()
	var err error
	switch mode {
	case config.ModeExact:
		err = s.exact.Add(ctx, userID, segment)
	default:
		err = s.approx.Add(ctx, userID, segment)
	}
	attrs := []any{
		"component", "estimation",
		"user_id", userID,
		"segment", segment,
		"mode", mode,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		logging.Error(ctx, logging.EventESAddFailed, append(attrs, "code", logging.CodeStorageError, "err", err)...)
		return err
	}
	logging.Info(ctx, logging.EventESAdd, attrs...)
	return nil
}

// Count returns unique active users for segment (exact or approximate per config).
func (s *Service) Count(ctx context.Context, segment string) (int64, error) {
	if segment == "" {
		return 0, fmt.Errorf("segment is required")
	}
	mode := s.cfg.Mode(segment)
	start := time.Now()
	var (
		n   int64
		err error
	)
	switch mode {
	case config.ModeExact:
		n, err = s.exact.Count(ctx, segment)
	default:
		n, err = s.approx.Count(ctx, segment)
	}
	attrs := []any{
		"component", "estimation",
		"segment", segment,
		"mode", mode,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		logging.Error(ctx, logging.EventESCountFailed, append(attrs, "code", logging.CodeStorageError, "err", err)...)
		return 0, err
	}
	logging.Info(ctx, logging.EventESCount, append(attrs, "count", n)...)
	return n, nil
}

// Mode returns the effective counting mode for segment (including default).
func (s *Service) Mode(segment string) config.Mode {
	return s.cfg.Mode(segment)
}
