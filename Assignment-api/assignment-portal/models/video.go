package models

import "github.com/google/uuid"

type Video struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	URL string    `json:"url"`
	Title string  `json:"title"`
	Tags  []string `json:"tags" gorm:"type:text[]"`
	Status string `json:"status"`
	Description string `json:"description"`
}