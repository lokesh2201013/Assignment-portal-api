package routes

import (
	"github.com/lokesh2201013/email-service/controllers"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/email-service/middleware"
	"github.com/lokesh2201013/email-service/metrics"
)

func SetupRoutes(app *fiber.App) {
	// public routes no auth required
	app.Post("/register", controllers.Register)
	app.Post("/login", controllers.Login)

	// protected routes auth required
	// middleware only to routes below
	app.Use(middleware.AuthRequired)  
	app.Post("/verify-email-identity", controllers.VerifyEmailID)
	app.Get("/list-verified-identities", controllers.ListVerifiedIdentities)
	app.Get("/list-unverified-identities", controllers.ListunVerifiedIdentities)
	app.Delete("/delete-identity/:email", controllers.DeleteIdentity)
	app.Post("/send-email", controllers.SendEmail)
	app.Post("/create-template", controllers.CreateTemplate)
	//app.Post("/send-custom-email", controllers.SendCustomEmail) 
	
	// metrics routes
	app.Get("/email-metrics/:senderEmail", metrics.GetEmailMetrics)
	app.Get("/admin-email-metrics/:adminName", metrics.GetAdminEmailMetrics)
}
