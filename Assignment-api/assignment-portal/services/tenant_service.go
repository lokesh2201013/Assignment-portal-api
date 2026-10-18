package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
	"github.com/lokesh2201013/utils"
)

var tenantSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,62}$`)
var branchRegex = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,19}$`)

type tenantTokenIssuer interface {
	GenerateForTenant(userID string, role string, tenantID string) (string, error)
}

type TenantService struct {
	db         *sqlx.DB
	jwt        tenantTokenIssuer
	notifier   AssignmentNotifier
	inviteBase string
}

type TenantCreateRequest struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Owner    string `json:"owner_email"`
}

type TenantInvitationRequest struct {
	Email    string     `json:"email"`
	Role     string     `json:"role"`
	Name     string     `json:"name"`
	Branch   string     `json:"branch"`
	Semester int        `json:"semester"`
	CourseID *uuid.UUID `json:"course_id,omitempty"`
}

type InvitationAcceptRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Branch   string `json:"branch"`
	Semester int    `json:"semester"`
}

type TenantCreateResponse struct {
	TenantID string `json:"tenant_id"`
	Message  string `json:"message"`
}

type InvitationResponse struct {
	Message string `json:"message"`
}

type InvitationAcceptResponse struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	Token    string `json:"token"`
}

func NewTenantService(db *sqlx.DB, jwt tenantTokenIssuer, notifier AssignmentNotifier, inviteBase string) *TenantService {
	return &TenantService{db: db, jwt: jwt, notifier: notifier, inviteBase: strings.TrimRight(inviteBase, "/")}
}

func (s *TenantService) CreateTenant(ctx context.Context, req TenantCreateRequest) (TenantCreateResponse, error) {
	req.TenantID = strings.ToLower(strings.TrimSpace(req.TenantID))
	req.Name = strings.TrimSpace(req.Name)
	req.Owner = normalizeEmail(req.Owner)
	if !tenantSlugPattern.MatchString(req.TenantID) || req.Name == "" {
		return TenantCreateResponse{}, apperrors.New(apperrors.ErrValidation, "tenant_id must be a 3-63 character slug and name is required")
	}
	if _, err := mail.ParseAddress(req.Owner); err != nil {
		return TenantCreateResponse{}, apperrors.New(apperrors.ErrValidation, "Valid owner_email is required")
	}
	token, tokenHash, err := newInvitationToken()
	if err != nil {
		return TenantCreateResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not create owner invitation", err)
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return TenantCreateResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not create tenant", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO tenants (tenant_id, name, status) VALUES ($1, $2, 'pending')`, req.TenantID, req.Name); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(err.Error(), "23505") {
			return TenantCreateResponse{}, apperrors.New(apperrors.ErrConflict, "Tenant ID is already registered")
		}
		return TenantCreateResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not create tenant", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_invitations (invitation_id, tenant_id, email, role, token_hash, expires_at) VALUES ($1, $2, $3, 'admin', $4, $5)`, uuid.New(), req.TenantID, req.Owner, tokenHash, time.Now().Add(48*time.Hour)); err != nil {
		return TenantCreateResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not create owner invitation", err)
	}
	if err := tx.Commit(); err != nil {
		return TenantCreateResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not create tenant", err)
	}
	if err := s.sendInvitation(ctx, req.TenantID, req.Owner, "admin", token); err != nil {
		return TenantCreateResponse{}, err
	}
	return TenantCreateResponse{TenantID: req.TenantID, Message: "Tenant created; owner must accept the emailed invitation to activate it"}, nil
}

func (s *TenantService) Invite(ctx context.Context, tenantID string, actorID uuid.UUID, req TenantInvitationRequest) (InvitationResponse, error) {
	if err := s.assertActiveAdmin(ctx, tenantID, actorID); err != nil {
		return InvitationResponse{}, err
	}
	if err := s.createAndSendInvitation(ctx, tenantID, actorID, req); err != nil {
		return InvitationResponse{}, err
	}
	return InvitationResponse{Message: "Invitation sent; membership becomes active after email-token acceptance"}, nil
}

