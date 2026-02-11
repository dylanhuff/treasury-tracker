package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"treasury-tracker/internal/database"
	"treasury-tracker/internal/handlers"
	"treasury-tracker/internal/services"
)

const (
	serverPort         = ":8080"
	serverReadTimeout  = 15 * time.Second
	serverWriteTimeout = 15 * time.Second
	serverIdleTimeout  = 60 * time.Second
	shutdownTimeout    = 30 * time.Second
	corsMaxAge         = 300
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse DATABASE_URL: %v", err)
	}

	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Database connection established")

	queries := database.New(pool)
	userHandler := handlers.NewUserHandler(queries)

	treasuryService := services.NewTreasuryService()
	treasuryService.WarmCache()

	yieldHandler := handlers.NewYieldHandler(treasuryService)

	txService := services.NewTransactionService(queries, pool)
	txHandlers := handlers.NewTransactionHandlers(txService, queries, treasuryService)
	holdingsHandlers := handlers.NewHoldingsHandlers(queries)

	r := chi.NewRouter()

	// Nginx proxy handles same-origin in production; these support direct API access during dev
	allowedOrigins := []string{
		"http://localhost:5173",
		"http://localhost:5174",
		"http://localhost:80",
		"http://localhost",
		"http://localhost:3000",
	}

	if envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); envOrigins != "" {
		for _, origin := range strings.Split(envOrigins, ",") {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           corsMaxAge,
	}))

	r.Get("/api/v1/users", userHandler.GetAllUsers)
	r.Get("/api/v1/users/{userId}/transactions", txHandlers.GetUserTransactions)
	r.Get("/api/v1/users/{id}/holdings", holdingsHandlers.GetUserHoldings)
	r.Get("/api/yields/historical", yieldHandler.GetHistoricalYields)
	r.Get("/api/yields", yieldHandler.GetYields)
	r.Post("/api/v1/fund", txHandlers.FundHandler)
	r.Post("/api/v1/withdraw", txHandlers.WithdrawHandler)
	r.Post("/api/v1/buy", txHandlers.BuyHandler)
	r.Post("/api/v1/sell", txHandlers.SellHandler)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         serverPort,
		Handler:      r,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}

	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}
