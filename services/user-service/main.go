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

	"github.com/pulse-platform/user-service/gen/pb"
)

// Config holds everything this service reads from the environment.
type Config struct {
	GRPCPort     string
	MetricsPort  string
	DatabaseURL  string
	KafkaBrokers []string
	JWTSecret    string
	OTelEndpoint string
}

// loadConfig reads config from the environment. Anything the service cannot run
// correctly without is required, so we fail at startup instead of misbehaving
// later — an empty JWT secret would sign tokens anyone could forge.
func loadConfig() Config {
	return Config{
		GRPCPort:     envOr("GRPC_PORT", "50051"),
		MetricsPort:  envOr("METRICS_PORT", "9090"),
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		JWTSecret:    mustEnv("JWT_SECRET"),
		OTelEndpoint: os.Getenv("OTEL_ENDPOINT"),
	}
}

func main() {
	cfg := loadConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.OTelEndpoint != "" {
		shutdown, err := initTracing(ctx, "user-service", cfg.OTelEndpoint)
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
		log.Fatalf("create users table: %v", err)
	}

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcprom.UnaryServerInterceptor,
			otelgrpc.UnaryServerInterceptor(),
		),
	)
	pb.RegisterUserServiceServer(grpcServer, &UserServer{
		db:       db,
		producer: producer,
		secret:   cfg.JWTSecret,
	})
	grpcprom.Register(grpcServer)

	go serveMetrics(cfg.MetricsPort)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.GRPCPort, err)
	}

	go func() {
		log.Printf("user-service listening on :%s", cfg.GRPCPort)
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
