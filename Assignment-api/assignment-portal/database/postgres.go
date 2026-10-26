package database

import (
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/lokesh2201013/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func SQLXDB() (*sqlx.DB, error) {
	if DB == nil {
		return nil, fmt.Errorf("PostgreSQL is not initialized")
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("get PostgreSQL connection: %w", err)
	}
	return sqlx.NewDb(sqlDB, "postgres"), nil
}

func ConnectDB() {
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

	if err := db.AutoMigrate(&models.User{}, &models.Assignment{}, &models.SubmitAssignment{}, &models.Video{}); err != nil {
		log.Fatalf("AutoMigrate failed: %v\n", err)
	}

	DB = db
	log.Println("connected to PostgreSQL")
}

func createTables() error {
	return nil
}
