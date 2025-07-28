package main

import (
	"database/sql"
	"dynamic-pricing-service/internal/api"
	"dynamic-pricing-service/internal/repository"
	"dynamic-pricing-service/internal/service"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"os"
	"time"
)

func connectDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/dynamic-pricing-db")
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	db, err := connectDB()
	if err != nil {
		panic(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Initialize product service
	pricingRepo := repository.NewPricingRepository(db)
	pricingService := service.NewPricingService(pricingRepo, rdb, "http://localhost:8081")
	pricingHandler := api.NewPricingHandler(pricingService)

	// Initialize echo
	e := echo.New()
	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(echojwt.JWT([]byte("secret")))

	// Routes
	e.POST("/pricing", pricingHandler.GetPricing)

	e.GET("/pricing/health", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"status":  "ok",
			"service": "dynamic-pricing-service",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083" // default jika PORT tidak di-set
	}
	e.Logger.Fatal(e.Start(":" + port))
}