func (s *TenantService) AcceptInvitation(ctx context.Context, req InvitationAcceptRequest) (InvitationAcceptResponse, error) {
	req.Token = strings.TrimSpace(req.Token)
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Token) < 32 || req.Name == "" || len(req.Password) < 12 {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrValidation, "A valid invitation token, name, and password of at least 12 characters are required")
	}
	if req.Semester < 0 || req.Semester > 20 {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrValidation, "Invalid semester")
	}
	tokenHash := hashInvitationToken(req.Token)
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not accept invitation", err)
	}
	defer tx.Rollback()
	var invitation struct {
		TenantID    string         `db:"tenant_id"`
		Email       string         `db:"email"`
		Role        string         `db:"role"`
		Branch      sql.NullString `db:"branch"`
		Semester    sql.NullInt64  `db:"semester"`
		DisplayName sql.NullString `db:"display_name"`
		CourseID    uuid.NullUUID  `db:"course_id"`
	}
	if err := tx.GetContext(ctx, &invitation, `SELECT tenant_id, email, role, branch, semester, display_name, course_id FROM tenant_invitations WHERE token_hash = $1 AND accepted_at IS NULL AND expires_at > NOW() FOR UPDATE`, tokenHash); err != nil {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrUnauthorized, "Invitation is invalid or expired")
	}
	if invitation.DisplayName.Valid && strings.TrimSpace(invitation.DisplayName.String) != "" && req.Name == "" {
		req.Name = strings.TrimSpace(invitation.DisplayName.String)
	}
	if invitation.Role == "user" {
		if strings.TrimSpace(req.Branch) == "" && invitation.Branch.Valid {
			req.Branch = invitation.Branch.String
		}
		if req.Semester == 0 && invitation.Semester.Valid {
			req.Semester = int(invitation.Semester.Int64)
		}
	}
	if invitation.Role == "user" && (req.Semester < 1 || !branchRegex.MatchString(strings.ToUpper(strings.TrimSpace(req.Branch)))) {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrValidation, "Student invitations require a valid branch and semester")
	}
	var tenantStatus string
	if err := tx.GetContext(ctx, &tenantStatus, `SELECT status FROM tenants WHERE tenant_id = $1 FOR UPDATE`, invitation.TenantID); err != nil {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrNotFound, "Tenant not found")
	}
	if tenantStatus != "active" && invitation.Role != "admin" {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrForbidden, "Tenant is not active")
	}
	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not secure password", err)
	}
	userID := uuid.New()
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (user_id, tenant_id, name, email, password, role, branch, semester) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, userID, invitation.TenantID, req.Name, invitation.Email, passwordHash, invitation.Role, strings.ToUpper(strings.TrimSpace(req.Branch)), req.Semester); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(err.Error(), "23505") {
			return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrConflict, "An account already exists for this email in the tenant")
		}
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not create invited user", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id, user_id, role, status) VALUES ($1, $2, $3, 'verified')`, invitation.TenantID, userID, invitation.Role); err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not verify tenant membership", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tenant_invitations SET accepted_at = NOW() WHERE token_hash = $1`, tokenHash); err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not consume invitation", err)
	}
	if tenantStatus == "pending" && invitation.Role == "admin" {
		if _, err := tx.ExecContext(ctx, `UPDATE tenants SET status = 'active' WHERE tenant_id = $1`, invitation.TenantID); err != nil {
			return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not activate tenant", err)
		}
	}
	if invitation.CourseID.Valid {
		if _, err := tx.ExecContext(ctx, `INSERT INTO course_enrollments (tenant_id, course_id, user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, invitation.TenantID, invitation.CourseID.UUID, userID); err != nil {
			return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not enroll invited user in the course", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not accept invitation", err)
	}
	token, err := s.jwt.GenerateForTenant(userID.String(), invitation.Role, invitation.TenantID)
	if err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not issue access token", err)
	}
	return InvitationAcceptResponse{TenantID: invitation.TenantID, Role: invitation.Role, Token: token}, nil
}

