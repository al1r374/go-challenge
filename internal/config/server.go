package config

import "time"

// Server holds HTTP listen address and timeouts (seconds via env).
type Server struct {
	Addr              string        // ES_ADDR
	ReadHeaderTimeout time.Duration // ES_HTTP_READ_HEADER_TIMEOUT_SEC
	ReadTimeout       time.Duration // ES_HTTP_READ_TIMEOUT_SEC
	WriteTimeout      time.Duration // ES_HTTP_WRITE_TIMEOUT_SEC
	IdleTimeout       time.Duration // ES_HTTP_IDLE_TIMEOUT_SEC
}

func serverFromEnv() Server {
	return Server{
		Addr:              envOr("ES_ADDR", ":8080"),
		ReadHeaderTimeout: envDurationSec("ES_HTTP_READ_HEADER_TIMEOUT_SEC", 5),
		ReadTimeout:       envDurationSec("ES_HTTP_READ_TIMEOUT_SEC", 15),
		WriteTimeout:      envDurationSec("ES_HTTP_WRITE_TIMEOUT_SEC", 30),
		IdleTimeout:       envDurationSec("ES_HTTP_IDLE_TIMEOUT_SEC", 60),
	}
}

func envDurationSec(key string, defaultSec int) time.Duration {
	sec := envInt(key, defaultSec)
	if sec < 1 {
		sec = defaultSec
	}
	return time.Duration(sec) * time.Second
}
