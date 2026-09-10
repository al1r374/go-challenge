package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/service"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/store"
	"github.com/redis/go-redis/v9"
)

// Example of calling Add and Count programmatically (without HTTP).
//
//	go run ./examples/usage.go
func main() {
	addr := env("REDIS_ADDR", "127.0.0.1:6379")
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: %v (start with: docker compose up -d redis)", err)
	}

	cfg, err := config.NewMemory(map[string]config.Mode{
		"sports":         config.ModeApproximate,
		"premium_users":  config.ModeExact,
	})
	if err != nil {
		log.Fatal(err)
	}

	es := service.New(
		cfg,
		store.NewRedisExact(rdb, nil, 14),
		store.NewRedisApproximate(rdb, nil, 14),
	)

	_ = es.Add(ctx, "u104010", "sports")
	_ = es.Add(ctx, "u104011", "sports")
	_ = es.Add(ctx, "u104010", "sports") // duplicate same day — still one unique

	n, err := es.Count(ctx, "sports")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sports unique users (mode=%s): %d\n", es.Mode("sports"), n)

	_ = es.Add(ctx, "u200", "premium_users")
	_ = es.Add(ctx, "u201", "premium_users")
	n, _ = es.Count(ctx, "premium_users")
	fmt.Printf("premium_users unique users (mode=%s): %d\n", es.Mode("premium_users"), n)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
