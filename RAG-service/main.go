package main

import (
	//"log"
	"os"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/Logger"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/routes"
	"go.uber.org/zap"
	"fmt"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"net"
	"log"
	"github.com/gofiber/fiber/v2"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/controllers"
	"github.com/gofiber/fiber/v2/middleware/cors"
)


func main() {
	// Load variables from .env file
	logger.InitLogger() // Initialize the logger
	if err := godotenv.Load(); err != nil {
		logger.Log.Warn("No .env file found, using default values")
	}
    go func() {
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		grpcServer := grpc.NewServer()
		pb.RegisterRAGServiceServer(grpcServer, &controllers.RAGServiceServer{})

		log.Println("🚀 gRPC server listening on :50052")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	//Increase the body limit to 20MB
	//Default is 4MB
    app := fiber.New(fiber.Config{
    BodyLimit: 20 * 1024 * 1024, // 20MB
})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))
    
	//app.Use(logger.New())
	
	// Register routes
	app.Use(logger.ZapLogger())
 
	routes.UseRoutes(app)


	port := os.Getenv("APP_PORT")
	if port == "" {
		fmt.Println("APP_PORT not set, using default port 8001")
		port = "8001"
	}

	if err := app.Listen(":" + port); err != nil {
		logger.Log.Fatal("Error starting server", zap.Error(err))
	}
}
