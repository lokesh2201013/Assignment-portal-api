package database

import (
    "log"
    "github.com/joho/godotenv"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "github.com/lokesh2201013/email-service/models"
    "os"
)

var DB *gorm.DB

func InitDB() {
    // Load environment variables from .env file
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }

    // Read database connection details from environment variables
    dsn := "host=" + os.Getenv("DB_HOST") + 
           " user=" + os.Getenv("DB_USER") + 
           " password=" + os.Getenv("DB_PASSWORD") + 
           " dbname=" + os.Getenv("DB_NAME") + 
           " port=" + os.Getenv("DB_PORT") + 
           " sslmode=" + os.Getenv("DB_SSLMODE")

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    // Auto-migrate models
    db.AutoMigrate(&models.Sender{}, &models.Template{}, &models.User{}, &models.Analytics{})

    // Assign DB to the global variable
    DB = db
}
