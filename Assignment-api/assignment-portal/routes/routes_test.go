package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestWorkflowRoutesRequireAuthentication(t *testing.T) {
	app := fiber.New()
	AuthRoutes(app, nil, nil)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/getassignments"},
		{http.MethodGet, "/admin/submissions?assignment_id=00000000-0000-0000-0000-000000000000"},
		{http.MethodPost, "/admin/assign_assignments"},
		{http.MethodPost, "/admin/submissions/grade"},
		{http.MethodPost, "/admin/invitations/bulk"},
		{http.MethodPost, "/admin/roster/import"},
		{http.MethodGet, "/admin/courses"},
		{http.MethodGet, "/admin/users"},
		{http.MethodPost, "/tenant/invitations"},
		{http.MethodGet, "/user/dashboard"},
		{http.MethodGet, "/user/submissions/00000000-0000-0000-0000-000000000000/revisions"},
		{http.MethodPost, "/user/aid/upload"},
		{http.MethodPut, "/aid/video/00000000-0000-0000-0000-000000000000/status"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected route to reject unauthenticated request with 401, got %d", resp.StatusCode)
			}
		})
	}
}
