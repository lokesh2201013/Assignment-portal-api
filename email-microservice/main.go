package main

import (
	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/routes"
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	database.InitDB()
	app := fiber.New()
	routes.SetupRoutes(app)
	log.Fatal(app.Listen(":3000"))
}
