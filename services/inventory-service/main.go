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

	"github.com/pulse-platform/inventory-service/gen/pb"
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
		GRPCPort:     envOr("GRPC_PORT", "50053"),
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
		shutdown, err := initTracing(ctx, "inventory-service", cfg.OTelEndpoint)
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
	if err := db.CreateTables(ctx); err != nil {
		log.Fatalf("create inventory tables: %v", err)
	}

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	handler := &Handler{db: db, producer: producer}

	consumer := NewConsumer(cfg.KafkaBrokers, []string{"order.events"}, "inventory-service", handler.Handle)
	defer consumer.Close()

	go consumer.Run(ctx)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcprom.UnaryServerInterceptor,
			otelgrpc.UnaryServerInterceptor(),
		),
	)
	pb.RegisterInventoryServiceServer(grpcServer, &InventoryServer{db: db})
	grpcprom.Register(grpcServer)

	go serveMetrics(cfg.MetricsPort)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.GRPCPort, err)
	}

	go func() {
		log.Printf("inventory-service listening on :%s", cfg.GRPCPort)
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
