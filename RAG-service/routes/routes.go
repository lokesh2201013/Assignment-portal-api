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
	app.Get("/collections", ctrl.ListCollectionsHTTP)
	app.Get("/documents", ctrl.ListDocumentsHTTP)
	app.Delete("/documents/:document_id", ctrl.DeleteDocumentHTTP)
	app.Patch("/documents/:document_id", ctrl.UpdateDocumentStatusHTTP)
	app.Put("/documents/:document_id", ctrl.UpdateDocumentStatusHTTP)
}
