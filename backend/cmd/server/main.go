package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"treasury-tracker/internal/database"
	"treasury-tracker/internal/handlers"
	"treasury-tracker/internal/services"
)

func init() {
	// Ensure decimal.Decimal values marshal as JSON numbers (e.g. 4.52)
	// instead of the default quoted strings (e.g. "4.52").
	decimal.MarshalJSONWithoutQuotes = true
}

const (
	serverPort         = ":8080"
	serverReadTimeout  = 15 * time.Second
	serverWriteTimeout = 15 * time.Second
	serverIdleTimeout  = 60 * time.Second
	shutdownTimeout    = 30 * time.Second
	corsMaxAge         = 300
)

func main() {
	var logger *zap.Logger
	var err error
	if os.Getenv("ENV") == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	if err := godotenv.Load(); err != nil {
		logger.Info("No .env file found")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Fatal("DATABASE_URL environment variable not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logger.Fatal("Unable to parse DATABASE_URL", zap.Error(err))
	}

	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		logger.Fatal("Unable to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("Unable to ping database", zap.Error(err))
	}
	logger.Info("Database connection established")

	queries := database.New(pool)
	userHandler := handlers.NewUserHandler(queries, logger)

	treasuryService := services.NewTreasuryService(queries, pool, logger)
	if err := treasuryService.SyncYields(ctx); err != nil {
		logger.Error("initial yield sync failed", zap.Error(err))
	}
	treasuryService.StartRefreshTicker(ctx)

	yieldHandler := handlers.NewYieldHandler(treasuryService, logger)

	txService := services.NewTransactionService(queries, pool, logger)
	txHandlers := handlers.NewTransactionHandlers(txService, queries, treasuryService, logger)
	holdingsHandlers := handlers.NewHoldingsHandlers(queries, logger)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)

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
		logger.Info("Starting server", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	cancel() // Stop ticker and background work
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	logger.Info("Server exited")
}

func requestLogger(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}
