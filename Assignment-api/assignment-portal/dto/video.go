package dto

type VideoPresignRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}
