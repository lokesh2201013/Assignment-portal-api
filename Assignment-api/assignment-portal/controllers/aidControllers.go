package controllers

import (
	"context"
	"fmt"

	//"strings"

	//"io/ioutil"
	//"fmt"
	//"github.com/atgsgrouptest/genet-microservice/RAG-service/Logger"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/models"
	"github.com/lokesh2201013/search"
	"github.com/lokesh2201013/utils"

	//	"context"
	//"fmt"
	"log"
	//"time"

	"cloud.google.com/go/storage"
	//"google.golang.org/api/option"
	//"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
	"google.golang.org/grpc"

	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/status"
	"google.golang.org/grpc/credentials/insecure"
	//"go.uber.org/zap"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	//"mime/multipart"
	//"github.com/atgsgrouptest/genet-microservice/RAG-service/embedding"
)

func UploadFileHandler(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Failed to parse multipart form",
			"details": err.Error(),
		})
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No files uploaded",
		})
	}
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	fmt.Println("gRPC connection created")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to connect to gRPC server",
			"details": err.Error(),
		})
	}
	defer conn.Close()

	// gRPC client from app context or global
	client := pb.NewRAGServiceClient(conn)

	// Upload each file to the gRPC server
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to open file",
				"details": err.Error(),
			})
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to read file",
				"details": err.Error(),
			})
		}

		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := &pb.FileUploadRequest{
			Filename: fileHeader.Filename,
			Content:  content,
		}

		res, err := client.UploadFile(ctx, req)
		if err != nil || !res.Message {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "gRPC Upload failed",
				"details": err.Error(),
			})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Files processed successfully",
	})
}


func GetHelp(c *fiber.Ctx) error {
	type QueryRequest struct {
		Query string `json:"query"`
	}
	var queryRequest QueryRequest
	if err := c.BodyParser(&queryRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	if queryRequest.Query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query cannot be empty",
		})
	}

	query := queryRequest.Query
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query parameter 'query' is required",
		})
	}

	// gRPC client (get it from your app context or global variable)
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	fmt.Println("gRPC connection created")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to connect to gRPC server",
			"details": err.Error(),
		})
	}
	defer conn.Close()


	client := pb.NewRAGServiceClient(conn)


	req := &pb.QueryRequest{
		Query: query,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := client.QueryWithContext(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "gRPC call failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"answer": res.Answer,})
}

func GetData(c *fiber.Ctx) error {
	type QueryRequest struct {
		Query string `json:"query"`
	}

	var queryRequest QueryRequest

	if err := c.BodyParser(&queryRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if queryRequest.Query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query cannot be empty",
		})
	}

assignment := `assignments {
    assignment_id: uuid
    email: string
    admin_id: uuid
    task: string
    updated_at: string
    due_date: string
    branch: string
    semester: int
    subject_code: string
}`

user := `users {
    user_id: string
    name: string
    email: string
    password: string
    role: string
    branch: string
    semester: int
}`

submit := `submit_assignments {
    submission_id: uuid
    assignment_id: uuid
    user_id: uuid
    status: string
    file: string
    image: string
    comments: string
    late_submission: bool
    created_at: string
}`


	context := fmt.Sprintf("%+v %+v %+v", assignment, user, submit)
	fmt.Println(assignment,user,submit)
	prompt := "Return ONLY a raw SQL query without any explanations with ```sql fences. These are your DB models: " + context + ". Query: " + queryRequest.Query


	// Call Ollama
	payload := map[string]interface{}{
		"model":  "llama3.1:8b",
		"prompt": prompt,
		"stream": false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error marshaling Ollama payload",
		})
	}

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(data))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to call Ollama",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ollama API error: %s", string(body)),
		})
	}
      
	var result struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to decode Ollama response",
		})
	}
	query := utils.CleanSQL(result.Response)
	fmt.Println("Generated SQL Query:", query)
	rows, err := database.DB.Raw(query).Rows()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("SQL error: %v", err),
			"query": query,
		})
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get columns",
		})
	}

	results := []map[string]interface{}{}

	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columnValues := make([]interface{}, len(columns))
		columnPointers := make([]interface{}, len(columns))
		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Row scan error: %v", err),
			})
		}

		// Create map for this row
		rowMap := map[string]interface{}{}
		for i, colName := range columns {
			val := columnValues[i]
			// Convert []byte to string if needed
			if b, ok := val.([]byte); ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = val
			}
		}

		results = append(results, rowMap)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": results,
		"query": query,
	})
}
func generateSignedURL(bucketName, objectName, serviceAccountKey string) (string, error) {
	//ctx := context.Background()

	// Signed URL options
	opts := &storage.SignedURLOptions{
		GoogleAccessID: "your-service-account@project-id.iam.gserviceaccount.com",
		PrivateKey:     []byte(serviceAccountKey),
		Method:         "PUT",
		Expires:        time.Now().Add(15 * time.Minute),
		ContentType:    "video/mp4", // restrict uploads to video/mp4
	}

	// Generate signed URL
	url, err := storage.SignedURL(bucketName, objectName, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %v", err)
	}

	return url, nil
}
func GetPresignedURL(c *fiber.Ctx) error {
	var videoData models.Video
	if err := c.BodyParser(&videoData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	if videoData.Title == "" || videoData.Description == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Title and Description are required",
		})
	}
	bucketName := "my-video-bucket"
	objectName := "uploads/user123/video.mp4"

	serviceAccountKey := `-----BEGIN PRIVATE KEY-----
                               YOUR_PRIVATE_KEY_CONTENT
                               -----END PRIVATE KEY-----`
    
	url, err := generateSignedURL(bucketName, objectName, serviceAccountKey)
	if err != nil {
		log.Fatalf("Error generating signed URL: %v", err)
	}
	search.IndexVideo(videoData)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"presigned_url": url,
	})

}

