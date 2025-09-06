package search

import (
	"bytes"
	 "context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/models"
)

func IndexVideo(video models.Video) {
    data, err := json.Marshal(video)
    if err != nil {
        log.Printf("Error marshaling video: %s", err)
        return
    }

    req := bytes.NewReader(data)
    res, err := database.Es.Index(
        "videos",       
        req,
       database.Es.Index.WithDocumentID(video.ID.String()), 
        database.Es.Index.WithRefresh("true"),              
    )
    if err != nil {
        log.Printf("Error indexing video: %s", err)
        return
    }
    defer res.Body.Close()
    fmt.Println("Indexed video:", video.Title)
}


func SearchVideos(query string) []models.Video {
    var buf bytes.Buffer
    searchQuery := map[string]interface{}{
        "query": map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query":  query,
                "fields": []string{"title^3", "description", "tags^2"},
                "fuzziness": "AUTO",
            },
        },
    }

    if err := json.NewEncoder(&buf).Encode(searchQuery); err != nil {
        log.Printf("Error encoding query: %s", err)
        return nil
    }

    res, err := database.Es.Search(
        database.Es.Search.WithContext(context.Background()),
        database.Es.Search.WithIndex("videos"),
        database.Es.Search.WithBody(&buf),
        database.Es.Search.WithTrackTotalHits(true),
    )
    if err != nil {
        log.Printf("Search error: %s", err)
        return nil
    }
    defer res.Body.Close()

    var r map[string]interface{}
    if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
        log.Printf("Error parsing search response: %s", err)
        return nil
    }

    var videos []models.Video
    for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
        src := hit.(map[string]interface{})["_source"]
        data, _ := json.Marshal(src)
        var v models.Video
        json.Unmarshal(data, &v)
        videos = append(videos, v)
    }

    return videos
}
