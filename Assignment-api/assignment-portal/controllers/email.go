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
	if len(assignments) == 0 {
		return "", fmt.Errorf("no assignments provided")
	}

	var emails []string
	var task, branch, duedate string
	var semester, adminID int

	for _, a := range assignments {
		emails = append(emails, a.Email)
		task = a.Task
		branch = a.Branch
		semester = a.Semester
		duedate = a.DueDate
		adminID = a.AdminID
	}

	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("You have a task assigned by Admin %d", adminID),
		Body:       fmt.Sprintf("You have been assigned a new task.\nTask: %s\nBranch: %s\nSemester: %d\nDue Date: %s", task, branch, semester, duedate),
		Recipients: emails,
	}

	return sendAssignmentNotification(req)
}

func sendAssignmentNotification(req *pb.AssignmentEmailRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("email request is nil")
	}

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	fmt.Println("gRPC connection created")
	if err != nil {
		return "", fmt.Errorf("could not connect to EmailService: %v", err)
	}
	defer conn.Close()

	client := pb.NewEmailServiceClient(conn)

	res, err := client.SendAssignmentNotification(context.Background(), req)
	if err != nil {
		fmt.Printf("Error sending email: %v\n", err)
		return "Email not sent", err
	}

	fmt.Println("gRPC call successful")
	return res.Message, nil
}

