package database

import (
	"log"
	"github.com/lokesh2201013/assignment-portal/models"
"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB(){
	 dsn := "host=localhost user=postgres password= 9910994194lokesh dbname= assignment port= 5432 sslmode=disable"
     db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err!=nil{
		log.Fatalf("Failed to connect to db ,err = %v\n",err)

	}
	db.AutoMigrate(&models.User{})
	db.AutoMigrate(&models.Assignment{})
	DB=db
	log.Println("Connected to PostgreSQL using GORM")
}