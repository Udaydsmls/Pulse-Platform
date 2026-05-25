package config

import (
	"os"
	"strings"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	KafkaBrokers     []string
	DynamoDBTable    string
	AWSRegion        string
	SendGridAPIKey   string
	TwilioAccountSID string
	TwilioAuthToken  string
	OTelEndpoint     string
}

// Load reads configuration from environment variables and returns a Config.
func Load() (*Config, error) {
	brokers := []string{"localhost:9092"}
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		brokers = strings.Split(v, ",")
	}

	return &Config{
		KafkaBrokers:     brokers,
		DynamoDBTable:    getEnvOrDefault("DYNAMODB_TABLE", "notification_log"),
		AWSRegion:        getEnvOrDefault("AWS_REGION", "us-east-1"),
		SendGridAPIKey:   os.Getenv("SENDGRID_API_KEY"),
		TwilioAccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		OTelEndpoint:     os.Getenv("OTEL_ENDPOINT"),
	}, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
