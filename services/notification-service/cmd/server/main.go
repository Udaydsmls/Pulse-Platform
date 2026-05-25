package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"github.com/pulse-platform/notification-service/internal/config"
	"github.com/pulse-platform/notification-service/internal/handler"
	notkafka "github.com/pulse-platform/notification-service/internal/kafka"
	"github.com/pulse-platform/notification-service/internal/notification"
	"github.com/pulse-platform/notification-service/internal/repository"
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

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		logger.Fatal("load aws config", zap.Error(err))
	}
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	repo := repository.NewNotificationRepository(dynamoClient, cfg.DynamoDBTable)

	emailClient := notification.NewSendGridClient(cfg.SendGridAPIKey, logger)
	smsClient := notification.NewTwilioClient(cfg.TwilioAccountSID, cfg.TwilioAuthToken, logger)

	notifHandler := handler.NewNotificationHandler(emailClient, smsClient, repo, logger)

	consumer, err := notkafka.NewConsumer(cfg.KafkaBrokers, "notification-service", notifHandler, logger)
	if err != nil {
		logger.Fatal("create kafka consumer", zap.Error(err))
	}
	defer consumer.Close()

	errCh := make(chan error, 1)

	go func() {
		logger.Info("notification service starting, consuming all domain events")
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
