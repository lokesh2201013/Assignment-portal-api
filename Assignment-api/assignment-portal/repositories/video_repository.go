package repositories

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/models"
)

type VideoRepository struct {
	db *sqlx.DB
}

func NewVideoRepository(db *sqlx.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

func (r *VideoRepository) Create(video *models.Video) error {
	if video.ID == uuid.Nil {
		video.ID = uuid.New()
	}
	tags, err := json.Marshal(video.Tags)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not save video", err)
	}
	if video.TenantID == "" {
		video.TenantID = "default"
	}
	query := `INSERT INTO videos (id, tenant_id, url, title, tags, status, description)
			  VALUES (:id, :tenant_id, :url, :title, :tags, :status, :description)`

	params := map[string]interface{}{
		"id":          video.ID,
		"tenant_id":   video.TenantID,
		"url":         video.URL,
		"title":       video.Title,
		"tags":        tags,
		"status":      video.Status,
		"description": video.Description}

	if _, err := r.db.NamedExec(query, params); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not save video", err)
	}
	return nil
}

func (r *VideoRepository) UpdateStatus(id string, status string) error {
	return r.UpdateStatusInTenant("default", id, status)
}

func (r *VideoRepository) UpdateStatusInTenant(tenantID string, id string, status string) error {
	query := `UPDATE videos 
				SET status = :status
			  WHERE tenant_id = :tenant_id AND id = :id`
	params := map[string]interface{}{
		"status":    status,
		"tenant_id": tenantID,
		"id":        id,
	}
	if _, err := r.db.NamedExec(query, params); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not update video status", err)
	}
	return nil
}
