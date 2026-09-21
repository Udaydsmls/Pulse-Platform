package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds everything this service reads from the environment.
type Config struct {
	Port         string
	DatabaseURL  string
	KafkaBrokers []string
	JWTSecret    string
}

func loadConfig() Config {
	return Config{
		Port:         envOr("PORT", "8081"),
		DatabaseURL:  mustEnv("DATABASE_URL"),
		KafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		JWTSecret:    mustEnv("JWT_SECRET"),
	}
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()

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

	server := &Server{db: db, producer: producer, secret: cfg.JWTSecret}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /register", server.Register)
	mux.HandleFunc("POST /login", server.Login)
	mux.HandleFunc("GET /users/{id}", server.GetUser)

	log.Printf("user-service listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
