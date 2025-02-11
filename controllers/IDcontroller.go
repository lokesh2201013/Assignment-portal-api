package controllers

import (
	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/models"
	"github.com/gofiber/fiber/v2"
	"gopkg.in/gomail.v2"
	"fmt"
)

func verifySMTP(sender models.Sender) error {
	d := gomail.NewDialer(sender.SMTPHost, sender.SMTPPort, sender.Username, sender.AppPassword)
	_, err := d.Dial()
	return err
}

func VerifyEmailID(c *fiber.Ctx) error {
	var sender models.Sender
	fmt.Println("AppPassword before insert:", sender.AppPassword)

	if err := c.BodyParser(&sender); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request content"})
	}

	// Ensure password is provided
	if sender.AppPassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Password must be provided"})
	}

	// Check if sender already exists
	var existingSender models.Sender
	if err := database.DB.Where("email = ?", sender.Email).First(&existingSender).Error; err != nil {
		// If sender doesn't exist, create new sender
		if err := verifySMTP(sender); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "SMTP verification failure"})
		}
		 // Ensure this is set correctly
		sender.Verified = true
		if err := database.DB.Create(&sender).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to add sender"})
		}

		// Ensure sender.ID is populated after creation
		if sender.ID == 0 {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to get sender ID"})
		}

		// Create Analytics only after sender creation
		analytics := models.Analytics{
			AdminName:   sender.AdminName,
			SenderID:    sender.ID, // Use the correct ID here
			TotalEmails: 0,
			Delivered:   0,
			Bounced:     0,
			Complaints:  0,
			Rejected:    0,
		}
		if err := database.DB.Create(&analytics).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create analytics for sender"})
		}
		return c.JSON(fiber.Map{"message": "Email verified and added to sender list", "email": sender.Email})
	} else {
		// If sender exists, verify and update
		if existingSender.Verified {
			return c.Status(400).JSON(fiber.Map{"error": "Sender already verified"})
		}
		if err := verifySMTP(existingSender); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "SMTP verification failure for existing sender"})
		}
		existingSender.Verified = true
		if err := database.DB.Save(&existingSender).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update sender"})
		}

		// Create Analytics for updated sender
		analytics := models.Analytics{
			AdminName:   existingSender.AdminName,
			SenderID:    existingSender.ID,
			TotalEmails: 0,
			Delivered:   0,
			Bounced:     0,
			Complaints:  0,
			Rejected:    0,
		}
		if err := database.DB.Create(&analytics).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create analytics for sender"})
		}

		return c.JSON(fiber.Map{"message": "Email verified and updated", "email": existingSender.Email})
	}
}

func ListVerifiedIdentities(c *fiber.Ctx) error {
	var senders []models.Sender
	adminUsername := c.Locals("admin").(string)
	if err := database.DB.Where("verified = ? AND admin_name = ?", true, adminUsername).Find(&senders).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve senders"})
	}
	var emails []string
	for _, sender := range senders {
		emails = append(emails, sender.Email)
	}
	return c.JSON(fiber.Map{"idmail": emails})
}

func ListunVerifiedIdentities(c *fiber.Ctx) error {
	var senders []models.Sender
	adminUsername := c.Locals("admin").(string)
	if err := database.DB.Where("verified = ? AND admin_name = ?", false, adminUsername).Find(&senders).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve senders"})
	}
	var emails []string
	for _, sender := range senders {
		emails = append(emails, sender.Email)
	}
	return c.JSON(fiber.Map{"idmail": emails})
}

func DeleteIdentity(c *fiber.Ctx) error {
    email := c.Params("email")
    fmt.Println("Deleting sender with email:", email)

    // Delete associated analytics records first
    if err := database.DB.Where("sender_id = (SELECT id FROM senders WHERE email = ?)", email).Delete(&models.Analytics{}).Error; err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Failed to delete associated analytics records"})
    }

    // Now delete the sender record
    if err := database.DB.Where("email = ?", email).Delete(&models.Sender{}).Error; err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Failed to delete sender"})
    }

    return c.JSON(fiber.Map{"message": "Sender deleted successfully"})
}
