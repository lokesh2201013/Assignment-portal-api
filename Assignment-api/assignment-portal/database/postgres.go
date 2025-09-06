package database

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/lokesh2201013/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	dsn := "host=" + os.Getenv("DB_HOST") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" port=" + os.Getenv("DB_PORT") +
		" sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to db: %v\n", err)
	}

	// Migrate all models in one call
	if err := db.AutoMigrate(&models.User{}, &models.Assignment{}); err != nil {
		log.Fatalf("AutoMigrate failed: %v\n", err)
	}
    if err := db.AutoMigrate(&models.SubmitAssignment{}); err != nil {
    log.Fatalf("AutoMigrate for SubmitAssignment failed: %v\n", err)
}
   if err := db.AutoMigrate(&models.Video{}); err != nil {
    log.Fatalf("AutoMigrate for Video failed: %v\n", err)
}
	DB = db
	log.Println("Connected to PostgreSQL using GORM")
}
