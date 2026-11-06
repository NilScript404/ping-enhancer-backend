package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"Enhancer_Api/ent"
	"Enhancer_Api/ent/migrate"
	"Enhancer_Api/ent/user"

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
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	defer client.Close()

	err = client.Schema.Create(context.Background(), migrate.WithDropColumn(true), migrate.WithDropIndex(true))
	if err != nil {
		log.Fatalf("failed creating schema resource: %v", err)
	}

	app := fiber.New()

	app.Use(fiber_middleware)

	app.Get("/create_user", create_user(client))
	app.Get("/get_user/:id", get_user_by_id)
	app.Get("/get_user", get_user(client))

	log.Fatal(app.Listen(":3000"))
}

func CreateUser(ctx context.Context, client *ent.Client) (*ent.User, error) {
	u, err := client.User.Create().
		SetName("first_user").
		SetEmail("test_email").
		SetPassword("test_password").
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		SetSessionToken("whatevertoken").
		Save(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed creating user in database: %w", err)
	}

	log.Println("user was created: ", u)
	return u, nil
}

func QueryUser(ctx context.Context, client *ent.Client) (*ent.User, error) {
	u, err := client.User.
		Query().
		Where(user.Name("first_user")).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed querying user: %w", err)
	}
	log.Println("user returned: ", u)
	return u, nil
}

func fiber_middleware(c fiber.Ctx) error {
	c.Set("Meow", "a cat meowed")
	return c.Next()
}

func get_user(client *ent.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		u, err := client.User.
			Query().
			Where(user.Name("first_user")).
			Only(c)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString(u.Email)
	}
}

func get_user_by_id(c fiber.Ctx) error {
	user_id := c.Params("id")
	resp := "the user id is: " + user_id
	return c.SendString(resp)
}

func create_user(client *ent.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		_, err := CreateUser(c.Context(), client)
		if err != nil {
			log.Println(err)
			return c.Status(500).SendString("Failed to create user")
		}

		return c.SendString("created the user")
	}
}
