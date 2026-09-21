package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds everything this service reads from the environment.
type Config struct {
	Port         string
	DatabaseURL  string
	KafkaBrokers []string
}

func loadConfig() Config {
	return Config{
		Port:         envOr("PORT", "8082"),
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
	}
}

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
		log.Fatalf("create orders table: %v", err)
	}

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	saga := &Saga{db: db, producer: producer}

	// The saga waits on results from inventory-service and payment-service.
	consumer := NewConsumer(
		cfg.KafkaBrokers,
		[]string{"inventory.events", "payment.events"},
		"order-service",
		saga.Handle,
	)
	defer consumer.Close()

	go consumer.Run(ctx)

	server := &Server{db: db, producer: producer, saga: saga}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /orders", server.CreateOrder)
	mux.HandleFunc("GET /orders/{id}", server.GetOrder)
	mux.HandleFunc("POST /orders/{id}/cancel", server.CancelOrder)

	log.Printf("order-service listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
