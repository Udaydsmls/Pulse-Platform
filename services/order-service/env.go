package main

import (
	"log"
	"os"
)

// mustEnv returns the environment variable or exits. Used for values the service
// cannot safely default (secrets, database URL).
func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return value
}

// envOr returns the environment variable, or fallback when it is not set.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
