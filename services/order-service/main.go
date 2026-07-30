package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/pulse-platform/order-service/gen/pb"
)

// Config holds everything this service reads from the environment.
type Config struct {
	GRPCPort     string
	MetricsPort  string
	DatabaseURL  string
	KafkaBrokers []string
	OTelEndpoint string
}

func loadConfig() Config {
	return Config{
		GRPCPort:     envOr("GRPC_PORT", "50052"),
		MetricsPort:  envOr("METRICS_PORT", "9090"),
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		OTelEndpoint: os.Getenv("OTEL_ENDPOINT"),
	}
}

func main() {
	cfg := loadConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.OTelEndpoint != "" {
		shutdown, err := initTracing(ctx, "order-service", cfg.OTelEndpoint)
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
		log.Fatalf("create orders table: %v", err)
	}

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	saga := &Saga{db: db, producer: producer}

	// The saga waits on results from the other two saga participants.
	consumer := NewConsumer(
		cfg.KafkaBrokers,
		[]string{"inventory.events", "payment.events"},
		"order-service",
		saga.Handle,
	)
	defer consumer.Close()

	go consumer.Run(ctx)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcprom.UnaryServerInterceptor,
			otelgrpc.UnaryServerInterceptor(),
		),
	)
	pb.RegisterOrderServiceServer(grpcServer, &OrderServer{db: db, producer: producer, saga: saga})
	grpcprom.Register(grpcServer)

	go serveMetrics(cfg.MetricsPort)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.GRPCPort, err)
	}

	go func() {
		log.Printf("order-service listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("grpc server stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down")
	grpcServer.GracefulStop()
}
