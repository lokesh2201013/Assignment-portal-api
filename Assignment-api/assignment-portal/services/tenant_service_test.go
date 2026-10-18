package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
)

func TestParseRosterCSVValid(t *testing.T) {
	csvData := `name,email,role,branch,semester,course_id
Alice,alice@example.com,user,CSE,4,a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d
Bob,bob@example.com,user,ECE,6,`

	entries, err := parseRosterCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected error parsing roster: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Email != "alice@example.com" || entries[0].Name != "Alice" || entries[0].Branch != "CSE" || entries[0].Semester != 4 {
		t.Fatalf("unexpected entry 0: %+v", entries[0])
	}
	expectedCourseID := uuid.MustParse("a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d")
	if entries[0].CourseID == nil || *entries[0].CourseID != expectedCourseID {
		t.Fatalf("expected course ID %v, got %v", expectedCourseID, entries[0].CourseID)
	}

	if entries[1].Email != "bob@example.com" || entries[1].CourseID != nil {
		t.Fatalf("unexpected entry 1: %+v", entries[1])
	}
}

func TestParseRosterCSVMissingEmailHeader(t *testing.T) {
	csvData := `name,role,branch,semester
Alice,user,CSE,4`
	_, err := parseRosterCSV(strings.NewReader(csvData))
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error for missing email header, got %v", err)
	}
}

func TestParseRosterCSVNoDataRows(t *testing.T) {
	csvData := `name,email,role`
	_, err := parseRosterCSV(strings.NewReader(csvData))
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error for empty rows, got %v", err)
	}
}

func TestParseRosterCSVInvalidSemester(t *testing.T) {
	csvData := `email,semester
student@example.com,invalid_num`
	_, err := parseRosterCSV(strings.NewReader(csvData))
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error for invalid semester, got %v", err)
	}
}

func TestParseRosterCSVInvalidCourseUUID(t *testing.T) {
	csvData := `email,course_id
student@example.com,not-a-uuid`
	_, err := parseRosterCSV(strings.NewReader(csvData))
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error for invalid course_id UUID, got %v", err)
	}
}

func TestParseRosterCSVExceedsRowLimit(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("email\n")
	for i := 0; i < 205; i++ {
		builder.WriteString("student@example.com\n")
	}
	_, err := parseRosterCSV(strings.NewReader(builder.String()))
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error for exceeding 200 row limit, got %v", err)
	}
}
