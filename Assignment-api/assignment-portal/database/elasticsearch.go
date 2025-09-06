package database

import (
    "github.com/elastic/go-elasticsearch/v8"
    "log"
)

var Es *elasticsearch.Client

func InitES() {
    cfg := elasticsearch.Config{
        Addresses: []string{
            "http://localhost:9200", // your ES endpoint
        },
    }
    es, err := elasticsearch.NewClient(cfg)
    if err != nil {
        log.Fatalf("Error creating ES client: %s", err)
    }

    Es = es
}
