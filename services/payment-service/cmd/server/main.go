package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "github.com/pulse-platform/payment-service/gen/pb"
	"github.com/pulse-platform/payment-service/internal/config"
	"github.com/pulse-platform/payment-service/internal/handler"
	paykafka "github.com/pulse-platform/payment-service/internal/kafka"
	"github.com/pulse-platform/payment-service/internal/repository"
	"github.com/pulse-platform/payment-service/internal/server"
	"github.com/pulse-platform/payment-service/internal/stripe"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.OTelEndpoint != "" {
		tp, err := initTracer(ctx, cfg.OTelEndpoint)
		if err != nil {
			logger.Warn("init tracer", zap.Error(err))
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("connect postgres", zap.Error(err))
	}
	defer pool.Close()

	repo := repository.NewPaymentRepository(pool)
	if err := repo.CreateTable(ctx); err != nil {
		logger.Fatal("create tables", zap.Error(err))
	}

	stripeClient := stripe.NewMockStripeClient()

	producer, err := paykafka.NewKafkaProducer(cfg.KafkaBrokers)
	if err != nil {
		logger.Fatal("create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	invHandler := handler.NewInventoryReservedHandler(repo, stripeClient, producer, logger)

	consumer, err := paykafka.NewConsumer(cfg.KafkaBrokers, "payment-service", invHandler, logger)
	if err != nil {
		logger.Fatal("create kafka consumer", zap.Error(err))
	}
	defer consumer.Close()

	payServer := server.NewPaymentServer(repo, stripeClient, producer, logger)

	reg := prometheus.NewRegistry()
	srvMetrics := grpcprom.NewServerMetrics()
	reg.MustRegister(srvMetrics)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(srvMetrics.UnaryServerInterceptor()),
	)
	pb.RegisterPaymentServiceServer(grpcServer, payServer)
	srvMetrics.InitializeMetrics(grpcServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Fatal("listen gRPC", zap.Error(err))
	}

	errCh := make(chan error, 2)

	go func() {
		logger.Info("gRPC server starting", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	go func() {
		if err := consumer.Start(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer: %w", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received signal", zap.String("signal", sig.String()))
	case err := <-errCh:
		logger.Error("service error", zap.Error(err))
	}

	logger.Info("shutting down")
	cancel()
	grpcServer.GracefulStop()
}

func initTracer(ctx context.Context, endpoint string) (*trace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	return tp, nil
}
