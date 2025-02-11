package main

import (
	"log"
      "time"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/lokesh2201013/assignment-portal/database"
	"github.com/lokesh2201013/assignment-portal/routes"
)

func main() {
	// Initialize Fiber app
	app := fiber.New()


	app.Use(logger.New())

	database.ConnectDB()

	// Enable CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	
	app.Use(limiter.New(limiter.Config{
		Max:        100,             
		Expiration: 60 * time.Second, 
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Please try again later.",
			})
		},
	}))


	routes.AuthRoutes(app)

	
	port := ":8080" 
	log.Printf("Server is running on http://localhost%s\n", port)
	log.Fatal(app.Listen(port))
}
