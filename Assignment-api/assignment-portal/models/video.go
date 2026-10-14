package models

import "github.com/google/uuid"

type Video struct {
	ID          uuid.UUID `json:"id"`
	TenantID    string    `json:"tenant_id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Tags        []string  `json:"tags"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
}
