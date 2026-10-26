package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/middlewares"
)

func AuthRoutes(app *fiber.App, assignments *controllers.AssignmentController, tenants *controllers.TenantController) {
	app.Post("/signup", controllers.Register)
	app.Post("/login", controllers.Login)
	app.Post("/tenants", tenants.Create)
	app.Post("/tenant-invitations/accept", tenants.AcceptInvitation)

	protected := app.Group("", middleware.AuthMiddleware())
	protected.Get("/admin/getassignments", middleware.AdminOnly(assignments.GetAdminAssignments))
	protected.Get("/admin/submissions", middleware.AdminOnly(assignments.GetPendingSubmissions))
	protected.Post("/admin/assignments/accept", middleware.AdminOnly(controllers.AcceptAssignment))
	protected.Post("/admin/assignments/reject", middleware.AdminOnly(controllers.RejectAssignment))
	protected.Post("/admin/assign_assignments", middleware.AdminOnly(assignments.AssignToStudents))
	protected.Post("/admin/submissions/grade", middleware.AdminOnly(assignments.GradeSubmission))
	protected.Post("/admin/aid/getPresignedURL", middleware.AdminOnly(controllers.GetPresignedURL))
	protected.Post("/admin/aid/uploadFile", middleware.AdminOnly(controllers.UploadFileHandler))
	protected.Post("/admin/aid/getData", middleware.AdminOnly(controllers.GetData))
	protected.Post("/admin/invitations/bulk", middleware.AdminOnly(tenants.BulkInvite))
	protected.Post("/admin/roster/import", middleware.AdminOnly(tenants.ImportRoster))
	protected.Post("/admin/courses", middleware.AdminOnly(tenants.CreateCourse))
	protected.Get("/admin/courses", middleware.AdminOnly(tenants.ListCourses))
	protected.Get("/admin/users", middleware.AdminOnly(tenants.ListMembers))
	protected.Post("/admin/users/:user_id/status", middleware.AdminOnly(tenants.SetMemberStatus))
	protected.Post("/tenant/invitations", middleware.AdminOnly(tenants.Invite))
	protected.Post("/user/aid/upload", middleware.UserOnly(controllers.GetHelp))
	protected.Post("/user/upload", middleware.UserOnly(assignments.UploadSubmission))
	protected.Get("/user/admins", middleware.UserOnly(controllers.GetAllAdmins))
	protected.Get("/user/assignments/:user_id", middleware.UserOnly(controllers.GetUserAssignments))
	protected.Get("/user/dashboard", middleware.UserOnly(assignments.GetLearnerDashboard))
	protected.Get("/user/submissions/:submission_id/revisions", middleware.UserOnly(assignments.GetSubmissionRevisions))
	protected.Put("/aid/video/:id/status", middleware.AdminOnly(controllers.UpdateVideoStatus))
}
