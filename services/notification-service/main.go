package main

import (
	"context"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds everything this service reads from the environment.
type Config struct {
	DatabaseURL  string
	KafkaBrokers []string
}

func loadConfig() Config {
	return Config{
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
	}
}

// This service has no HTTP API — it only reacts to events.
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
		log.Fatalf("create notifications table: %v", err)
	}

	handler := &Handler{
		db:    db,
		email: NewLogNotifier("email"),
		sms:   NewLogNotifier("sms"),
	}

	// Every topic, because this service notifies on events from all of them.
	consumer := NewConsumer(
		cfg.KafkaBrokers,
		[]string{"user.events", "order.events", "inventory.events", "payment.events"},
		"notification-service",
		handler.Handle,
	)
	defer consumer.Close()

	log.Println("notification-service consuming events")
	consumer.Run(ctx)
}
