package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration values for the order service.
type Config struct {
	DatabaseURL  string
	KafkaBrokers []string
	GRPCPort     int
	OTelEndpoint string
}

// Load reads configuration from environment variables and returns a populated Config.
func Load() (*Config, error) {
	grpcPort := 50051
	if portStr := os.Getenv("GRPC_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid GRPC_PORT: %w", err)
		}
		grpcPort = p
	}

	brokers := []string{"localhost:9092"}
	if brokersStr := os.Getenv("KAFKA_BROKERS"); brokersStr != "" {
		brokers = strings.Split(brokersStr, ",")
	}

	return &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		KafkaBrokers: brokers,
		GRPCPort:     grpcPort,
		OTelEndpoint: os.Getenv("OTEL_ENDPOINT"),
	}, nil
}
