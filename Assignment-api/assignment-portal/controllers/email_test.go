package controllers

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestEmailServiceContextRequiresAndSendsInternalToken(t *testing.T) {
	if _, err := emailServiceContext(context.Background(), " "); err == nil {
		t.Fatal("expected missing internal token to fail")
	}

	ctx, err := emailServiceContext(context.Background(), " shared-test-token ")
	if err != nil {
		t.Fatalf("create email service context: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md.Get("x-internal-token")) != 1 || md.Get("x-internal-token")[0] != "shared-test-token" {
		t.Fatalf("expected x-internal-token metadata, got %#v", md)
	}
}
