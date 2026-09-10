package config

// Logging holds log level and format from the environment.
type Logging struct {
	Level  string // ES_LOG_LEVEL: debug|info|warn|error
	Format string // ES_LOG_FORMAT: text|json
}

func loggingFromEnv() Logging {
	return Logging{
		Level:  envOr("ES_LOG_LEVEL", "info"),
		Format: envOr("ES_LOG_FORMAT", "text"),
	}
}
