package controllers

import (
	"strings"
	"time"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/metrics"
	"github.com/lokesh2201013/email-service/models"
	gomail "gopkg.in/gomail.v2"
	"go.uber.org/zap"
)

func getLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

type EmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	Format  string   `json:"format"`
}

// accumulate all total email in sender which have the same admin
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

// handle the error for email failed
func handleEmailError(err error, analytics *models.Analytics) error {
	errMsg := err.Error()
	if strings.Contains(errMsg, "550") || strings.Contains(errMsg, "551") || strings.Contains(errMsg, "554") || strings.Contains(errMsg, "553") {
		analytics.Bounced++
	} else if strings.Contains(errMsg, "421") || strings.Contains(errMsg, "452") || strings.Contains(errMsg, "521") || strings.Contains(errMsg, "450") {
		analytics.Rejected++
	}
	metricsWrapper := &metrics.AnalyticsWrapper{Analytics: *analytics}
	metricsWrapper.CalculateMetrics()
	database.DB.Save(&analytics)
	return fiber.NewError(500, "Failed to send email: " + errMsg)
}

func SendEmail(c *fiber.Ctx) error {
	logger := getLogger()
	defer logger.Sync()

	reqID := c.Locals("requestid")

	var req EmailRequest
	if err := c.BodyParser(&req); err != nil {
		logger.Warn("Invalid request content", zap.Any("requestid", reqID))
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request content"})
	}

	logger.Info("Initiating SendEmail request", zap.Any("requestid", reqID), zap.Strings("recipients", req.To))

	var sender models.Sender
	if err := database.DB.Where("email = ? AND verified = ?", req.From, true).First(&sender).Error; err != nil {
		logger.Warn("Sender not found or unverified", zap.Any("requestid", reqID), zap.String("email", req.From))
		return c.Status(400).JSON(fiber.Map{"error": "Sender not found"})
	}

	var admine models.User
	if err := database.DB.Where("username = ?", sender.AdminName).First(&admine).Error; err != nil {
		logger.Warn("Admin not found for sender", zap.Any("requestid", reqID))
		return c.Status(400).JSON(fiber.Map{"error": "Admin not found"})
	}

	d := gomail.NewDialer(sender.SMTPHost, sender.SMTPPort, sender.Username, sender.AppPassword)

	var analytics models.Analytics
	if err := database.DB.Where("admin_name = ? AND sender_id = ?", sender.AdminName, sender.ID).First(&analytics).Error; err != nil {
		logger.Warn("Analytics record not found", zap.Any("requestid", reqID))
		return c.Status(404).JSON(fiber.Map{"error": "Analytics record not found"})
	}

	for _, recipient := range req.To {
		// Rate limiting simulated
		logger.Debug("Sleeping for rate limit prevention", zap.Any("requestid", reqID), zap.String("recipient", recipient))
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
			logger.Error("Error sending isolated email", zap.Error(err), zap.String("recipient", recipient), zap.Any("requestid", reqID))
			handleEmailError(err, &analytics)
			continue
		}

		analytics.TotalEmails++
		analytics.Delivered++
		logger.Info("Email sent successfully", zap.Any("requestid", reqID), zap.String("recipient", recipient))
	}

	modifyAccumulatedEmail(admine.Username)

	metricsWrapper := &metrics.AnalyticsWrapper{Analytics: analytics}
	metricsWrapper.CalculateMetrics()

	database.DB.Save(&analytics)

	return c.JSON(fiber.Map{"message": "Emails processed successfully"})
}

func SendEmail_Grpc(subject string, body string, Email []string) error {
	logger := getLogger()
	defer logger.Sync()

	portStr := os.Getenv("SMTPPort")
	if portStr == "" {
		portStr = "587" // default port
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Error("Invalid SMTP port specified", zap.Error(err))
		return err
	}
	
	host := os.Getenv("SMTPHost")
	username := os.Getenv("Name")
	password := os.Getenv("AppPassword")
	senderEmail := os.Getenv("SenderEmail")
	
	d := gomail.NewDialer(host, port, username, password)
	
	var sender models.Sender
	if err := database.DB.Where("email = ? AND verified = ?", senderEmail, true).First(&sender).Error; err != nil {
		logger.Warn("Sender not found or unverified in gRPC", zap.String("email", senderEmail))
	}

	var analytics models.Analytics
	if sender.ID != 0 {
		database.DB.Where("admin_name = ? AND sender_id = ?", sender.AdminName, sender.ID).First(&analytics)
	}
	
	for _, to := range Email {
		m := gomail.NewMessage()
		m.SetHeader("From", senderEmail)
		m.SetHeader("To", to)
		m.SetHeader("Subject", subject)
		m.SetBody("text/plain", body)
		
		err := d.DialAndSend(m)
		if err != nil {
			logger.Error("gRPC Error sending email", zap.Error(err), zap.String("recipient", to))
			if sender.ID != 0 {
				handleEmailError(err, &analytics)
			}
			continue
		}
		
		if sender.ID != 0 {
			analytics.TotalEmails++
			analytics.Delivered++
		}
		logger.Info("gRPC Email sent successfully", zap.String("recipient", to))
	}
  
	if sender.ID != 0 {
		modifyAccumulatedEmail(sender.AdminName)
		metricsWrapper := &metrics.AnalyticsWrapper{Analytics: analytics}
		metricsWrapper.CalculateMetrics()
		database.DB.Save(&analytics)
	}

	return nil
}