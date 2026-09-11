// Package config loads process and segment settings from the environment.
//
//	ES_ADDR, REDIS_*, ES_LOG_*, ES_RETENTION_DAYS, ES_SEGMENTS, ES_DEFAULT_MODE
//
// Trade-off summary (counting modes):
//   - Exact: precise unique counts via one Redis Sorted Set per segment
//     (score = last-seen); memory grows with unique users in the window.
//   - Approximate: HyperLogLog (~12KB fixed per daily bucket); ~0.81% standard error;
//     ideal for high-cardinality segments where exact membership is unnecessary.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
	"github.com/joho/godotenv"
)

// Mode selects the counting strategy for a segment.
type Mode string

const (
	ModeExact       Mode = "exact"
	ModeApproximate Mode = "approximate"
)

// DefaultMode is used when a segment is absent from configuration
// and ES_DEFAULT_MODE is unset.
const DefaultMode = ModeApproximate

// Settings is the full process configuration loaded from the environment.
type Settings struct {
	Server        Server
	Redis         Redis
	Logging       Logging
	RetentionDays int    // ES_RETENTION_DAYS
	SegmentsRaw   string // ES_SEGMENTS
	DefaultMode   Mode   // ES_DEFAULT_MODE
	DotEnvLoaded  bool   // true if a .env file was found and loaded
}

// FromEnv loads settings from a local .env file (if present), then process env vars.
// Already-exported process env vars win over .env (godotenv does not override).
// Restart the process after changing .env — values are read only at startup.
func FromEnv() Settings {
	dotEnvLoaded := godotenv.Load() == nil

	return Settings{
		Server:        serverFromEnv(),
		Redis:         redisFromEnv(),
		Logging:       loggingFromEnv(),
		RetentionDays: timebucket.NormalizeRetention(envInt("ES_RETENTION_DAYS", timebucket.DefaultRetentionDays)),
		SegmentsRaw:   os.Getenv("ES_SEGMENTS"),
		DefaultMode:   Mode(envOr("ES_DEFAULT_MODE", string(DefaultMode))),
		DotEnvLoaded:  dotEnvLoaded,
	}
}

// SegmentProvider builds the in-memory mode map from ES_SEGMENTS / ES_DEFAULT_MODE.
func (s Settings) SegmentProvider() (*Memory, error) {
	return Load(LoadOptions{
		SegmentsEnv: s.SegmentsRaw,
		DefaultMode: s.DefaultMode,
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// LoadOptions controls how segment modes are assembled.
type LoadOptions struct {
	SegmentsEnv string
	DefaultMode Mode
}

// Provider resolves the counting mode for a segment.
type Provider interface {
	Mode(segment string) Mode
	SetMode(segment string, mode Mode) error
	All() map[string]Mode
}

// Memory is a thread-safe in-memory Provider.
type Memory struct {
	mu          sync.RWMutex
	segments    map[string]Mode
	defaultMode Mode
}

// NewMemory builds a Provider from an in-memory map.
func NewMemory(segments map[string]Mode) (*Memory, error) {
	return NewMemoryWithDefault(segments, DefaultMode)
}

// NewMemoryWithDefault is like NewMemory but sets the fallback mode for unknown segments.
func NewMemoryWithDefault(segments map[string]Mode, defaultMode Mode) (*Memory, error) {
	if defaultMode == "" {
		defaultMode = DefaultMode
	}
	if err := validateMode(defaultMode); err != nil {
		return nil, fmt.Errorf("default mode: %w", err)
	}
	m := &Memory{
		segments:    make(map[string]Mode, len(segments)),
		defaultMode: defaultMode,
	}
	for name, mode := range segments {
		if err := validateMode(mode); err != nil {
			return nil, fmt.Errorf("segment %q: %w", name, err)
		}
		m.segments[name] = mode
	}
	return m, nil
}

// Load builds a Provider from ES_SEGMENTS.
func Load(opts LoadOptions) (*Memory, error) {
	segments := make(map[string]Mode)
	if opts.SegmentsEnv != "" {
		fromEnv, err := ParseSegments(opts.SegmentsEnv)
		if err != nil {
			return nil, fmt.Errorf("ES_SEGMENTS: %w", err)
		}
		segments = fromEnv
	}
	return NewMemoryWithDefault(segments, opts.DefaultMode)
}

// ParseSegments parses "sports:approximate,premium_users:exact".
func ParseSegments(raw string) (map[string]Mode, error) {
	out := make(map[string]Mode)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, modeStr, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("invalid entry %q (want name:mode)", part)
		}
		name = strings.TrimSpace(name)
		modeStr = strings.TrimSpace(strings.ToLower(modeStr))
		if name == "" {
			return nil, fmt.Errorf("empty segment name in %q", part)
		}
		mode := Mode(modeStr)
		if err := validateMode(mode); err != nil {
			return nil, fmt.Errorf("segment %q: %w", name, err)
		}
		out[name] = mode
	}
	return out, nil
}

// Mode returns the configured mode, or the provider default if unknown.
func (m *Memory) Mode(segment string) Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mode, ok := m.segments[segment]; ok {
		return mode
	}
	return m.defaultMode
}

// SetMode updates (or creates) the mode for a segment.
func (m *Memory) SetMode(segment string, mode Mode) error {
	if err := validateMode(mode); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.segments[segment] = mode
	return nil
}

// All returns a copy of the configured segment→mode map (excludes defaults).
func (m *Memory) All() map[string]Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]Mode, len(m.segments))
	for k, v := range m.segments {
		out[k] = v
	}
	return out
}

func validateMode(mode Mode) error {
	switch mode {
	case ModeExact, ModeApproximate:
		return nil
	default:
		return fmt.Errorf("invalid mode %q (want %q or %q)", mode, ModeExact, ModeApproximate)
	}
}
