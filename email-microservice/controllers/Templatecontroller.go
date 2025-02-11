package controllers

import(
	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/models"

	"github.com/gofiber/fiber/v2"
)

// it allows user to strore a custom template
func CreateTemplate(c *fiber.Ctx) error{
	var template models.Template

	if err:=c.BodyParser(&template);err!=nil{
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request content"})
	}

	if err:=database.DB.Create(&template).Error;err!=nil{
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create template"})
	}
	return c.JSON(fiber.Map{"message": "Template created successfully","template": template})
}