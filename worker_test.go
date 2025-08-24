package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorker_HandleRequest(t *testing.T) {
	worker := NewWorker()

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		expected int
	}{
		{
			name:     "Root endpoint",
			method:   "GET",
			path:     "/",
			body:     "",
			expected: 200,
		},
		{
			name:     "Health endpoint",
			method:   "GET",
			path:     "/health",
			body:     "",
			expected: 200,
		},
		{
			name:     "Webhook endpoint with valid data",
			method:   "POST",
			path:     "/webhook",
			body:     `{"id":"test","venueName":"Test Venue","comment":"Test comment","url":"https://example.com","createdAt":"2024-01-01T12:00:00Z","userId":"user1"}`,
			expected: 200,
		},
		{
			name:     "Webhook endpoint with invalid JSON",
			method:   "POST",
			path:     "/webhook",
			body:     `invalid json`,
			expected: 400,
		},
		{
			name:     "Polling endpoint",
			method:   "POST",
			path:     "/poll",
			body:     "",
			expected: 200,
		},
		{
			name:     "Manual polling endpoint",
			method:   "POST",
			path:     "/manual-poll",
			body:     "",
			expected: 200,
		},
		{
			name:     "Get config endpoint",
			method:   "GET",
			path:     "/config",
			body:     "",
			expected: 200,
		},
		{
			name:     "Update config endpoint with valid JSON",
			method:   "POST",
			path:     "/config",
			body:     `{"misskeyInstance":"https://test.com","postTemplate":"Test template"}`,
			expected: 200,
		},
		{
			name:     "Update config endpoint with invalid JSON",
			method:   "POST",
			path:     "/config",
			body:     `invalid json`,
			expected: 400,
		},
		{
			name:     "Not found endpoint",
			method:   "GET",
			path:     "/notfound",
			body:     "",
			expected: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				Method:  tt.method,
				URL:     &URL{Path: tt.path},
				Body:    tt.body,
				Headers: make(map[string]string),
			}

			resp, err := worker.HandleRequest(context.Background(), req)
			if err != nil {
				t.Errorf("HandleRequest() error = %v", err)
				return
			}

			if resp.Status != tt.expected {
				t.Errorf("HandleRequest() status = %v, want %v", resp.Status, tt.expected)
			}
		})
	}
}

func TestWorker_CreatePostText(t *testing.T) {
	worker := NewWorker()

	checkin := SwarmCheckin{
		ID:        "test-id",
		VenueName: "Test Restaurant",
		Comment:   "Great food!",
		URL:       "https://swarmapp.com/checkin/test",
		CreatedAt: time.Now(),
		UserID:    "user1",
	}

	text := worker.createPostText(checkin)

	// Check if placeholders are replaced
	if !strings.Contains(text, "Test Restaurant") {
		t.Errorf("createPostText() does not contain venue name")
	}

	if !strings.Contains(text, "Great food!") {
		t.Errorf("createPostText() does not contain comment")
	}

	if !strings.Contains(text, "https://swarmapp.com/checkin/test") {
		t.Errorf("createPostText() does not contain URL")
	}
}

