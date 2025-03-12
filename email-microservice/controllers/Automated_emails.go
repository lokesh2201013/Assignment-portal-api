package controllers


import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	pb "github.com/lokesh2201013/email-microservice/proto"
)

type EmailServer struct {
	pb.UnimplementedEmailServiceServer
}

func (s *EmailServer) SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (*pb.EmailResponse, error) {
	log.Printf("Sending email to: %v", req.Recipients)

	// Simulate email sending (replace with actual SMTP logic)
	for _, email := range req.Recipients {
		fmt.Printf("Sending email to %s with subject: %s\n", email, req.Subject)
	}

	return &pb.EmailResponse{
		Message: "Emails sent successfully!",
		Success: true,
	}, nil
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterEmailServiceServer(grpcServer, &EmailServer{})

	log.Println("Email microservice running on port 50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
