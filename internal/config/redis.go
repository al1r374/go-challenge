package config

// Redis holds Redis connection settings from the environment.
type Redis struct {
	Addr     string // REDIS_ADDR
	Password string // REDIS_PASSWORD
	DB       int    // REDIS_DB
}

func redisFromEnv() Redis {
	return Redis{
		Addr:     envOr("REDIS_ADDR", "127.0.0.1:6379"),
		Password: envOr("REDIS_PASSWORD", ""),
		DB:       envInt("REDIS_DB", 0),
	}
}
