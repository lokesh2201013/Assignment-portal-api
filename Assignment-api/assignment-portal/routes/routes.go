package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/middlewares"

)

func AuthRoutes(app *fiber.App) {
	app.Post("/signup", controllers.Register)
	app.Post("/login", controllers.Login)

	app.Use(middleware.AuthMiddleware())

    app.Get("/admin/getassignments", middleware.AdminOnly(controllers.GetAdminAssignments))

// Admin: Accept/reject assignments (no changes needed here)
    app.Post("/admin/assignments/accept", middleware.AdminOnly(controllers.AcceptAssignment))
    app.Post("/admin/assignments/reject", middleware.AdminOnly(controllers.RejectAssignment))
	app.Post("/admin/assign_assignments" ,middleware.AdminOnly(controllers.AssignTostudents))
     
//Admin aid 
    app.Post(("/admin/aid/video"),middleware.AdminOnly(controllers.UploadVideo))
    app.Post("/admin/aid/uploadFile", middleware.AdminOnly(controllers.UploadFileHandler))
    app.Get("/admin//aid/getData", middleware.AdminOnly(controllers.GetData))
	//Use multipart form data for this request to send the files and images and comments
	app.Get(("/user/aid/getVidData"),middleware.UserOnly(controllers.GetVidData))
	app.Get("/user/aid/upload", middleware.UserOnly(controllers.GetHelp))
	app.Post("/user/upload", middleware.UserOnly(controllers.UploadAssignment))
	app.Get("/user/admins", controllers.GetAllAdmins)
	app.Get("/user/assignments/:user_id", middleware.UserOnly(controllers.GetUserAssignments))
	//Give a user_id as a query param
	app.Get("/user/assignments", middleware.UserOnly(controllers.GetUserAssignments))
}
