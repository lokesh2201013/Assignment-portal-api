package controllers

import (
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/lokesh2201013/models"
)

func TestResolveLoginUserTenantScope(t *testing.T) {
	tenantUser := models.User{TenantID: "tenant-one", Email: "person@example.com"}
	otherTenantUser := models.User{TenantID: "tenant-two", Email: "person@example.com"}

	tests := []struct {
		name     string
		users    []models.User
		tenantID string
		want     models.User
		wantErr  error
	}{
		{name: "one user without tenant selector", users: []models.User{tenantUser}, want: tenantUser},
		{name: "explicit tenant selects matching user", users: []models.User{tenantUser, otherTenantUser}, tenantID: "tenant-two", want: otherTenantUser},
		{name: "explicit default selects legacy empty tenant", users: []models.User{{Email: "person@example.com"}}, tenantID: "default", want: models.User{Email: "person@example.com"}},
		{name: "multiple tenant memberships require selector", users: []models.User{tenantUser, otherTenantUser}, wantErr: errTenantRequired},
		{name: "no user", wantErr: errLoginUserNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveLoginUser(test.users, test.tenantID)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("expected error %v, got %v", test.wantErr, err)
			}
			if err == nil && got != test.want {
				t.Fatalf("expected user %+v, got %+v", test.want, got)
			}
		})
	}
}

func TestGenerateLoginTokenUsesUserTenant(t *testing.T) {
	user := models.User{
		UserID:   uuid.New(),
		TenantID: "tenant-one",
		Role:     "user",
	}
	token, err := generateLoginToken(user)
	if err != nil {
		t.Fatalf("generate login token: %v", err)
	}

	parsed, _, err := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parse login token claims: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || claims["tenantID"] != user.TenantID {
		t.Fatalf("expected tenantID claim %q, got %#v", user.TenantID, claims["tenantID"])
	}
}

func TestLoginTenantIDDefaultsEmptyLegacyTenant(t *testing.T) {
	if got := loginTenantID(models.User{}); got != "default" {
		t.Fatalf("expected legacy user tenant default, got %q", got)
	}
}
