package main

import (
	_ "embed"

	"github.com/gofiber/fiber/v2"
)

//go:embed docs/openapi.yaml
var openAPISpec []byte

func registerAPIDocs(app *fiber.App) {
	app.Get("/openapi.yaml", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/yaml; charset=utf-8")
		return c.Send(openAPISpec)
	})
	app.Get("/docs", func(c *fiber.Ctx) error {
		c.Type("html", "utf-8")
		return c.SendString(swaggerUIPage)
	})
}

const swaggerUIPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Email Microservice API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>window.onload = () => SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });</script>
</body>
</html>`
