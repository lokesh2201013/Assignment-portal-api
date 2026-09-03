package repositories

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/models"
)

type VideoRepository struct {
	db *sql.DB
}

func NewVideoRepository(db *sql.DB) *VideoRepository {
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
	if _, err := r.db.Exec(`INSERT INTO videos (id, url, title, tags, status, description) VALUES ($1, $2, $3, $4, $5, $6)`, video.ID, video.URL, video.Title, tags, video.Status, video.Description); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not save video", err)
	}
	return nil
}

func (r *VideoRepository) UpdateStatus(id string, status string) error {
	if _, err := r.db.Exec(`UPDATE videos SET status = $1 WHERE id = $2`, status, id); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not update video status", err)
	}
	return nil
}
