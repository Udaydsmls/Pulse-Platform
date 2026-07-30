package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Config holds everything this service reads from the environment.
type Config struct {
	MetricsPort   string
	KafkaBrokers  []string
	DynamoDBTable string
	OTelEndpoint  string
}

func loadConfig() Config {
	return Config{
		MetricsPort:   envOr("METRICS_PORT", "9090"),
		KafkaBrokers:  strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		DynamoDBTable: envOr("DYNAMODB_TABLE", "pulse-notifications"),
		OTelEndpoint:  os.Getenv("OTEL_ENDPOINT"),
	}
}

// This service has no gRPC API — it only reacts to events.
func main() {
	cfg := loadConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.OTelEndpoint != "" {
		shutdown, err := initTracing(ctx, "notification-service", cfg.OTelEndpoint)
		if err != nil {
			log.Printf("tracing disabled: %v", err)
		} else {
			defer shutdown()
		}
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	handler := &Handler{
		store:    &Store{client: dynamodb.NewFromConfig(awsCfg), table: cfg.DynamoDBTable},
		email:    NewLogNotifier("email"),
		sms:      NewLogNotifier("sms"),
		producer: producer,
	}

	// Every topic, because this service notifies on events from all of them.
	consumer := NewConsumer(
		cfg.KafkaBrokers,
		[]string{"user.events", "order.events", "inventory.events", "payment.events"},
		"notification-service",
		handler.Handle,
	)
	defer consumer.Close()

	go serveMetrics(cfg.MetricsPort)
	go consumer.Run(ctx)

	log.Println("notification-service consuming events")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down")
	cancel()
}
