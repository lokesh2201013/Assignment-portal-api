package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/apperrors"
	pb "github.com/lokesh2201013/proto"
	"github.com/lokesh2201013/utils"
)

var tenantSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,62}$`)

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
	Email string `json:"email"`
	Role  string `json:"role"`
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
	req.Email = normalizeEmail(req.Email)
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return InvitationResponse{}, apperrors.New(apperrors.ErrValidation, "Valid email is required")
	}
	if req.Role != "user" && req.Role != "admin" {
		return InvitationResponse{}, apperrors.New(apperrors.ErrValidation, "role must be user or admin")
	}
	var authorized bool
	if err := s.db.GetContext(ctx, &authorized, `SELECT EXISTS (SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2 AND role = 'admin' AND status = 'verified')`, tenantID, actorID); err != nil || !authorized {
		return InvitationResponse{}, apperrors.New(apperrors.ErrForbidden, "Verified tenant admin membership is required")
	}
	var tenantStatus string
	if err := s.db.GetContext(ctx, &tenantStatus, `SELECT status FROM tenants WHERE tenant_id = $1`, tenantID); err != nil {
		return InvitationResponse{}, apperrors.New(apperrors.ErrNotFound, "Tenant not found")
	}
	if tenantStatus != "active" {
		return InvitationResponse{}, apperrors.New(apperrors.ErrForbidden, "Tenant is not active")
	}
	token, tokenHash, err := newInvitationToken()
	if err != nil {
		return InvitationResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not create invitation", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO tenant_invitations (invitation_id, tenant_id, email, role, token_hash, invited_by, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`, uuid.New(), tenantID, req.Email, req.Role, tokenHash, actorID, time.Now().Add(48*time.Hour))
	if err != nil {
		return InvitationResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not save invitation", err)
	}
	if err := s.sendInvitation(ctx, tenantID, req.Email, req.Role, token); err != nil {
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
		TenantID string `db:"tenant_id"`
		Email    string `db:"email"`
		Role     string `db:"role"`
	}
	if err := tx.GetContext(ctx, &invitation, `SELECT tenant_id, email, role FROM tenant_invitations WHERE token_hash = $1 AND accepted_at IS NULL AND expires_at > NOW() FOR UPDATE`, tokenHash); err != nil {
		return InvitationAcceptResponse{}, apperrors.New(apperrors.ErrUnauthorized, "Invitation is invalid or expired")
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
	if err := tx.Commit(); err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not accept invitation", err)
	}
	token, err := s.jwt.GenerateForTenant(userID.String(), invitation.Role, invitation.TenantID)
	if err != nil {
		return InvitationAcceptResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not issue access token", err)
	}
	return InvitationAcceptResponse{TenantID: invitation.TenantID, Role: invitation.Role, Token: token}, nil
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
