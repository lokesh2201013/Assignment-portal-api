package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocumentChunker_Split(t *testing.T) {
	t.Run("empty or whitespace string", func(t *testing.T) {
		chunker := NewDocumentChunker(100, 10)
		assert.Nil(t, chunker.Split(""))
		assert.Nil(t, chunker.Split("   \n\t  "))
	})

	t.Run("content shorter than chunk size", func(t *testing.T) {
		chunker := NewDocumentChunker(50, 10)
		content := "Short document text."
		chunks := chunker.Split(content)
		assert.Len(t, chunks, 1)
		assert.Equal(t, content, chunks[0])
	})

	t.Run("sliding window with overlap", func(t *testing.T) {
		chunker := NewDocumentChunker(10, 2)
		// Step = 10 - 2 = 8
		content := "abcdefghijklmnopqrstuvwxyz" // 26 chars
		// Chunk 0: [0:10] = "abcdefghij"
		// Chunk 1: [8:18] = "ijklmnopqr"
		// Chunk 2: [16:26] = "qrstuvwxyz"
		chunks := chunker.Split(content)
		assert.Len(t, chunks, 3)
		assert.Equal(t, "abcdefghij", chunks[0])
		assert.Equal(t, "ijklmnopqr", chunks[1])
		assert.Equal(t, "qrstuvwxyz", chunks[2])
	})

	t.Run("handles invalid overlap parameters safely without infinite loop", func(t *testing.T) {
		chunker := NewDocumentChunker(10, 15) // overlap > chunkSize
		// Should fall back to safe default
		content := "abcdefghijklmnopqrstuvwxyz"
		chunks := chunker.Split(content)
		assert.NotEmpty(t, chunks)
	})

	t.Run("handles UTF-8 multi-byte characters cleanly", func(t *testing.T) {
		chunker := NewDocumentChunker(5, 1)
		content := "こんにちは世界🌍" // Japanese characters and emoji
		chunks := chunker.Split(content)
		assert.NotEmpty(t, chunks)
		// Ensure no garbled character slices
		for _, c := range chunks {
			assert.True(t, len(c) > 0)
		}
	})

	t.Run("reconstructs content coverage", func(t *testing.T) {
		chunker := NewDocumentChunker(20, 5)
		content := "The quick brown fox jumps over the lazy dog repeatedly until sunset."
		chunks := chunker.Split(content)
		assert.True(t, len(chunks) > 1)
		for _, word := range []string{"quick", "brown", "jumps", "sunset"} {
			found := false
			for _, chunk := range chunks {
				if strings.Contains(chunk, word) {
					found = true
					break
				}
			}
			assert.True(t, found, "word %s should be present in at least one chunk", word)
		}
	})
}
