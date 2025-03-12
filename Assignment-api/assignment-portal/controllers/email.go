package controllers

import (
	"context"
	"log"
	"time"

	pb "github.com/lokesh2201013/assignment-portal/proto" // Import generated proto
	"google.golang.org/grpc"
)

func SendEmailNotification(adminName, subject, body string, recipients []string) {
	conn, err := grpc.Dial("email-microservice:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Could not connect to Email Service: %v", err)
	}
	defer conn.Close()

	client := pb.NewEmailServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	req := &pb.AssignmentEmailRequest{
		AdminName:  adminName,
		Subject:    subject,
		Body:       body,
		Recipients: recipients,
	}

	resp, err := client.SendAssignmentNotification(ctx, req)
	if err != nil {
		log.Fatalf("Error calling Email Service: %v", err)
	}

	log.Printf("Email Service Response: %s", resp.Message)
}
