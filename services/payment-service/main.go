package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds everything this service reads from the environment.
type Config struct {
	MetricsPort  string
	DatabaseURL  string
	KafkaBrokers []string
	OTelEndpoint string
	DeclineOver  float64
}

func loadConfig() Config {
	// The mock gateway declines anything above this, which is how the saga's
	// rollback path gets exercised without a real Stripe account.
	declineOver := 5000.0
	if raw := os.Getenv("PAYMENT_DECLINE_OVER"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			log.Fatalf("invalid PAYMENT_DECLINE_OVER %q: %v", raw, err)
		}
		declineOver = parsed
	}

	return Config{
		MetricsPort:  envOr("METRICS_PORT", "9090"),
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		OTelEndpoint: os.Getenv("OTEL_ENDPOINT"),
		DeclineOver:  declineOver,
	}
}

// This service has no gRPC API — it charges in response to inventory.reserved
// and announces the result, so Kafka is its only interface.
func main() {
	cfg := loadConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.OTelEndpoint != "" {
		shutdown, err := initTracing(ctx, "payment-service", cfg.OTelEndpoint)
		if err != nil {
			log.Printf("tracing disabled: %v", err)
		} else {
			defer shutdown()
		}
	}

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

	go serveMetrics(cfg.MetricsPort)
	go consumer.Run(ctx)

	log.Println("payment-service consuming inventory.events")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down")
	cancel()
}
