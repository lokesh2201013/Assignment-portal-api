package services

import "testing"

func TestResolveAnalyticsReport(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{name: "report identifier", input: "submission_status", wantName: "submission_status"},
		{name: "documented alias", input: "overdue assignments", wantName: "overdue_assignments"},
		{name: "unknown SQL rejected", input: "SELECT * FROM users", wantName: ""},
		{name: "unknown report rejected", input: "student passwords", wantName: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, ok := resolveAnalyticsReport(test.input)
			if test.wantName == "" {
				if ok {
					t.Fatalf("resolveAnalyticsReport(%q) unexpectedly accepted", test.input)
				}
				return
			}
			if !ok || report.name != test.wantName {
				t.Fatalf("resolveAnalyticsReport(%q) = %q, %t; want %q, true", test.input, report.name, ok, test.wantName)
			}
		})
	}
}
