package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/pulse-platform/user-service/gen/pb"
	"github.com/pulse-platform/user-service/internal/auth"
	"github.com/pulse-platform/user-service/internal/config"
	"github.com/pulse-platform/user-service/internal/kafka"
	"github.com/pulse-platform/user-service/internal/repository"
	"github.com/pulse-platform/user-service/internal/server"
)

func initTracer(ctx context.Context, endpoint string) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName("user-service")),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var tp *sdktrace.TracerProvider
	if cfg.OTelEndpoint != "" {
		tp, err = initTracer(ctx, cfg.OTelEndpoint)
		if err != nil {
			logger.Warn("init tracer", zap.Error(err))
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("connect to database", zap.Error(err))
	}
	defer pool.Close()

	repo := repository.NewUserRepository(pool)
	if err := repo.CreateTable(ctx); err != nil {
		logger.Fatal("run schema migration", zap.Error(err))
	}

	producer, err := kafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		logger.Fatal("create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	googleOAuth := auth.NewGoogleProvider(
		cfg.OAuthGoogleClientID,
		cfg.OAuthGoogleClientSecret,
		"http://localhost/auth/google/callback",
	)
	githubOAuth := auth.NewGithubProvider(
		cfg.OAuthGithubClientID,
		cfg.OAuthGithubClientSecret,
		"http://localhost/auth/github/callback",
	)

	userServer := server.NewUserServer(repo, producer, jwtManager, cfg, logger, googleOAuth, githubOAuth)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpc_prometheus.UnaryServerInterceptor,
			otelgrpc.UnaryServerInterceptor(),
		),
	)

	pb.RegisterUserServiceServer(grpcServer, userServer)
	grpc_prometheus.Register(grpcServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Fatal("listen", zap.Int("port", cfg.GRPCPort), zap.Error(err))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("starting gRPC server", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down")
	grpcServer.GracefulStop()
}
