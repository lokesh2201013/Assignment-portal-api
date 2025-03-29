package controllers

import (
	"context"
	//"fmt"
	"log"
	"net"

	pb "github.com/lokesh2201013/email-service/proto"
	"google.golang.org/grpc"
)

type emailServiceServer struct {
	pb.UnimplementedEmailServiceServer
}

func (s *emailServiceServer) SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (*pb.EmailResponse, error) {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterEmailServiceServer(grpcServer, &emailServiceServer{})

	log.Println("Email Service is running on port 50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	log.Printf("Sending email to: %v", req.Recipients)

	// Simulate sending email (replace with actual email-sending logic)
	SendEmail_Grpc(req.Subject, req.Body, req.Recipients)
    
	return &pb.EmailResponse{
		Message: "Emails sent successfully",
		Success: true,
	}, nil
}
