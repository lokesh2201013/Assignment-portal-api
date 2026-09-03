package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/middlewares"
	"github.com/lokesh2201013/utils"
	"log/slog"
)

type Controllers struct {
	Users       *controllers.UserController
	Assignments *controllers.AssignmentController
	Aid         *controllers.AidController
	JWT         *utils.JWTManager
	Logger      *slog.Logger
}

func AuthRoutes(app *fiber.App, handlers Controllers) {
	app.Post("/signup", handlers.Users.Register)
	app.Post("/login", handlers.Users.Login)

	app.Use(middleware.AuthMiddleware(handlers.JWT, handlers.Logger))

	app.Get("/admin/getassignments", middleware.AdminOnly(handlers.Assignments.GetAdminAssignments))
	app.Get("/admin/submissions", middleware.AdminOnly(handlers.Assignments.GetSubmittedAssignments))

	app.Post("/admin/assignments/accept", middleware.AdminOnly(handlers.Assignments.AcceptAssignment))
	app.Post("/admin/assignments/reject", middleware.AdminOnly(handlers.Assignments.RejectAssignment))
	app.Post("/admin/assign_assignments", middleware.AdminOnly(handlers.Assignments.AssignToStudents))

	app.Post("/admin/aid/getPresignedURL", middleware.AdminOnly(handlers.Aid.GetPresignedURL))
	app.Post("/admin/aid/uploadFile", middleware.AdminOnly(handlers.Aid.UploadFileHandler))
	app.Get("/admin/aid/getData", middleware.AdminOnly(handlers.Aid.GetData))
	app.Get("/user/aid/upload", middleware.UserOnly(handlers.Aid.GetHelp))
	app.Post("/user/upload", middleware.UserOnly(handlers.Assignments.UploadAssignment))
	app.Get("/user/admins", handlers.Users.GetAllAdmins)
	app.Get("/user/assignments/:user_id", middleware.UserOnly(handlers.Assignments.GetUserAssignments))
	app.Get("/user/assignments", middleware.UserOnly(handlers.Assignments.GetUserAssignments))

	app.Put("/aid/video/status", middleware.AdminOnly(handlers.Aid.UpdateVideoStatus))
}
