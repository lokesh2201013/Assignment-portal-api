package controllers

import (
	"context"
	"fmt"
	"log"

	pb "path/to/generated/email/proto"
	"github.com/lokesh2201013/assignment-portal/models"
	"google.golang.org/grpc"
)

func assignTask(assignments []models.Assignment) {
	conn, err := grpc.Dial("email-service:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Could not connect to EmailService: %v", err)
	}
	defer conn.Close()
   
	client := pb.NewEmailServiceClient(conn)
     var email []string
	for _, a := range assignments {
       email=append(email, a.Email)
	}
		req := &pb.AssignmentEmailRequest{
			Subject:    fmt.Sprintf("You have a task assingmed by Admin %d", a.AdminID),
			Body:       fmt.Sprintf("You have been assigned a new task.\nTask: %s\nBranch: %s\nSemester: %d", a.Task, a.Branch, a.Semester),
			Recipients: email,
		}
		
		res, err := client.SendAssignmentNotification(context.Background(), req)
		if err != nil {
			log.Printf("Could not send email %v", err)
		}

		log.Printf("Email sent %v", res.Message)
	}
