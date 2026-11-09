package main

import (
	"context"
	"log"
	"os"

	"Enhancer_Api/api"
	"Enhancer_Api/ent"
	"Enhancer_Api/ent/migrate"

	"github.com/gofiber/contrib/v3/swaggerui"
	"github.com/gofiber/fiber/v3"
	_ "github.com/lib/pq"
)

func main() {
	dbConn := os.Getenv("DB_CONN")
	if dbConn == "" {
		dbConn = "host=localhost port=5432 user=postgres dbname=ping_enhancer password=123 sslmode=disable"
	}

	client, err := ent.Open("postgres", dbConn)
	if err != nil {
		log.Fatalf("failed connecting to postgres: %v", err)
	}
	defer client.Close()

	err = client.Schema.Create(
		context.Background(),
		migrate.WithDropColumn(true),
		migrate.WithDropIndex(true),
	)
	if err != nil {
		log.Fatalf("failed creating schema for: %v", err)
	}

	app := fiber.New()

	cfg := swaggerui.Config{
		BasePath: "/api",
		FilePath: "./docs/swagger.json",
		Path:     "docs",
		Title:    "Swagger API Docs",
	}
	app.Use(swaggerui.New(cfg))

	// Register feature routes
	api.RegisterGameRoutes(app, client)
	api.RegisterConfigRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
