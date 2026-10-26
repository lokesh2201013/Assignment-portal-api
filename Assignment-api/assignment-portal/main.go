package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/google/uuid"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/database"
	pb "github.com/lokesh2201013/proto"
	"github.com/lokesh2201013/repositories"
	"github.com/lokesh2201013/routes"
	"github.com/lokesh2201013/services"
	"github.com/lokesh2201013/utils"
)

type emailNotifier struct{}

func (emailNotifier) SendAssignmentNotification(_ context.Context, req *pb.AssignmentEmailRequest) (string, error) {
	return controllers.SendAssignmentNotification(req)
}

type tenantTokenIssuer struct{}

func (tenantTokenIssuer) GenerateForTenant(userID string, role string, tenantID string) (string, error) {
	return utils.GenerateForTenant(userID, role, tenantID)
}

type localFileSaver struct{}

func (localFileSaver) Save(file *multipart.FileHeader, directory string) (string, error) {
	if file == nil {
		return "", errors.New("upload file is required")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", err
	}
	name := services.SafeUploadedFilename(file.Filename)
	if name == "" || name == "." {
		return "", errors.New("invalid upload filename")
	}
	path := filepath.Join(directory, fmt.Sprintf("%s_%s", uuid.NewString(), name))
	source, err := file.Open()
	if err != nil {
		return "", err
	}
	defer source.Close()
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (localFileSaver) Delete(_ context.Context, objectRef string) error {
	err := os.Remove(objectRef)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (localFileSaver) SignedDownloadURLForTenant(string, string, time.Duration) (string, error) {
	return "", errors.New("signed downloads are unavailable with local file storage")
}

func main() {
	app := fiber.New()
	app.Use(logger.New())

	database.ConnectDB()
	if err := database.InitES(); err != nil {
		log.Fatalf("Failed to initialize Elasticsearch: %v", err)
	}

	sqlDB, err := database.SQLXDB()
	if err != nil {
		log.Fatalf("Failed to initialize SQL repositories: %v", err)
	}
	notifier := emailNotifier{}
	assignmentService := services.NewAssignmentService(
		repositories.NewAssignmentRepository(sqlDB),
		repositories.NewUserRepository(sqlDB),
		notifier,
		localFileSaver{},
	)
	assignmentController := controllers.NewAssignmentController(assignmentService, slog.Default(), "./uploads/images", "./uploads/files")
	tenantService := services.NewTenantService(sqlDB, tenantTokenIssuer{}, notifier, os.Getenv("PUBLIC_APP_URL"))
	tenantController := controllers.NewTenantController(tenantService, slog.Default())

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 60 * time.Second,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Please try again later.",
			})
		},
	}))

	routes.AuthRoutes(app, assignmentController, tenantController)

	port := ":8080"
	log.Printf("Server is running on http://localhost%s\n", port)
	log.Fatal(app.Listen(port))
}
