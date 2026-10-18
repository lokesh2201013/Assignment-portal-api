package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/middlewares"
)

func AuthRoutes(app *fiber.App) {
	app.Post("/signup", controllers.Register)
	app.Post("/login", controllers.Login)

	protected := app.Group("", middleware.AuthMiddleware())
	protected.Get("/admin/getassignments", middleware.AdminOnly(controllers.GetSubmittedAssignments))
	protected.Get("/admin/submissions", middleware.AdminOnly(controllers.GetSubmittedAssignments))
	protected.Post("/admin/assignments/accept", middleware.AdminOnly(controllers.AcceptAssignment))
	protected.Post("/admin/assignments/reject", middleware.AdminOnly(controllers.RejectAssignment))
	protected.Post("/admin/assign_assignments", middleware.AdminOnly(controllers.AssignToStudents))
	protected.Post("/admin/aid/getPresignedURL", middleware.AdminOnly(controllers.GetPresignedURL))
	protected.Post("/admin/aid/uploadFile", middleware.AdminOnly(controllers.UploadFileHandler))
	protected.Get("/admin/aid/getData", middleware.AdminOnly(controllers.GetData))
	protected.Get("/user/aid/upload", middleware.UserOnly(controllers.GetHelp))
	protected.Post("/user/upload", middleware.UserOnly(controllers.UploadAssignment))
	protected.Get("/user/admins", middleware.UserOnly(controllers.GetAllAdmins))
	protected.Get("/user/assignments/:user_id", middleware.UserOnly(controllers.GetUserAssignments))
	protected.Get("/aid/video/status", middleware.AdminOnly(controllers.UpdateVideoStatus))
}