func (s *TenantService) CreateCourse(ctx context.Context, tenantID string, actorID uuid.UUID, req dto.CourseCreateRequest) (models.Course, error) {
	if err := s.assertActiveAdmin(ctx, tenantID, actorID); err != nil {
		return models.Course{}, err
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	req.Branch = strings.ToUpper(strings.TrimSpace(req.Branch))
	if req.Code == "" || req.Name == "" || !branchRegex.MatchString(req.Branch) || req.Semester < 1 || req.Semester > 20 {
		return models.Course{}, apperrors.New(apperrors.ErrValidation, "Course code, name, valid branch, and semester are required")
	}
	course := models.Course{
		CourseID:  uuid.New(),
		TenantID:  tenantID,
		Code:      req.Code,
		Name:      req.Name,
		Branch:    req.Branch,
		Semester:  req.Semester,
		CreatedBy: actorID,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if _, err := s.db.NamedExecContext(ctx, `INSERT INTO courses (course_id, tenant_id, code, name, branch, semester, created_by, created_at) VALUES (:course_id, :tenant_id, :code, :name, :branch, :semester, :created_by, :created_at)`, course); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(err.Error(), "23505") {
			return models.Course{}, apperrors.New(apperrors.ErrConflict, "A course with this code already exists")
		}
		return models.Course{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not create course", err)
	}
	return course, nil
}

func (s *TenantService) ListCourses(ctx context.Context, tenantID string) ([]models.Course, error) {
	var courses []models.Course
	if err := s.db.SelectContext(ctx, &courses, `SELECT course_id, tenant_id, code, name, branch, semester, created_by, created_at FROM courses WHERE tenant_id = $1 ORDER BY code`, tenantID); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not list courses", err)
	}
	if courses == nil {
		courses = []models.Course{}
	}
	return courses, nil
}

func (s *TenantService) BulkInvite(ctx context.Context, tenantID string, actorID uuid.UUID, req dto.BulkInvitationRequest) (dto.RosterImportResult, error) {
	if err := s.assertActiveAdmin(ctx, tenantID, actorID); err != nil {
		return dto.RosterImportResult{}, err
	}
	if len(req.Invitations) == 0 {
		return dto.RosterImportResult{}, apperrors.New(apperrors.ErrValidation, "At least one invitation is required")
	}
	if len(req.Invitations) > 200 {
		return dto.RosterImportResult{}, apperrors.New(apperrors.ErrValidation, "A maximum of 200 invitations can be sent at once")
	}
	result := dto.RosterImportResult{}
	for i, entry := range req.Invitations {
		err := s.createAndSendInvitation(ctx, tenantID, actorID, TenantInvitationRequest{
			Email: entry.Email, Role: entry.Role, Name: entry.Name, Branch: entry.Branch, Semester: entry.Semester, CourseID: entry.CourseID,
		})
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %s", i+1, apperrors.PublicMessage(err)))
			continue
		}
		result.Invited++
	}
	return result, nil
}

func (s *TenantService) ImportRoster(ctx context.Context, tenantID string, actorID uuid.UUID, reader io.Reader) (dto.RosterImportResult, error) {
	entries, err := parseRosterCSV(reader)
	if err != nil {
		return dto.RosterImportResult{}, err
	}
	return s.BulkInvite(ctx, tenantID, actorID, dto.BulkInvitationRequest{Invitations: entries})
}

