package api

import (
	"net/url"

	"Enhancer_Api/ent"
	"Enhancer_Api/ent/games"

	"github.com/gofiber/fiber/v3"
)

type GameDTO struct {
	Name        string `json:"name"`
	ProcessName string `json:"process_name"`
}

func RegisterGameRoutes(app *fiber.App, client *ent.Client) {
	gamesGroup := app.Group("/api/games")

	gamesGroup.Get("/", getGames(client))
	gamesGroup.Post("/", createGame(client))
	gamesGroup.Put("/:name", updateGame(client))
	gamesGroup.Delete("/:name", deleteGame(client))
}

// GET /api/games
func getGames(client *ent.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		rows, err := client.Games.Query().All(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		resp := make([]GameDTO, len(rows))
		for i, g := range rows {
			resp[i] = GameDTO{
				Name:        g.Name,
				ProcessName: g.ProcessName,
			}
		}

		return c.JSON(resp)
	}
}

// POST /api/games
func createGame(client *ent.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req GameDTO

		err := c.Bind().Body(&req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json body"})
		}

		g, err := client.Games.Create().
			SetName(req.Name).
			SetProcessName(req.ProcessName).
			Save(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(201).JSON(GameDTO{
			Name:        g.Name,
			ProcessName: g.ProcessName,
		})
	}
}

// PUT /api/games/:name
func updateGame(client *ent.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		targetName, err := url.PathUnescape(c.Params("name"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid game name in url"})
		}

		var req GameDTO
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json body"})
		}

		updater := client.Games.Update().Where(games.NameEQ(targetName))
		if req.Name != "" {
			updater.SetName(req.Name)
		}
		if req.ProcessName != "" {
			updater.SetProcessName(req.ProcessName)
		}

		affected, err := updater.Save(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if affected == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "game not found"})
		}

		return c.JSON(req)
	}
}

// DELETE /api/games/:name
func deleteGame(client *ent.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		targetName, err := url.PathUnescape(c.Params("name"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid game name in url"})
		}

		affected, err := client.Games.Delete().Where(games.NameEQ(targetName)).Exec(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if affected == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "game not found"})
		}

		return c.SendStatus(204)
	}
}
