package main

import (
	"database/sql"
	"fmt"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
	"log"
	"order-service/internal/api"
	"order-service/internal/config"
	"order-service/internal/repository"
	"order-service/internal/service"
	"order-service/internal/sharding"
	"order-service/migrations"
	"os"
	"time"
)

func connectDBEnv(host, port, user, pass, dbname string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, dbname)

	var db *sql.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("✅ Connected to DB %s", dbname)
				return db, nil
			}
		}
		log.Printf("❌ Retry %d: Failed to connect to DB %s (%s:%s): %v", i+1, dbname, host, port, err)
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("failed to connect to DB %s at %s:%s after retries: %v", dbname, host, port, err)
}

func main() {
	db1, err := connectDBEnv(os.Getenv("DB1_HOST"), os.Getenv("DB1_PORT"), os.Getenv("DB1_USER"), os.Getenv("DB1_PASS"), os.Getenv("DB1_NAME"))
	if err != nil {
		panic(err)
	}
	db2, err := connectDBEnv(os.Getenv("DB2_HOST"), os.Getenv("DB2_PORT"), os.Getenv("DB2_USER"), os.Getenv("DB2_PASS"), os.Getenv("DB2_NAME"))
	if err != nil {
		panic(err)
	}
	db3, err := connectDBEnv(os.Getenv("DB3_HOST"), os.Getenv("DB3_PORT"), os.Getenv("DB3_USER"), os.Getenv("DB3_PASS"), os.Getenv("DB3_NAME"))
	if err != nil {
		panic(err)
	}

	err = migrations.AutoMigrateOrders(3, db1, db2, db3)
	if err != nil {
		log.Fatalf("Failed to migrate orders table: %v", err)
	}

	err = migrations.AutoMigrateProductRequests(3, db1, db2, db3)
	if err != nil {
		log.Fatalf("Failed to migrate product_requests table: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	kafkaWriter := config.NewKafkaWriter("order-topic")

	router := sharding.NewShardRouter(3)

	orderRepo := repository.NewOrderRepository([]*sql.DB{db1, db2, db3}, router)
	orderService := service.NewOrderService(*orderRepo, "http://localhost:8081", "http://localhost:8083", kafkaWriter, rdb)
	orderHandler := api.NewOrderHandler(*orderService)

	e := echo.New()

	limiterConfig := middleware.RateLimiterConfig{
		Skipper: middleware.DefaultSkipper,
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(1),
				Burst:     3,
				ExpiresIn: 3 * time.Minute,
			}),
		IdentifierExtractor: func(context echo.Context) (string, error) {
			return context.Request().RemoteAddr, nil
		},
		ErrorHandler: func(context echo.Context, err error) error {
			return context.JSON(429, map[string]string{"error": "rate limit exceeded"})
		},
		DenyHandler: func(context echo.Context, identifier string, err error) error {
			return context.JSON(429, map[string]string{"error": "rate limit exceeded"})
		},
	}

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RateLimiterWithConfig(limiterConfig))

	e.POST("/orders", orderHandler.CreateOrder)
	e.PUT("/orders", orderHandler.UpdateOrder)
	e.DELETE("/orders/:id", orderHandler.CancelOrder)

	e.GET("/orders/health", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"status":  "ok",
			"service": "order-service",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	e.Logger.Fatal(e.Start(":8082"))
}
