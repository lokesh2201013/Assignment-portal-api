package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/assignment-portal/controllers"
	"github.com/lokesh2201013/assignment-portal/middlewares"

)

func AuthRoutes(app *fiber.App) {
	app.Post("/signup", controllers.Register)
	app.Post("/login", controllers.Login)

	app.Use(middleware.AuthMiddleware())

	app.Post("/admin/getassignments", middleware.AdminOnly(controllers.GetAdminAssignments))
	app.Post("/admin/assignments/:id/accept", middleware.AdminOnly(controllers.AcceptAssignment))
	app.Post("/admin/assignments/:id/reject", middleware.AdminOnly(controllers.RejectAssignment))
	app.Post("/admin/assign_assignments" ,middleware.AdminOnly(controllers.AssignTostudents))

	app.Post("/user/upload", middleware.UserOnly(controllers.UploadAssignment))
	app.Get("/user/admins", controllers.GetAllAdmins)
	app.Get("/user/assignments", middleware.UserOnly(controllers.GetUserAssignments))
}
