package common

import (
	"os"
	"strconv"
)

// Config holds runtime settings read from environment variables.
type Config struct {
	Port        int
	ServiceName string
	DBHost      string
	DBPort      int
	DBUser      string
	DBPassword  string
	DBName      string
	KafkaBrokers []string
	RedisAddr   string
	OrchestratorURL string
	LogLevel    string
	Approach    string
}

func EnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func EnvIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// LoadConfig reads standard env vars for a service.
func LoadConfig(serviceName string) Config {
	return Config{
		Port:        EnvIntOr("PORT", 8080),
		ServiceName: serviceName,
		DBHost:      EnvOr("DB_HOST", ""),
		DBPort:      EnvIntOr("DB_PORT", 5432),
		DBUser:      EnvOr("DB_USER", ""),
		DBPassword:  EnvOr("DB_PASSWORD", ""),
		DBName:      EnvOr("DB_NAME", ""),
		KafkaBrokers: splitCSV(EnvOr("KAFKA_BROKERS", "saga-kafka:9092")),
		RedisAddr:   EnvOr("REDIS_ADDR", "saga-redis:6379"),
		OrchestratorURL: EnvOr("ORCHESTRATOR_URL", ""),
		LogLevel:    EnvOr("LOG_LEVEL", "info"),
		Approach:    EnvOr("APPROACH", "choreography"),
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}
