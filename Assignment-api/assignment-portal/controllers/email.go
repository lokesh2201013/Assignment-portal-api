package controllers

import (
	"context"
	"fmt"
	//"log"

	"github.com/lokesh2201013/assignment-portal/models"
	pb "github.com/lokesh2201013/assignment-portal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func assignTask(assignments []models.Assignment) (string, error) {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
     fmt.Println("Working fine 2")
	if err != nil {
		return "", fmt.Errorf("could not connect to EmailService: %v", err)
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
    
	//res, err := client.SendAssignmentNotification(context.Background(), req)
	res, err := client.SendAssignmentNotification(context.Background(), req)
	fmt.Println("Working fine 3")
if err != nil {
	fmt.Printf("Error sending email: %v\n", err) // Print the actual error
	return "Email not sent", err
}
fmt.Println("Working fine 9")

	return res.Message, nil
}
