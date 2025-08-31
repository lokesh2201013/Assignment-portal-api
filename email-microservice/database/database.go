package database

import (
    "log"
  //  "github.com/joho/godotenv"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "github.com/lokesh2201013/email-service/models"
    "os"
)

var DB *gorm.DB

func InitDB() {
    // Load environment variables from .env file
   

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

    db.AutoMigrate(&models.Sender{}, &models.Template{}, &models.User{}, &models.Analytics{})


    DB = db
}
