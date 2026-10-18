package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
)

func TestEvaluateLatePolicyRejectsAfterDueDate(t *testing.T) {
	due := time.Now().Add(-time.Hour).Format(time.RFC3339)
	_, err := evaluateLatePolicy(models.LatePolicyReject, due, time.Now())
	if !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("expected forbidden late rejection, got %v", err)
	}
}

func TestEvaluateLatePolicyAllowsPenaltyAfterDueDate(t *testing.T) {
	due := time.Now().Add(-time.Hour).Format(time.RFC3339)
	late, err := evaluateLatePolicy(models.LatePolicyPenalty, due, time.Now())
	if err != nil || !late {
		t.Fatalf("expected late=true without error, got late=%v err=%v", late, err)
	}
}

func TestCanResubmitRules(t *testing.T) {
	if err := canResubmit(false, 1, 1, false, "pending"); err != nil {
		t.Fatalf("first attempt should be allowed: %v", err)
	}
	if err := canResubmit(false, 3, 1, true, "pending"); !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("resubmit without policy should be forbidden, got %v", err)
	}
	if err := canResubmit(true, 2, 2, true, "rejected"); !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("max attempts should be forbidden, got %v", err)
	}
	if err := canResubmit(true, 3, 1, true, "accepted"); !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("accepted work should not resubmit, got %v", err)
	}
}

func TestApplyLatePenalty(t *testing.T) {
	if got := applyLatePenalty(80, 25); got != 60 {
		t.Fatalf("expected 60, got %v", got)
	}
}

func TestNormalizeAssignmentPolicyDefaults(t *testing.T) {
	policy, maxScore, attempts, reminder, penalty, err := normalizeAssignmentPolicy(dto.AssignmentCreateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if policy != models.LatePolicyAllow || maxScore != 100 || attempts != 1 || reminder != 48 || penalty != 0 {
		t.Fatalf("unexpected defaults: %s %v %d %d %v", policy, maxScore, attempts, reminder, penalty)
	}
}

func TestResolveGradeRequiresScoreForAcceptance(t *testing.T) {
	assignment := models.Assignment{MaxScore: 100}
	_, _, err := resolveGrade(assignment, dto.GradeSubmissionRequest{Status: "accepted"})
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBuildLearnerDashboardBuckets(t *testing.T) {
	score := 88.0
	items := []models.DashboardItem{
		{Task: "upcoming unpaid", HoursUntilDue: 10, ReminderHours: 48, Status: "pending"},
		{Task: "reminder soon", HoursUntilDue: 12, ReminderHours: 24, Status: "pending"},
		{Task: "overdue", HoursUntilDue: -3, Status: "pending"},
		{Task: "graded", Status: "accepted", Score: &score, Submitted: true},
		{Task: "pending review", HoursUntilDue: 5, Status: "pending", Submitted: true, ReminderHours: 48},
	}
	dashboard := buildLearnerDashboard(items)
	if dashboard.Summary.Upcoming != 2 || dashboard.Summary.Overdue != 1 || dashboard.Summary.Graded != 1 || dashboard.Summary.PendingReview != 1 {
		t.Fatalf("unexpected summary: %+v", dashboard.Summary)
	}
	if len(dashboard.Reminders) != 2 {
		t.Fatalf("expected two reminders, got %d", len(dashboard.Reminders))
	}
}

func TestNormalizeRubricsRejectsOverflow(t *testing.T) {
	_, err := normalizeRubrics([]dto.RubricRequest{{Criterion: "Accuracy", MaxPoints: 80}, {Criterion: "Style", MaxPoints: 40}}, 100)
	if !errors.Is(err, apperrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestResolveGradeSumsRubricPoints(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	assignment := models.Assignment{
		MaxScore: 10,
		Rubrics: []models.Rubric{
			{RubricID: first, Criterion: "Accuracy", MaxPoints: 6},
			{RubricID: second, Criterion: "Clarity", MaxPoints: 4},
		},
	}
	score, scores, err := resolveGrade(assignment, dto.GradeSubmissionRequest{
		Status: "accepted",
		RubricScores: []dto.RubricScoreInput{
			{RubricID: first, Points: 5},
			{RubricID: second, Points: 3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if score == nil || *score != 8 || len(scores) != 2 {
		t.Fatalf("unexpected grade: score=%v scores=%d", score, len(scores))
	}
}
