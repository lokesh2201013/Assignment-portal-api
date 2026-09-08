package routes

import (
	"github.com/atgsgrouptest/genet-microservice/RAG-service/controllers"
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all HTTP endpoints for the RAG service.
func RegisterRoutes(app *fiber.App, ctrl *controllers.RAGHTTPController) {
	// Preserved endpoints for full backward compatibility
	app.Post("/sendFiles", ctrl.SendFilesHTTP)
	app.Get("/getPromptWithContext", ctrl.GetPromptWithContextHTTP)

	// Additional RESTful and operational endpoints
	app.Post("/getPromptWithContext", ctrl.GetPromptWithContextHTTP)
	app.Get("/healthz", ctrl.HealthCheckHTTP)
	app.Get("/readyz", ctrl.HealthCheckHTTP)
}