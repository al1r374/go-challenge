package config_test

import (
	"os"
	"testing"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
)

func TestDefaultModeForUnknownSegment(t *testing.T) {
	m, err := config.NewMemory(map[string]config.Mode{
		"sports": config.ModeExact,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Mode("sports"); got != config.ModeExact {
		t.Fatalf("sports: got %s want exact", got)
	}
	if got := m.Mode("unknown"); got != config.DefaultMode {
		t.Fatalf("unknown: got %s want %s", got, config.DefaultMode)
	}
}

func TestCustomDefaultMode(t *testing.T) {
	m, err := config.NewMemoryWithDefault(nil, config.ModeExact)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Mode("anything"); got != config.ModeExact {
		t.Fatalf("got %s want exact", got)
	}
}

func TestSetModeRuntime(t *testing.T) {
	m, err := config.NewMemory(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetMode("sports", config.ModeExact); err != nil {
		t.Fatal(err)
	}
	if got := m.Mode("sports"); got != config.ModeExact {
		t.Fatalf("got %s want exact", got)
	}
	if err := m.SetMode("sports", "bogus"); err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestParseSegments(t *testing.T) {
	got, err := config.ParseSegments("sports:approximate, premium_users:exact")
	if err != nil {
		t.Fatal(err)
	}
	if got["sports"] != config.ModeApproximate {
		t.Fatalf("sports=%s", got["sports"])
	}
	if got["premium_users"] != config.ModeExact {
		t.Fatalf("premium_users=%s", got["premium_users"])
	}
}

func TestParseSegmentsInvalid(t *testing.T) {
	if _, err := config.ParseSegments("sports"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := config.ParseSegments("sports:nope"); err == nil {
		t.Fatal("expected invalid mode")
	}
}

func TestLoadFromEnv(t *testing.T) {
	m, err := config.Load(config.LoadOptions{
		SegmentsEnv: "sports:approximate,premium_users:exact",
		DefaultMode: config.ModeApproximate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Mode("sports") != config.ModeApproximate {
		t.Fatal("sports")
	}
	if m.Mode("premium_users") != config.ModeExact {
		t.Fatal("premium_users")
	}
	if len(m.All()) != 2 {
		t.Fatalf("all=%v", m.All())
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("ES_ADDR", ":9090")
	t.Setenv("REDIS_ADDR", "10.0.0.1:6379")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("ES_LOG_LEVEL", "debug")
	t.Setenv("ES_LOG_FORMAT", "json")
	t.Setenv("ES_RETENTION_DAYS", "7")
	t.Setenv("ES_DEFAULT_MODE", "exact")
	t.Setenv("ES_SEGMENTS", "sports:approximate")
	t.Setenv("ES_HTTP_READ_HEADER_TIMEOUT_SEC", "3")
	t.Setenv("ES_HTTP_READ_TIMEOUT_SEC", "10")
	t.Setenv("ES_HTTP_WRITE_TIMEOUT_SEC", "20")
	t.Setenv("ES_HTTP_IDLE_TIMEOUT_SEC", "40")

	s := config.FromEnv()
	if s.Server.Addr != ":9090" || s.Redis.Addr != "10.0.0.1:6379" || s.Redis.DB != 2 {
		t.Fatalf("%+v", s)
	}
	if s.Logging.Level != "debug" || s.Logging.Format != "json" || s.RetentionDays != 7 {
		t.Fatalf("%+v", s)
	}
	if s.Server.ReadHeaderTimeout.Seconds() != 3 || s.Server.ReadTimeout.Seconds() != 10 {
		t.Fatalf("timeouts=%+v", s.Server)
	}
	if s.Server.WriteTimeout.Seconds() != 20 || s.Server.IdleTimeout.Seconds() != 40 {
		t.Fatalf("timeouts=%+v", s.Server)
	}
	if s.DefaultMode != config.ModeExact {
		t.Fatalf("%+v", s)
	}
	p, err := s.SegmentProvider()
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode("sports") != config.ModeApproximate || p.Mode("unknown") != config.ModeExact {
		t.Fatal("provider")
	}
}

func TestFromEnvLoadsDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := dir + "/.env"
	if err := os.WriteFile(envPath, []byte("ES_SEGMENTS=sports:exact\nES_DEFAULT_MODE=approximate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Ensure process env does not override .env for this key.
	t.Setenv("ES_SEGMENTS", "")
	_ = os.Unsetenv("ES_SEGMENTS")

	s := config.FromEnv()
	if !s.DotEnvLoaded {
		t.Fatal("expected .env to load")
	}
	if s.SegmentsRaw != "sports:exact" {
		t.Fatalf("segments=%q", s.SegmentsRaw)
	}
	p, err := s.SegmentProvider()
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode("sports") != config.ModeExact {
		t.Fatalf("mode=%s", p.Mode("sports"))
	}
}

func TestFromEnvRetentionDefault(t *testing.T) {
	_ = os.Unsetenv("ES_RETENTION_DAYS")
	s := config.FromEnv()
	if s.RetentionDays != timebucket.DefaultRetentionDays {
		t.Fatalf("got %d", s.RetentionDays)
	}
}
