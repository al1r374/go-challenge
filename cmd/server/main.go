package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/api"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/service"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.FromEnv()
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)

	segments, err := cfg.SegmentProvider()
	if err != nil {
		logging.ErrorNoCtx(logging.EventConfigFailed, "err", err)
		os.Exit(1)
	}
	logging.InfoNoCtx(logging.EventConfigLoaded,
		"dotenv_loaded", cfg.DotEnvLoaded,
		"segments_env", cfg.SegmentsRaw,
		"default_mode", cfg.DefaultMode,
		"retention_days", cfg.RetentionDays,
		"segments", len(segments.All()),
		"modes", segments.All(),
	)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		logging.ErrorNoCtx(logging.EventRedisFailed, "addr", cfg.Redis.Addr, "err", err)
		os.Exit(1)
	}
	logging.InfoNoCtx(logging.EventRedisConnected, "addr", cfg.Redis.Addr, "db", cfg.Redis.DB)

	es := service.New(
		segments,
		store.NewRedisExact(rdb, nil, cfg.RetentionDays),
		store.NewRedisApproximate(rdb, nil, cfg.RetentionDays),
	)

	mux := http.NewServeMux()
	api.NewHandler(es).Routes(mux)

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           api.Middleware(mux),
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	go func() {
		logging.InfoNoCtx(logging.EventServerListening,
			"addr", cfg.Server.Addr,
			"redis", cfg.Redis.Addr,
			"default_mode", cfg.DefaultMode,
			"retention_days", cfg.RetentionDays,
			"log_level", cfg.Logging.Level,
			"log_format", cfg.Logging.Format,
			"read_header_timeout", cfg.Server.ReadHeaderTimeout.String(),
			"read_timeout", cfg.Server.ReadTimeout.String(),
			"write_timeout", cfg.Server.WriteTimeout.String(),
			"idle_timeout", cfg.Server.IdleTimeout.String(),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.ErrorNoCtx(logging.EventServerFailed, "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	logging.InfoNoCtx(logging.EventShutdownSignal, "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logging.ErrorNoCtx(logging.EventShutdownHTTP, "err", err)
	}
	if err := rdb.Close(); err != nil {
		logging.ErrorNoCtx(logging.EventRedisClosed, "err", err)
	}
	logging.InfoNoCtx(logging.EventShutdownComplete)
}
