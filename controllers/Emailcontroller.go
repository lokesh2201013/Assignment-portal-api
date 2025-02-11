package controllers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/metrics"
	"github.com/lokesh2201013/email-service/models"
	"gopkg.in/gomail.v2"
)

const (
	maxEmailsPerDay    = 200
	maxEmailsPerSecond = 1
)
type EmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	Format  string   `json:"format"`
}

//accumulate all total email in sender which have the same admin
func modifyAccumulatedEmail(adminName string) error {
	var analytics []models.Analytics

	
	if err := database.DB.Where("admin_name = ?", adminName).Find(&analytics).Error; err != nil {
		return err
	}

	totalAccumulatedEmails := 0
	for _, record := range analytics {
		totalAccumulatedEmails += record.AccumulatedEmail
	}

	
	for i := range analytics {
		analytics[i].AccumulatedEmail = totalAccumulatedEmails
	}
	
	if err := database.DB.Save(&analytics).Error; err != nil {
		return err
	}
	return nil
}


/*func createEmailMessage(sender models.Sender, req *EmailRequest) (*gomail.Message, error) {
	mail := gomail.NewMessage()
	mail.SetHeader("From", sender.Email)
	mail.SetHeader("To", req.To...)
	mail.SetHeader("Subject", req.Subject)

	switch req.Format {
	case "html":
		mail.SetBody("text/html", req.Body)
	case "text":
		mail.SetBody("text/plain", req.Body)
	default:
		return nil, fiber.NewError(400, "Invalid format")
	}
	return mail, nil
}*/

// handle the error for email fialed
func handleEmailError(err error, analytics *models.Analytics) error {
	errMsg := err.Error()
	if strings.Contains(errMsg, "550") || strings.Contains(errMsg, "551") || strings.Contains(errMsg, "554") || strings.Contains(errMsg, "553") {
		analytics.Bounced++
	} else if strings.Contains(errMsg, "421") || strings.Contains(errMsg, "452") || strings.Contains(errMsg, "521") || strings.Contains(errMsg, "450") {
		analytics.Rejected++
	}
	metricsWrapper := &metrics.AnalyticsWrapper{*analytics}
	metricsWrapper.CalculateMetrics()
	database.DB.Save(&analytics)
	return fiber.NewError(500, "Failed to send email: " + errMsg)
}

func SendEmail(c *fiber.Ctx) error {
	var req EmailRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request content"})
	}

	// Query the Sender table to get the admin_name of the email in req.From
	var sender models.Sender
	if err := database.DB.Where("email = ? AND verified = ?", req.From, true).First(&sender).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Sender not found"})
	}

	// Now, retrieve the admin details from the User table using the admin_name from sender
	var admine models.User
	if err := database.DB.Where("username = ?", sender.AdminName).First(&admine).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Admin not found"})
	}

	// Setup SMTP dialer
	d := gomail.NewDialer(sender.SMTPHost, sender.SMTPPort, sender.Username, sender.AppPassword)

	// Get the analytics record for the sender's admin
	var analytics models.Analytics
	if err := database.DB.Where("admin_name = ? AND sender_id = ?", sender.AdminName, sender.ID).First(&analytics).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Analytics record not found"})
	}

	// Ensure we respect the rate limits
	daysSinceCreation := int(time.Since(admine.CreatedAt).Hours() / 24)
	if daysSinceCreation <= 7 {
		if analytics.AccumulatedEmail >= maxEmailsPerDay*daysSinceCreation+1 {
			return c.Status(400).JSON(fiber.Map{"error": "Mail limit of day exceeded"})
		}
	}

	// Send emails one by one
	for _, recipient := range req.To {
		time.Sleep(time.Second)
		mail := gomail.NewMessage()
		mail.SetHeader("From", sender.Email)
		mail.SetHeader("To", recipient)
		mail.SetHeader("Subject", req.Subject)

		switch req.Format {
		case "html":
			mail.SetBody("text/html", req.Body)
		case "text":
			mail.SetBody("text/plain", req.Body)
		default:
			return c.Status(400).JSON(fiber.Map{"error": "Invalid format"})
		}

		err := d.DialAndSend(mail)
		if err != nil {
			handleEmailError(err, &analytics)
			continue
		}

		// Update analytics
		analytics.TotalEmails++
		analytics.Delivered++
	}

	// Update accumulated email count
	modifyAccumulatedEmail(admine.Username)

	// Calculate and update metrics
	metricsWrapper := &metrics.AnalyticsWrapper{analytics}
	metricsWrapper.CalculateMetrics()

	// Save updated analytics
	database.DB.Save(&analytics)

	return c.JSON(fiber.Map{"message": "Emails processed successfully"})
}
