package main

// DEPRECATED: Use internal/grpc/handler package instead
//
// The gRPC server is now implemented in:
//   - internal/grpc/handler/email_service.go
//
// It is registered and started in main.go:
//   startGRPCServer(cfg, handler.NewEmailServiceServer(services.Email, cfg.SMTPFrom, logger), logger)
