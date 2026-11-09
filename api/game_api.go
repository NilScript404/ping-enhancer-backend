package api

import (
	"net/url"

	"Enhancer_Api/ent"
	"Enhancer_Api/ent/games"

	"github.com/gofiber/fiber/v3"
)

type GameDTO struct {
	Name        string `json:"name" example:"Apex Legends"`
	ProcessName string `json:"process_name" example:"r5apex.exe"`
}

func RegisterGameRoutes(app *fiber.App, client *ent.Client) {
	gamesGroup := app.Group("/api/games")

	gamesGroup.Get("/", getGames(client))
	gamesGroup.Post("/", createGame(client))
	gamesGroup.Put("/:name", updateGame(client))
	gamesGroup.Delete("/:name", deleteGame(client))
}

// getGames godoc
// @Summary      Get all games
// @Description  Retrieves a list of all supported games and their executable process names
// @Tags         games
// @Produce      json
// @Success      200  {array}   GameDTO
// @Router       /api/games [get]
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

// createGame godoc
// @Summary      Create a new game
// @Description  Adds a new game and its target process name to the database
// @Tags         games
// @Accept       json
// @Produce      json
// @Param        game  body      GameDTO  true  "Game payload"
// @Success      201   {object}  GameDTO
// @Router       /api/games [post]
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

// updateGame godoc
// @Summary      Update an existing game
// @Description  Updates a game's name or process name by its current name
// @Tags         games
// @Accept       json
// @Produce      json
// @Param        name  path      string   true  "Current Game Name (URL encoded)"
// @Param        game  body      GameDTO  true  "Updated game fields"
// @Success      200   {object}  GameDTO
// @Router       /api/games/{name} [put]
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

// deleteGame godoc
// @Summary      Delete a game
// @Description  Removes a game from the database by its name
// @Tags         games
// @Produce      json
// @Param        name  path      string  true  "Game Name (URL encoded)"
// @Success      204   "No Content"
// @Router       /api/games/{name} [delete]
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
