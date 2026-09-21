package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds everything this service reads from the environment.
type Config struct {
	DatabaseURL  string
	KafkaBrokers []string
	DeclineOver  float64
}

func loadConfig() Config {
	// The fake card gateway declines anything above this, which is how the
	// rollback path gets tested without a real payment provider.
	declineOver := 5000.0
	if raw := os.Getenv("PAYMENT_DECLINE_OVER"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			log.Fatalf("invalid PAYMENT_DECLINE_OVER %q: %v", raw, err)
		}
		declineOver = parsed
	}

	return Config{
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		DeclineOver:  declineOver,
	}
}

// This service has no HTTP API. It charges in response to inventory.reserved
// and publishes the result, so Kafka is its only interface.
func main() {
	cfg := loadConfig()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	db := &DB{pool: pool}
	if err := db.CreateTable(ctx); err != nil {
		log.Fatalf("create payments table: %v", err)
	}

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	handler := &Handler{
		db:       db,
		gateway:  NewMockGateway(cfg.DeclineOver),
		producer: producer,
	}

	consumer := NewConsumer(cfg.KafkaBrokers, []string{"inventory.events"}, "payment-service", handler.Handle)
	defer consumer.Close()

	log.Println("payment-service consuming inventory.events")
	consumer.Run(ctx)
}
