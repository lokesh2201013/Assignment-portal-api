package services

import (
	"strings"
)

// DocumentChunker splits raw text into overlapping chunks.
type DocumentChunker struct {
	chunkSize int
	overlap   int
}

// NewDocumentChunker creates a new DocumentChunker with validated parameters.
func NewDocumentChunker(chunkSize, overlap int) *DocumentChunker {
	if chunkSize <= 0 {
		chunkSize = 250
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = 20
	}
	return &DocumentChunker{
		chunkSize: chunkSize,
		overlap:   overlap,
	}
}

// Split splits a string into chunks of characters based on chunkSize and overlap.
// It is rune-aware to prevent breaking multi-byte UTF-8 characters.
func (c *DocumentChunker) Split(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	runes := []rune(content)
	totalRunes := len(runes)
	step := c.chunkSize - c.overlap
	if step <= 0 {
		step = c.chunkSize
	}

	var chunks []string
	for i := 0; i < totalRunes; i += step {
		end := i + c.chunkSize
		if end > totalRunes {
			end = totalRunes
		}

		chunk := strings.TrimSpace(string(runes[i:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		if end == totalRunes {
			break
		}
	}

	return chunks
}