func (s *TenantService) ListMembers(ctx context.Context, tenantID string, actorID uuid.UUID, status string, page int, limit int) ([]models.TenantMember, dto.PageMetadata, error) {
	if err := s.assertActiveAdmin(ctx, tenantID, actorID); err != nil {
		return nil, dto.PageMetadata{}, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	where := `WHERE u.tenant_id = $1`
	args := []interface{}{tenantID}
	if status != "" {
		if status != "pending" && status != "verified" && status != "revoked" {
			return nil, dto.PageMetadata{}, apperrors.New(apperrors.ErrValidation, "status must be pending, verified, or revoked")
		}
		where += " AND m.status = $2"
		args = append(args, status)
	}
	var total int
	if err := s.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users u JOIN tenant_memberships m ON m.tenant_id = u.tenant_id AND m.user_id = u.user_id `+where, args...); err != nil {
		return nil, dto.PageMetadata{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not count members", err)
	}
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	args = append(args, limit, (page-1)*limit)
	var members []models.TenantMember
	if err := s.db.SelectContext(ctx, &members, `SELECT u.user_id, u.tenant_id, u.name, u.email, m.role, m.status, COALESCE(u.branch, '') AS branch, COALESCE(u.semester, 0) AS semester FROM users u JOIN tenant_memberships m ON m.tenant_id = u.tenant_id AND m.user_id = u.user_id `+where+` ORDER BY u.name LIMIT $`+strconv.Itoa(limitPos)+` OFFSET $`+strconv.Itoa(offsetPos), args...); err != nil {
		return nil, dto.PageMetadata{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not list members", err)
	}
	if members == nil {
		members = []models.TenantMember{}
	}
	return members, dto.PageMetadata{Page: page, Limit: limit, Total: total, TotalPages: pageCount(limit, total)}, nil
}

func (s *TenantService) SetMemberStatus(ctx context.Context, tenantID string, actorID uuid.UUID, userID uuid.UUID, status string) error {
	if err := s.assertActiveAdmin(ctx, tenantID, actorID); err != nil {
		return err
	}
	if status != "verified" && status != "revoked" {
		return apperrors.New(apperrors.ErrValidation, "status must be verified or revoked")
	}
	if userID == actorID {
		return apperrors.New(apperrors.ErrForbidden, "You cannot change your own membership status")
	}
	var member models.TenantMember
	if err := s.db.GetContext(ctx, &member, `SELECT u.user_id, u.tenant_id, u.name, u.email, m.role, m.status, COALESCE(u.branch, '') AS branch, COALESCE(u.semester, 0) AS semester FROM users u JOIN tenant_memberships m ON m.tenant_id = u.tenant_id AND m.user_id = u.user_id WHERE u.tenant_id = $1 AND u.user_id = $2`, tenantID, userID); err != nil {
		return apperrors.New(apperrors.ErrNotFound, "User is not a member of this tenant")
	}
	if member.Role == "admin" && member.Status == "verified" && status == "revoked" {
		var admins int
		if err := s.db.GetContext(ctx, &admins, `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id = $1 AND role = 'admin' AND status = 'verified'`, tenantID); err != nil {
			return apperrors.Wrap(apperrors.ErrDatabase, "Could not update membership", err)
		}
		if admins <= 1 {
			return apperrors.New(apperrors.ErrForbidden, "The last verified administrator cannot be deactivated")
		}
	}
	result, err := s.db.ExecContext(ctx, `UPDATE tenant_memberships SET status = $1 WHERE tenant_id = $2 AND user_id = $3`, status, tenantID, userID)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not update membership", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.New(apperrors.ErrNotFound, "User is not a member of this tenant")
	}
	return nil
}

func (s *TenantService) assertActiveAdmin(ctx context.Context, tenantID string, actorID uuid.UUID) error {
	var authorized bool
	if err := s.db.GetContext(ctx, &authorized, `SELECT EXISTS (SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2 AND role = 'admin' AND status = 'verified')`, tenantID, actorID); err != nil || !authorized {
		return apperrors.New(apperrors.ErrForbidden, "Verified tenant admin membership is required")
	}
	var tenantStatus string
	if err := s.db.GetContext(ctx, &tenantStatus, `SELECT status FROM tenants WHERE tenant_id = $1`, tenantID); err != nil {
		return apperrors.New(apperrors.ErrNotFound, "Tenant not found")
	}
	if tenantStatus != "active" {
		return apperrors.New(apperrors.ErrForbidden, "Tenant is not active")
	}
	return nil
}

func (s *TenantService) createAndSendInvitation(ctx context.Context, tenantID string, actorID uuid.UUID, req TenantInvitationRequest) error {
	req.Email = normalizeEmail(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role == "" {
		req.Role = "user"
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return apperrors.New(apperrors.ErrValidation, "Valid email is required")
	}
	if req.Role != "user" && req.Role != "admin" {
		return apperrors.New(apperrors.ErrValidation, "role must be user or admin")
	}
	branch := strings.ToUpper(strings.TrimSpace(req.Branch))
	if req.Role == "user" && branch != "" && !branchRegex.MatchString(branch) {
		return apperrors.New(apperrors.ErrValidation, "Invalid branch")
	}
	if req.Semester < 0 || req.Semester > 20 {
		return apperrors.New(apperrors.ErrValidation, "Invalid semester")
	}
	token, tokenHash, err := newInvitationToken()
	if err != nil {
		return apperrors.Wrap(apperrors.ErrUnexpected, "Could not create invitation", err)
	}
	var courseID interface{}
	if req.CourseID != nil {
		courseID = *req.CourseID
	}
	var semester interface{}
	if req.Semester > 0 {
		semester = req.Semester
	}
	var branchValue interface{}
	if branch != "" {
		branchValue = branch
	}
	var name interface{}
	if strings.TrimSpace(req.Name) != "" {
		name = strings.TrimSpace(req.Name)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO tenant_invitations (invitation_id, tenant_id, email, role, token_hash, invited_by, expires_at, branch, semester, display_name, course_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, uuid.New(), tenantID, req.Email, req.Role, tokenHash, actorID, time.Now().Add(48*time.Hour), branchValue, semester, name, courseID); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not save invitation", err)
	}
	return s.sendInvitation(ctx, tenantID, req.Email, req.Role, token)
}

func parseRosterCSV(reader io.Reader) ([]dto.InvitationEntry, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, apperrors.New(apperrors.ErrValidation, "Roster file must be valid CSV")
	}
	if len(records) < 2 {
		return nil, apperrors.New(apperrors.ErrValidation, "Roster CSV requires a header row and at least one student")
	}
	header := map[string]int{}
	for i, name := range records[0] {
		header[strings.ToLower(strings.TrimSpace(name))] = i
	}
	emailIdx, ok := header["email"]
	if !ok {
		return nil, apperrors.New(apperrors.ErrValidation, "Roster CSV must include an email column")
	}
	if len(records)-1 > 200 {
		return nil, apperrors.New(apperrors.ErrValidation, "A maximum of 200 roster rows can be imported at once")
	}
	entries := make([]dto.InvitationEntry, 0, len(records)-1)
	for _, row := range records[1:] {
		if len(row) == 0 || strings.TrimSpace(row[emailIdx]) == "" {
			continue
		}
		entry := dto.InvitationEntry{Email: csvField(row, emailIdx), Role: "user"}
		if idx, ok := header["role"]; ok {
			if role := strings.ToLower(csvField(row, idx)); role != "" {
				entry.Role = role
			}
		}
		if idx, ok := header["name"]; ok {
			entry.Name = csvField(row, idx)
		}
		if idx, ok := header["branch"]; ok {
			entry.Branch = csvField(row, idx)
		}
		if idx, ok := header["semester"]; ok {
			if value := csvField(row, idx); value != "" {
				semester, err := strconv.Atoi(value)
				if err != nil {
					return nil, apperrors.New(apperrors.ErrValidation, "semester must be an integer")
				}
				entry.Semester = semester
			}
		}
		if idx, ok := header["course_id"]; ok {
			if value := csvField(row, idx); value != "" {
				courseID, err := uuid.Parse(value)
				if err != nil {
					return nil, apperrors.New(apperrors.ErrValidation, "course_id must be a UUID")
				}
				entry.CourseID = &courseID
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, apperrors.New(apperrors.ErrValidation, "Roster CSV did not contain any email addresses")
	}
	return entries, nil
}

func csvField(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func pageCount(limit, total int) int {
	if total <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}

func (s *TenantService) sendInvitation(ctx context.Context, tenantID, email, role, token string) error {
	acceptURL := s.inviteBase + "/accept-invitation?tenant_id=" + tenantID + "&token=" + token
	_, err := s.notifier.SendAssignmentNotification(ctx, &pb.AssignmentEmailRequest{
		Subject:    "Assignment Portal tenant invitation",
		Body:       fmt.Sprintf("You were invited to %s as %s. Accept within 48 hours: %s", tenantID, role, acceptURL),
		Recipients: []string{email},
	})
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Could not send invitation email", err)
	}
	return nil
}

func newInvitationToken() (string, string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(value)
	return token, hashInvitationToken(token), nil
}

func hashInvitationToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