func TestWorker_VerifyWebhookSignature(t *testing.T) {
	worker := NewWorker()

	tests := []struct {
		name      string
		signature string
		expected  bool
	}{
		{
			name:      "No signature",
			signature: "",
			expected:  true,
		},
		{
			name:      "With signature",
			signature: "test-signature",
			expected:  true, // Currently always returns true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				Headers: map[string]string{
					"X-Swarm-Signature": tt.signature,
				},
			}

			result := worker.verifyWebhookSignature(req)
			if result != tt.expected {
				t.Errorf("verifyWebhookSignature() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSwarmCheckin_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "test-id",
		"venueName": "Test Venue",
		"comment": "Test comment",
		"url": "https://example.com",
		"imageUrl": "https://example.com/image.jpg",
		"createdAt": "2024-01-01T12:00:00Z",
		"userId": "user1"
	}`

	var checkin SwarmCheckin
	err := json.Unmarshal([]byte(jsonData), &checkin)
	if err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}

	if checkin.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", checkin.ID)
	}

	if checkin.VenueName != "Test Venue" {
		t.Errorf("Expected VenueName 'Test Venue', got '%s'", checkin.VenueName)
	}

	if checkin.Comment != "Test comment" {
		t.Errorf("Expected Comment 'Test comment', got '%s'", checkin.Comment)
	}

	if checkin.URL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", checkin.URL)
	}

	if checkin.ImageURL != "https://example.com/image.jpg" {
		t.Errorf("Expected ImageURL 'https://example.com/image.jpg', got '%s'", checkin.ImageURL)
	}

	if checkin.UserID != "user1" {
		t.Errorf("Expected UserID 'user1', got '%s'", checkin.UserID)
	}
}

func TestResponse_MarshalJSON(t *testing.T) {
	response := APIResponse{
		Success: true,
		Message: "Test message",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Errorf("Failed to marshal JSON: %v", err)
	}

	var decoded APIResponse
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.Success != response.Success {
		t.Errorf("Expected Success %v, got %v", response.Success, decoded.Success)
	}

	if decoded.Message != response.Message {
		t.Errorf("Expected Message '%s', got '%s'", response.Message, decoded.Message)
	}
}

func TestWorker_PollingFunctions(t *testing.T) {
	worker := NewWorker()

	// Test getLastCheckinTime
	lastTime, err := worker.getLastCheckinTime(context.Background())
	if err != nil {
		t.Errorf("getLastCheckinTime() error = %v", err)
	}

	// Should return a time within the last hour (with some tolerance)
	if time.Since(lastTime) > time.Hour+time.Minute {
		t.Errorf("getLastCheckinTime() returned time too old: %v", lastTime)
	}

	// Test updateLastCheckinTime
	testTime := time.Now()
	err = worker.updateLastCheckinTime(context.Background(), testTime)
	if err != nil {
		t.Errorf("updateLastCheckinTime() error = %v", err)
	}
}

func TestWorker_ConfigManagement(t *testing.T) {
	worker := NewWorker()

	// Test initial configuration
	originalInstance := worker.config.MisskeyInstance
	originalTemplate := worker.config.PostTemplate

	// Test configuration update
	updateData := map[string]interface{}{
		"misskeyInstance": "https://test-instance.com",
		"postTemplate":    "Test template {venueName}",
		"visibility":      "home",
		"pollingInterval": 10.0,
	}

	updateJSON, err := json.Marshal(updateData)
	if err != nil {
		t.Errorf("Failed to marshal update data: %v", err)
	}

	req := &Request{
		Method:  "POST",
		URL:     &URL{Path: "/config"},
		Body:    string(updateJSON),
		Headers: make(map[string]string),
	}

	resp, err := worker.HandleRequest(context.Background(), req)
	if err != nil {
		t.Errorf("HandleRequest() error = %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("Expected status 200, got %d", resp.Status)
	}

	// Verify configuration was updated
	if worker.config.MisskeyInstance != "https://test-instance.com" {
		t.Errorf("MisskeyInstance not updated correctly")
	}

	if worker.config.PostTemplate != "Test template {venueName}" {
		t.Errorf("PostTemplate not updated correctly")
	}

	if worker.config.Visibility != "home" {
		t.Errorf("Visibility not updated correctly")
	}

	if worker.config.PollingInterval != 10 {
		t.Errorf("PollingInterval not updated correctly")
	}

	// Test get config endpoint
	req = &Request{
		Method:  "GET",
		URL:     &URL{Path: "/config"},
		Body:    "",
		Headers: make(map[string]string),
	}

	resp, err = worker.HandleRequest(context.Background(), req)
	if err != nil {
		t.Errorf("HandleRequest() error = %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("Expected status 200, got %d", resp.Status)
	}

	// Parse response to verify it contains expected fields
	var configResponse map[string]interface{}
	err = json.Unmarshal([]byte(resp.Body), &configResponse)
	if err != nil {
		t.Errorf("Failed to unmarshal config response: %v", err)
	}

	if configResponse["misskeyInstance"] != "https://test-instance.com" {
		t.Errorf("Config response does not contain updated misskeyInstance")
	}

	// Restore original configuration
	worker.config.MisskeyInstance = originalInstance
	worker.config.PostTemplate = originalTemplate
}

func TestWorker_SwarmAPIResponseParsing(t *testing.T) {
	// Test Swarm API response parsing
	apiResponse := `{
		"response": {
			"checkins": {
				"items": [
					{
						"id": "test-checkin-1",
						"createdAt": 1704067200,
						"venue": {
							"name": "Test Restaurant"
						},
						"shout": "Great food!",
						"url": "https://swarmapp.com/checkin/test1"
					},
					{
						"id": "test-checkin-2",
						"createdAt": 1704067800,
						"venue": {
							"name": "Test Cafe"
						},
						"shout": "Nice coffee!",
						"url": "https://swarmapp.com/checkin/test2"
					}
				]
			}
		}
	}`

	var swarmResp SwarmAPIResponse
	err := json.Unmarshal([]byte(apiResponse), &swarmResp)
	if err != nil {
		t.Errorf("Failed to unmarshal Swarm API response: %v", err)
	}

	if len(swarmResp.Response.Checkins.Items) != 2 {
		t.Errorf("Expected 2 checkins, got %d", len(swarmResp.Response.Checkins.Items))
	}

	// Check first checkin
	firstCheckin := swarmResp.Response.Checkins.Items[0]
	if firstCheckin.ID != "test-checkin-1" {
		t.Errorf("Expected ID 'test-checkin-1', got '%s'", firstCheckin.ID)
	}

	if firstCheckin.Venue.Name != "Test Restaurant" {
		t.Errorf("Expected venue name 'Test Restaurant', got '%s'", firstCheckin.Venue.Name)
	}

	if firstCheckin.Shout != "Great food!" {
		t.Errorf("Expected shout 'Great food!', got '%s'", firstCheckin.Shout)
	}

	// Check second checkin
	secondCheckin := swarmResp.Response.Checkins.Items[1]
	if secondCheckin.ID != "test-checkin-2" {
		t.Errorf("Expected ID 'test-checkin-2', got '%s'", secondCheckin.ID)
	}

	if secondCheckin.Venue.Name != "Test Cafe" {
		t.Errorf("Expected venue name 'Test Cafe', got '%s'", secondCheckin.Venue.Name)
	}
}
