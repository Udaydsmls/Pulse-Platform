package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	DatabaseURL  string
	KafkaBrokers []string
	GRPCPort     int
	OTelEndpoint string
}

// Load reads configuration from environment variables and returns a Config.
func Load() (*Config, error) {
	grpcPort := 50052
	if v := os.Getenv("GRPC_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid GRPC_PORT: %w", err)
		}
		grpcPort = p
	}

	brokers := []string{"localhost:9092"}
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		brokers = strings.Split(v, ",")
	}

	return &Config{
		DatabaseURL:  getEnvOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/inventory?sslmode=disable"),
		KafkaBrokers: brokers,
		GRPCPort:     grpcPort,
		OTelEndpoint: os.Getenv("OTEL_ENDPOINT"),
	}, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
