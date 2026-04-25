package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	baseURL = "http://localhost:8080"
)

var (
	authToken     string
	adminToken    string
	userToken     string
	testUserID    string
	testAdminID   string
	testAssignID  string

	// Fixed emails and passwords
	testUserEmail     = "lokeshchoraria2@gmail.com"
	testUserPassword  = "password123"
	testAdminEmail    = "lokeshchoraria60369@gmail.com"
	testAdminPassword = "admin123"
)

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type User struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Branch   string `json:"branch,omitempty"`
	Semester int    `json:"semester,omitempty"`
}

type Assignment struct {
	AssignmentID string `json:"id"`
	Email        string `json:"email"`
	AdminID      string `json:"admin_id"`
	Task         string `json:"task"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	DueDate      string `json:"due_date"`
	Branch       string `json:"branch"`
	Semester     int    `json:"semester"`
	SubjectCode  string `json:"subject_code"`
}

func makeRequest(method, url, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func printResponse(resp *http.Response, err error) {
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n\n", string(body))
}

func testRegister() {
	fmt.Println("=== Testing Register Endpoint ===")

	// Register user
	user := User{
		Name:     "Test User",
		Email:    testUserEmail,
		Password: testUserPassword,
		Role:     "user",
		Branch:   "CSE",
		Semester: 3,
	}
	userData, _ := json.Marshal(user)
	resp, err := makeRequest("POST", baseURL+"/signup", "", bytes.NewReader(userData))
	fmt.Println("User Registration:")
	printResponse(resp, err)

	// Register admin
	admin := User{
		Name:     "Test Admin",
		Email:    testAdminEmail,
		Password: testAdminPassword,
		Role:     "admin",
	}
	adminData, _ := json.Marshal(admin)
	resp, err = makeRequest("POST", baseURL+"/signup", "", bytes.NewReader(adminData))
	fmt.Println("Admin Registration:")
	printResponse(resp, err)
}

func testLogin() {
	fmt.Println("=== Testing Login Endpoint ===")

	// User login
	loginData := map[string]string{
		"email":    testUserEmail,
		"password": testUserPassword,
	}
	data, _ := json.Marshal(loginData)
	resp, err := makeRequest("POST", baseURL+"/login", "", bytes.NewReader(data))

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Println("User Login:")
	printResponse(&http.Response{
		StatusCode: resp.StatusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, err)

	if resp.StatusCode == http.StatusOK {
		var loginResp LoginResponse
		if err := json.Unmarshal(body, &loginResp); err == nil {
			userToken = loginResp.Token
		}
	}

	// Admin login
	adminLogin := map[string]string{
		"email":    testAdminEmail,
		"password": testAdminPassword,
	}
	adminData, _ := json.Marshal(adminLogin)
	resp, err = makeRequest("POST", baseURL+"/login", "", bytes.NewReader(adminData))

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Println("Admin Login:")
	printResponse(&http.Response{
		StatusCode: resp.StatusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, err)

	if resp.StatusCode == http.StatusOK {
		var loginResp LoginResponse
		if err := json.Unmarshal(body, &loginResp); err == nil {
			adminToken = loginResp.Token
		}
	}
}

func testGetAllAdmins() {
	fmt.Println("=== Testing Get All Admins ===")
	resp, err := makeRequest("GET", baseURL+"/admin/getadmins", adminToken, nil)
	printResponse(resp, err)
}

func testGetAdminAssignments() {
	fmt.Println("=== Testing Get Admin Assignments ===")
	resp, err := makeRequest("GET", baseURL+"/admin/getassignments", adminToken, nil)
	printResponse(resp, err)
}

func testAssignToStudents() {
	fmt.Println("=== Testing Assign To Students ===")
	assignment := map[string]interface{}{
		"email":        testUserEmail,
		"task":         "Complete chapter 5 exercises",
		"status":       "pending",
		"due_date":     time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
		"branch":       "CSE",
		"semester":     3,
		"subject_code": "CS301",
	}
	data, _ := json.Marshal(assignment)
	resp, err := makeRequest("POST", baseURL+"/admin/assign_assignments", adminToken, bytes.NewReader(data))
	printResponse(resp, err)
}

func testAcceptAssignment() {
	fmt.Println("=== Testing Accept Assignment ===")
	userIDs := "some-assignment-id"
	url := baseURL + "/admin/assignments/accept?id=" + userIDs
	resp, err := makeRequest("POST", url, adminToken, nil)
	printResponse(resp, err)
}

func testRejectAssignment() {
	fmt.Println("=== Testing Reject Assignment ===")
	userIDs := "some-assignment-id"
	reason := "Incomplete submission"
	url := baseURL + "/admin/assignments/reject?id=" + userIDs + "&reason=" + reason
	resp, err := makeRequest("POST", url, adminToken, nil)
	printResponse(resp, err)
}

func testGetUserAssignments() {
	fmt.Println("=== Testing Get User Assignments ===")
	userID := "some-user-id"
	url := baseURL + "/user/assignments/" + userID
	resp, err := makeRequest("GET", url, userToken, nil)
	printResponse(resp, err)
}

func testGetSubmittedAssignments() {
	fmt.Println("=== Testing Get Submitted Assignments ===")
	assignmentID := "some-assignment-id"
	url := baseURL + "/admin/submissions?assignment_id=" + assignmentID
	resp, err := makeRequest("GET", url, adminToken, nil)
	printResponse(resp, err)
}

func testProtectedEndpoints() {
	fmt.Println("=== Testing Protected Endpoints Without Token ===")
	resp, err := makeRequest("GET", baseURL+"/admin/getassignments", "", nil)
	fmt.Println("Access without token:")
	printResponse(resp, err)

	resp, err = makeRequest("GET", baseURL+"/admin/getassignments", userToken, nil)
	fmt.Println("User accessing admin endpoint:")
	printResponse(resp, err)
}

func main() {
	fmt.Println("Starting API Tests...\n")

	testRegister()
	testLogin()

	if adminToken == "" || userToken == "" {
		fmt.Println("Login failed. Using placeholder tokens.")
		adminToken = "placeholder_admin_token"
		userToken = "placeholder_user_token"
	}

	testGetAllAdmins()
	testGetAdminAssignments()
	testAssignToStudents()
	testAcceptAssignment()
	testRejectAssignment()
	testGetUserAssignments()
	testGetSubmittedAssignments()
	testProtectedEndpoints()

	fmt.Println("All tests completed!")
}
