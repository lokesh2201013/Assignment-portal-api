package main

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInternalGRPCAuthInterceptor(t *testing.T) {
	interceptor := internalGRPCAuthInterceptor("this-is-a-test-token-with-more-than-32-characters")
	handlerCalled := false
	handler := func(context.Context, interface{}) (interface{}, error) {
		handlerCalled = true
		return "ok", nil
	}

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token status = %v, want unauthenticated", status.Code(err))
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-internal-token", "wrong-token"))
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong token status = %v, want unauthenticated", status.Code(err))
	}

	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-internal-token", "this-is-a-test-token-with-more-than-32-characters"))
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if resp != "ok" || !handlerCalled {
		t.Fatalf("authorized handler response = %v, handlerCalled = %t", resp, handlerCalled)
	}
}
