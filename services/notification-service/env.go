package main

import (
	"log"
	"os"
)

// mustEnv returns an environment variable, or exits if it is not set.
func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return value
}

// envOr returns an environment variable, or fallback if it is not set.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
