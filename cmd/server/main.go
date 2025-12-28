package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ccxt-simulator/internal/config"
	"github.com/ccxt-simulator/internal/handler"
	exchangeBinance "github.com/ccxt-simulator/internal/handler/exchange/binance"
	exchangeBitget "github.com/ccxt-simulator/internal/handler/exchange/bitget"
	exchangeBybit "github.com/ccxt-simulator/internal/handler/exchange/bybit"
	exchangeHyperliquid "github.com/ccxt-simulator/internal/handler/exchange/hyperliquid"
	exchangeOKX "github.com/ccxt-simulator/internal/handler/exchange/okx"
	"github.com/ccxt-simulator/internal/middleware"
	"github.com/ccxt-simulator/internal/models"
	"github.com/ccxt-simulator/internal/repository"
	"github.com/ccxt-simulator/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Build info (injected at build time via -ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Redis
	rdb := initRedis(cfg)

	// Auto migrate database
	if err := autoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	accountRepo := repository.NewAccountRepository(db)
	positionRepo := repository.NewPositionRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	tradeRepo := repository.NewTradeRepository(db)
	closedPnLRepo := repository.NewClosedPnLRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo, cfg.JWT)
	accountService := service.NewAccountService(
		accountRepo,
		cfg.Encryption,
		"yourdomain.com", // Base URL for endpoint generation
	)

	// Initialize price service
	priceService := service.NewPriceService(rdb)

	// Initialize trading service
	tradingService := service.NewTradingService(
		accountRepo,
		positionRepo,
		orderRepo,
		tradeRepo,
		closedPnLRepo,
		priceService,
	)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	accountHandler := handler.NewAccountHandler(accountService)
	priceHandler := handler.NewPriceHandler(priceService)
	tradingHandler := handler.NewTradingHandler(tradingService, accountService)

	// Create Gin router
	router := gin.Default()

	// Add request logging middleware (logs all requests with error details)
	router.Use(middleware.RequestLoggerMiddleware())

	// Uncomment below for detailed debug logging (verbose output)
	// router.Use(middleware.DebugLoggerMiddleware())

	// Add CORS middleware
	router.Use(corsMiddleware())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"version":    Version,
			"commit":     Commit,
			"build_time": BuildTime,
			"time":       time.Now().Unix(),
			"exchanges":  priceService.GetExchangeStatus(),
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		authHandler.RegisterRoutes(v1)

		// Account routes (protected)
		authMiddleware := middleware.AuthMiddleware(authService)
		accountHandler.RegisterRoutes(v1, authMiddleware)

		// Price routes (public)
		priceHandler.RegisterRoutes(v1)

		// Trading routes (protected)
		tradingHandler.RegisterRoutes(v1, authMiddleware)
	}

	// Exchange-compatible API routes
	// These endpoints mirror the original exchange APIs for compatibility

	// Initialize ExchangeInfoService for caching exchange data
	exchangeInfoService := service.NewExchangeInfoService(rdb)
	go exchangeInfoService.Start(context.Background())

	// Binance compatible routes (/fapi/v1/*, /fapi/v2/*)
	binanceHandler := exchangeBinance.NewHandler(tradingService, priceService, exchangeInfoService)
	binanceAuthMiddleware := middleware.BinanceAuthMiddleware(accountService, cfg.Encryption.AESKey)
	binanceHandler.RegisterRoutes(router, binanceAuthMiddleware)

	// OKX compatible routes (/api/v5/*)
	okxHandler := exchangeOKX.NewHandler(tradingService, priceService, exchangeInfoService)
	okxAuthMiddleware := middleware.OKXAuthMiddleware(accountService, cfg.Encryption.AESKey)
	okxHandler.RegisterRoutes(router, okxAuthMiddleware)

	// Bybit compatible routes (/v5/*)
	bybitHandler := exchangeBybit.NewHandler(tradingService, priceService, exchangeInfoService)
	bybitAuthMiddleware := middleware.BybitAuthMiddleware(accountService, cfg.Encryption.AESKey)
	bybitHandler.RegisterRoutes(router, bybitAuthMiddleware)

	// Bitget compatible routes (/api/v2/mix/*)
	bitgetHandler := exchangeBitget.NewHandler(tradingService, priceService, exchangeInfoService)
	bitgetAuthMiddleware := middleware.BitgetAuthMiddleware(accountService, cfg.Encryption.AESKey)
	bitgetHandler.RegisterRoutes(router, bitgetAuthMiddleware)

	// Hyperliquid compatible routes (/info, /exchange)
	hyperliquidHandler := exchangeHyperliquid.NewHandler(tradingService, priceService, exchangeInfoService)
	hyperliquidAuthMiddleware := middleware.HyperliquidAuthMiddleware(accountService, cfg.Encryption.AESKey)
	hyperliquidHandler.RegisterRoutes(router, hyperliquidAuthMiddleware)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start price service
	ctx := context.Background()
	if err := priceService.Start(ctx); err != nil {
		log.Printf("Warning: Failed to start price service: %v", err)
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop price service
	priceService.Stop()

	// Graceful shutdown with 10 second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close Redis connection
	if err := rdb.Close(); err != nil {
		log.Printf("Error closing Redis connection: %v", err)
	}

	log.Println("Server exited properly")
}

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	gormLogger := logger.Default.LogMode(logger.Info)
	if cfg.Server.Mode == "release" {
		gormLogger = logger.Default.LogMode(logger.Warn)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func initRedis(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Position{},
		&models.Order{},
		&models.Trade{},
		&models.ClosedPnLRecord{},
	)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-MBX-APIKEY, OK-ACCESS-KEY, X-BAPI-API-KEY")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
