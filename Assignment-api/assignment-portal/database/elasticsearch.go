package database

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/lokesh2201013/config"
)

var Es *elasticsearch.Client

func InitES(cfg config.Config) (*elasticsearch.Client, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.ElasticAddresses,
	})
	if err != nil {
		return nil, err
	}

	Es = es
	return es, nil
}
