package controllers

import (
	"context"
	"fmt"
	"log"

	"github.com/lokesh2201013/assignment-portal/models"
	pb "github.com/lokesh2201013/assignment-portal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func assignTask(assignments []models.Assignment) (string, error) {
	conn, err := grpc.NewClient("email-service:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect to EmailService: %v", err)
	}
	defer conn.Close()

	client := pb.NewEmailServiceClient(conn)

	var emails []string
	var adminID int
	var task, branch string
	var semester int

	for _, a := range assignments {
		emails = append(emails, a.Email)
		task = a.Task
		branch = a.Branch
		semester = a.Semester
		adminID = a.AdminID
	}

	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("You have a task assigned by Admin %d", adminID),
		Body:       fmt.Sprintf("You have been assigned a new task.\nTask: %s\nBranch: %s\nSemester: %d", task, branch, semester),
		Recipients: emails,
	}

	res, err := client.SendAssignmentNotification(context.Background(), req)
	if err != nil {
		return "Email not sent", err
	}
	return res.Message, nil
}
