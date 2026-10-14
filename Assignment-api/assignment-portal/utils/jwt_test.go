package utils

import (
	"testing"
	"time"
)

func TestJWTManagerGenerateAndParse(t *testing.T) {
	manager := NewJWTManager("secret", "assignment-portal", time.Hour)

	token, err := manager.Generate("user-1", "admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.TenantID != "default" {
		t.Fatalf("expected legacy token to use default tenant, got %q", claims.TenantID)
	}
}

func TestJWTManagerPreservesTenant(t *testing.T) {
	manager := NewJWTManager("secret", "assignment-portal", time.Hour)
	token, err := manager.GenerateForTenant("user-1", "user", "school-42")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.TenantID != "school-42" {
		t.Fatalf("expected tenant claim school-42, got %q", claims.TenantID)
	}
}

func TestJWTManagerRejectsWrongSecret(t *testing.T) {
	token, err := NewJWTManager("secret", "assignment-portal", time.Hour).Generate("user-1", "admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = NewJWTManager("other", "assignment-portal", time.Hour).Parse(token)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestJWTManagerRejectsWrongIssuer(t *testing.T) {
	token, err := NewJWTManager("secret", "assignment-portal", time.Hour).Generate("user-1", "admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = NewJWTManager("secret", "other", time.Hour).Parse(token)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
