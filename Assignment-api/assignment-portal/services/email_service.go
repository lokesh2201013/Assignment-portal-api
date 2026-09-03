package services

import (
	"context"
	"fmt"

	"github.com/lokesh2201013/apperrors"
	pb "github.com/lokesh2201013/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AssignmentNotifier interface {
	SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (string, error)
}

type GRPCAssignmentNotifier struct {
	addr string
}

func NewGRPCAssignmentNotifier(addr string) *GRPCAssignmentNotifier {
	return &GRPCAssignmentNotifier{addr: addr}
}

func (n *GRPCAssignmentNotifier) SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (string, error) {
	if req == nil {
		return "", apperrors.New(apperrors.ErrValidation, "Email request is nil")
	}

	conn, err := grpc.NewClient(n.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Could not connect to email service", err)
	}
	defer conn.Close()

	res, err := pb.NewEmailServiceClient(conn).SendAssignmentNotification(ctx, req)
	if err != nil {
		return "Email not sent", apperrors.Wrap(apperrors.ErrExternal, "Could not send email notification", err)
	}
	if res == nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Email service returned no response", fmt.Errorf("nil response"))
	}
	return res.Message, nil
}
