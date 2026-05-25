module github.com/pulse-platform/inventory-service

go 1.22

require (
	github.com/IBM/sarama v1.43.2
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus v1.0.1
	github.com/jackc/pgx/v5 v5.6.0
	github.com/prometheus/client_golang v1.19.1
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.27.0
	go.opentelemetry.io/otel/sdk v1.27.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.64.0
)
