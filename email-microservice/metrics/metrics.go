package metrics

import (
	"github.com/lokesh2201013/email-service/models"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/email-service/database"
)

type AnalyticsWrapper struct {
	models.Analytics
}

func (a *AnalyticsWrapper) CalculateMetrics() {
	a.DeliveryRate = float64(a.Delivered) / float64(a.TotalEmails) * 100
	a.BounceRate = float64(a.Bounced) / float64(a.TotalEmails) * 100
	a.ComplaintRate = float64(a.Complaints) / float64(a.TotalEmails) * 100
	a.RejectRate = float64(a.Rejected) / float64(a.TotalEmails) * 100
}

func GetEmailMetrics(c *fiber.Ctx) error {
	senderEmail := c.Params("senderEmail")

	var sender models.Sender
	if err := database.DB.Where("email = ?", senderEmail).First(&sender).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Sender not found"})
	}

	var analytics models.Analytics
	if err := database.DB.Where("sender_id = ?", sender.ID).First(&analytics).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Analytics not found for sender"})
	}

	metricsWrapper := &AnalyticsWrapper{
		Analytics: analytics,
	}

	metricsWrapper.CalculateMetrics()

	return c.JSON(fiber.Map{
		"delivery_rate":   metricsWrapper.DeliveryRate,
		"bounce_rate":     metricsWrapper.BounceRate,
		"complaint_rate":  metricsWrapper.ComplaintRate,
		"reject_rate":     metricsWrapper.RejectRate,
		"total_emails":    analytics.TotalEmails,
		"delivered":       analytics.Delivered,
		"bounced":         analytics.Bounced,
		"complaints":      analytics.Complaints,
		"rejected":        analytics.Rejected,
	})
}

func GetAdminEmailMetrics(c *fiber.Ctx) error {
	adminName := c.Params("adminName")

	var senders []models.Sender
	if err := database.DB.Where("admin_name = ?", adminName).Find(&senders).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "No senders found for the specified admin"})
	}

	var allSenderMetrics []fiber.Map

	for _, sender := range senders {
		var analytics models.Analytics
		if err := database.DB.Where("sender_id = ?", sender.ID).First(&analytics).Error; err != nil {
			continue
		}

		metricsWrapper := AnalyticsWrapper{
			Analytics: analytics,
		}
		metricsWrapper.CalculateMetrics()

		allSenderMetrics = append(allSenderMetrics, fiber.Map{
			"sender_email":   sender.Email,
			"delivery_rate":  metricsWrapper.DeliveryRate,
			"bounce_rate":    metricsWrapper.BounceRate,
			"complaint_rate": metricsWrapper.ComplaintRate,
			"reject_rate":    metricsWrapper.RejectRate,
			"total_emails":   analytics.TotalEmails,
			"delivered":      analytics.Delivered,
			"bounced":        analytics.Bounced,
			"complaints":     analytics.Complaints,
			"rejected":       analytics.Rejected,
		})
	}

	return c.JSON(fiber.Map{
		"admin":           adminName,
		"senders_metrics": allSenderMetrics,
	})
}