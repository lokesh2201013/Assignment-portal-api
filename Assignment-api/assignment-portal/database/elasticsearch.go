package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

var Es *elasticsearch.Client

func InitES() error {
	address := strings.TrimSpace(os.Getenv("ELASTICSEARCH_URL"))
	if address == "" {
		address = "http://localhost:9200"
	}
	parsed, err := url.ParseRequestURI(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("invalid ELASTICSEARCH_URL %q: expected an http(s) URL with a host", address)
	}

	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{address}})
	if err != nil {
		return fmt.Errorf("create Elasticsearch client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := es.Info(es.Info.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("connect to Elasticsearch at %s: %w", address, err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("Elasticsearch at %s returned %s", address, res.Status())
	}

	Es = es
	return nil
}
