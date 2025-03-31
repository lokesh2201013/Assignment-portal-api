package main

import (
	"context"
	"log"
     "fmt"
	"github.com/lokesh2201013/email-service/controllers"
	pb "github.com/lokesh2201013/email-service/proto"
)


func (s *emailServiceServer) SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (*pb.EmailResponse, error) {
    log.Printf("Sending email to: %v", req.Recipients)
    fmt.Println("Working fine 4")
    err := controllers.SendEmail_Grpc(req.Subject, req.Body, req.Recipients)
    if err != nil {
        return &pb.EmailResponse{
            Message: "Failed to send emails",
            Success: false,
        }, err
    }
    fmt.Println("Working fine 8")
    return &pb.EmailResponse{
        Message: "Emails sent successfully",
        Success: true,
    }, nil
}
