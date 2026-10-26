package database

import (
	"strings"
	"testing"
)

func TestInitESRejectsInvalidConfiguredURL(t *testing.T) {
	t.Setenv("ELASTICSEARCH_URL", "ftp://search.example.test:9200")
	err := InitES()
	if err == nil || !strings.Contains(err.Error(), "ELASTICSEARCH_URL") {
		t.Fatalf("expected invalid ELASTICSEARCH_URL error, got %v", err)
	}
}
