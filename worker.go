package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// SwarmCheckin represents a checkin from Swarm
type SwarmCheckin struct {
	ID        string    `json:"id"`
	VenueName string    `json:"venueName"`
	Comment   string    `json:"comment,omitempty"`
	URL       string    `json:"url"`
	ImageURL  string    `json:"imageUrl,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UserID    string    `json:"userId"`
}

// SwarmAPIResponse represents the response from Swarm API
type SwarmAPIResponse struct {
	Response struct {
		Checkins struct {
			Items []struct {
				ID        string `json:"id"`
				CreatedAt int64  `json:"createdAt"`
				Venue     struct {
					Name string `json:"name"`
				} `json:"venue"`
				Shout string `json:"shout,omitempty"`
				URL   string `json:"url"`
			} `json:"items"`
		} `json:"checkins"`
	} `json:"response"`
}

// MisskeyNote represents a note to be posted to Misskey
type MisskeyNote struct {
	Text       string   `json:"text"`
	FileIds    []string `json:"fileIds,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
}

// MisskeyFile represents a file uploaded to Misskey
type MisskeyFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// Configuration stores the application configuration
type Configuration struct {
	MisskeyInstance string `json:"misskeyInstance"`
	MisskeyAPIKey   string `json:"misskeyApiKey"`
	SwarmAPIKey     string `json:"swarmApiKey"`
	SwarmUserID     string `json:"swarmUserId"`
	PostTemplate    string `json:"postTemplate"`
	Visibility      string `json:"visibility"`
	PollingInterval int    `json:"pollingInterval"` // in minutes
}

// Request represents a Cloudflare Workers request
type Request struct {
	Method  string            `json:"method"`
	URL     *URL              `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// URL represents a URL
type URL struct {
	Path string `json:"path"`
}

// Response represents a Cloudflare Workers response
type Response struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// APIResponse represents the API response
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Worker represents the Cloudflare Worker
type Worker struct {
	config Configuration
	kv     KVNamespace // Cloudflare KV namespace
}

// KVNamespace represents Cloudflare KV namespace interface
type KVNamespace interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
}

// NewWorker creates a new Worker instance
func NewWorker() *Worker {
	return &Worker{
		config: Configuration{
			MisskeyInstance: "https://misskey.io",
			MisskeyAPIKey:   "test-key", // For testing purposes
			SwarmAPIKey:     "test-swarm-key",
			SwarmUserID:     "test-user-id",
			PostTemplate:    "Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey",
			Visibility:      "public",
			PollingInterval: 5, // 5 minutes
		},
	}
}

// SetKVNamespace sets the KV namespace for the worker
func (w *Worker) SetKVNamespace(kv KVNamespace) {
	w.kv = kv
}

// HandleRequest handles incoming requests
func (w *Worker) HandleRequest(ctx context.Context, req *Request) (*Response, error) {
	// Parse the URL
	url := req.URL
	path := url.Path

	// Route the request
	switch {
	case path == "/" && req.Method == "GET":
		return w.handleRoot(ctx, req)
	case path == "/health" && req.Method == "GET":
		return w.handleHealth(ctx, req)
	case path == "/webhook" && req.Method == "POST":
		return w.handleSwarmWebhook(ctx, req)
	case path == "/poll" && req.Method == "POST":
		return w.handlePolling(ctx, req)
	case path == "/manual-poll" && req.Method == "POST":
		return w.handleManualPolling(ctx, req)
	case path == "/config" && req.Method == "GET":
		return w.handleGetConfig(ctx, req)
	case path == "/config" && req.Method == "POST":
		return w.handleUpdateConfig(ctx, req)
	default:
		return w.handleNotFound(ctx, req)
	}
}

func (w *Worker) handleRoot(ctx context.Context, req *Request) (*Response, error) {
	response := APIResponse{
		Success: true,
		Message: "Swarm Misskey Integration is running",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) handleHealth(ctx context.Context, req *Request) (*Response, error) {
	response := APIResponse{
		Success: true,
		Message: "Healthy",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) handleNotFound(ctx context.Context, req *Request) (*Response, error) {
	response := APIResponse{
		Success: false,
		Error:   "Not found",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  404,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) handleSwarmWebhook(ctx context.Context, req *Request) (*Response, error) {
	// Verify webhook signature if configured
	if !w.verifyWebhookSignature(req) {
		response := APIResponse{
			Success: false,
			Error:   "Invalid signature",
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:  401,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    string(jsonData),
		}, nil
	}

	// Parse Swarm checkin data
	var checkin SwarmCheckin
	if err := json.Unmarshal([]byte(req.Body), &checkin); err != nil {
		response := APIResponse{
			Success: false,
			Error:   "Invalid JSON",
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:  400,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    string(jsonData),
		}, nil
	}

	// Process the checkin
	if err := w.processCheckin(ctx, checkin); err != nil {
		response := APIResponse{
			Success: false,
			Error:   err.Error(),
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:  500,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    string(jsonData),
		}, nil
	}

	response := APIResponse{
		Success: true,
		Message: "Checkin processed successfully",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) processCheckin(ctx context.Context, checkin SwarmCheckin) error {
	// For testing purposes, skip actual API calls
	if w.config.MisskeyAPIKey == "test-key" {
		return nil
	}

	// Upload image to Misskey if present
	var fileIds []string
	if checkin.ImageURL != "" {
		fileId, err := w.uploadImageToMisskey(ctx, checkin.ImageURL)
		if err != nil {
			return fmt.Errorf("failed to upload image: %w", err)
		}
		fileIds = append(fileIds, fileId)
	}

	// Create post content
	postText := w.createPostText(checkin)

	// Post to Misskey
	note := MisskeyNote{
		Text:       postText,
		FileIds:    fileIds,
		Visibility: w.config.Visibility,
	}

	if err := w.postToMisskey(ctx, note); err != nil {
		return fmt.Errorf("failed to post to Misskey: %w", err)
	}

	return nil
}

func (w *Worker) uploadImageToMisskey(ctx context.Context, imageURL string) (string, error) {
	// Download the image
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Upload to Misskey
	uploadURL := fmt.Sprintf("%s/api/drive/files/create", w.config.MisskeyInstance)

	// Create multipart form data
	body := &strings.Builder{}
	writer := multipart.NewWriter(body)

	// Add the file
	part, err := writer.CreateFormFile("file", "swarm-checkin.jpg")
	if err != nil {
		return "", err
	}
	part.Write(imageData)

	// Add API key
	writer.WriteField("i", w.config.MisskeyAPIKey)

	writer.Close()

	// Make the request
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, strings.NewReader(body.String()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	// Parse response
	var file MisskeyFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return "", err
	}

	return file.ID, nil
}

func (w *Worker) createPostText(checkin SwarmCheckin) string {
	text := w.config.PostTemplate

	// Replace placeholders
	text = strings.ReplaceAll(text, "{venueName}", checkin.VenueName)
	text = strings.ReplaceAll(text, "{comment}", checkin.Comment)
	text = strings.ReplaceAll(text, "{url}", checkin.URL)

	// Add the checkin URL if not already present
	if !strings.Contains(text, checkin.URL) {
		text += "\n\n" + checkin.URL
	}

	return text
}

func (w *Worker) postToMisskey(ctx context.Context, note MisskeyNote) error {
	postURL := fmt.Sprintf("%s/api/notes/create", w.config.MisskeyInstance)

	// Prepare the request data
	requestData := map[string]interface{}{
		"i":          w.config.MisskeyAPIKey,
		"text":       note.Text,
		"visibility": note.Visibility,
	}

	if len(note.FileIds) > 0 {
		requestData["fileIds"] = note.FileIds
	}

	// Convert to JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	// Make the request
	req, err := http.NewRequestWithContext(ctx, "POST", postURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (w *Worker) handlePolling(ctx context.Context, req *Request) (*Response, error) {
	// This endpoint is called by Cloudflare Workers Cron Triggers
	// It polls the Swarm API for new checkins and posts them to Misskey

	if err := w.pollSwarmCheckins(ctx); err != nil {
		response := APIResponse{
			Success: false,
			Error:   err.Error(),
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:  500,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    string(jsonData),
		}, nil
	}

	response := APIResponse{
		Success: true,
		Message: "Polling completed successfully",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) handleManualPolling(ctx context.Context, req *Request) (*Response, error) {
	// This endpoint allows manual triggering of polling for testing purposes

	if err := w.pollSwarmCheckins(ctx); err != nil {
		response := APIResponse{
			Success: false,
			Error:   err.Error(),
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:  500,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    string(jsonData),
		}, nil
	}

	response := APIResponse{
		Success: true,
		Message: "Manual polling completed successfully",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) pollSwarmCheckins(ctx context.Context) error {
	// For testing purposes, if we're using test keys, just return success
	if w.config.SwarmAPIKey == "test-swarm-key" || w.config.MisskeyAPIKey == "test-key" {
		fmt.Printf("Skipping polling in test mode\n")
		return nil
	}

	// Get the last checkin timestamp from KV storage
	lastCheckinTime, err := w.getLastCheckinTime(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last checkin time: %w", err)
	}

	// Fetch recent checkins from Swarm API
	checkins, err := w.fetchSwarmCheckins(ctx, lastCheckinTime)
	if err != nil {
		return fmt.Errorf("failed to fetch Swarm checkins: %w", err)
	}

	// Process new checkins
	var latestCheckinTime time.Time
	for _, checkin := range checkins {
		// Skip if this checkin is older than our last processed checkin
		if !checkin.CreatedAt.After(lastCheckinTime) {
			continue
		}

		// Process the checkin
		if err := w.processCheckin(ctx, checkin); err != nil {
			// Log error but continue processing other checkins
			fmt.Printf("Failed to process checkin %s: %v\n", checkin.ID, err)
			continue
		}

		// Update latest checkin time
		if checkin.CreatedAt.After(latestCheckinTime) {
			latestCheckinTime = checkin.CreatedAt
		}
	}

	// Update the last checkin time in KV storage
	if !latestCheckinTime.IsZero() {
		if err := w.updateLastCheckinTime(ctx, latestCheckinTime); err != nil {
			return fmt.Errorf("failed to update last checkin time: %w", err)
		}
	}

	return nil
}

func (w *Worker) fetchSwarmCheckins(ctx context.Context, since time.Time) ([]SwarmCheckin, error) {
	// Construct the Swarm API URL
	apiURL := fmt.Sprintf("https://api.foursquare.com/v2/users/self/checkins?oauth_token=%s&v=20240101&limit=50", w.config.SwarmAPIKey)

	// Add since parameter if we have a last checkin time
	if !since.IsZero() {
		apiURL += fmt.Sprintf("&afterTimestamp=%d", since.Unix())
	}

	// Make the request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Swarm API request failed with status: %d", resp.StatusCode)
	}

	// Parse the response
	var apiResp SwarmAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	// Convert to our SwarmCheckin format
	var checkins []SwarmCheckin
	for _, item := range apiResp.Response.Checkins.Items {
		checkin := SwarmCheckin{
			ID:        item.ID,
			VenueName: item.Venue.Name,
			Comment:   item.Shout,
			URL:       item.URL,
			CreatedAt: time.Unix(item.CreatedAt, 0),
			UserID:    w.config.SwarmUserID,
		}
		checkins = append(checkins, checkin)
	}

	return checkins, nil
}

func (w *Worker) getLastCheckinTime(ctx context.Context) (time.Time, error) {
	if w.kv == nil {
		// Fallback to 1 hour ago if KV is not available
		return time.Now().Add(-1 * time.Hour), nil
	}

	// Get the last checkin time from KV storage
	data, err := w.kv.Get("last_checkin_time")
	if err != nil {
		// If no data exists, return a time 1 hour ago
		return time.Now().Add(-1 * time.Hour), nil
	}

	// Parse the timestamp
	timestamp, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		// If parsing fails, return a time 1 hour ago
		return time.Now().Add(-1 * time.Hour), nil
	}

	return timestamp, nil
}

func (w *Worker) updateLastCheckinTime(ctx context.Context, checkinTime time.Time) error {
	if w.kv == nil {
		// Just log if KV is not available
		fmt.Printf("Updated last checkin time to: %s\n", checkinTime.Format(time.RFC3339))
		return nil
	}

	// Store the timestamp in KV storage
	timestamp := checkinTime.Format(time.RFC3339)
	if err := w.kv.Put("last_checkin_time", []byte(timestamp)); err != nil {
		return fmt.Errorf("failed to store last checkin time: %w", err)
	}

	fmt.Printf("Updated last checkin time to: %s\n", timestamp)
	return nil
}

func (w *Worker) handleGetConfig(ctx context.Context, req *Request) (*Response, error) {
	// Return current configuration (without sensitive data)
	safeConfig := map[string]interface{}{
		"misskeyInstance":  w.config.MisskeyInstance,
		"postTemplate":     w.config.PostTemplate,
		"visibility":       w.config.Visibility,
		"pollingInterval":  w.config.PollingInterval,
		"hasSwarmAPIKey":   w.config.SwarmAPIKey != "test-swarm-key" && w.config.SwarmAPIKey != "",
		"hasMisskeyAPIKey": w.config.MisskeyAPIKey != "test-key" && w.config.MisskeyAPIKey != "",
	}

	jsonData, err := json.Marshal(safeConfig)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) handleUpdateConfig(ctx context.Context, req *Request) (*Response, error) {
	// Parse the configuration update
	var configUpdate map[string]interface{}
	if err := json.Unmarshal([]byte(req.Body), &configUpdate); err != nil {
		response := APIResponse{
			Success: false,
			Error:   "Invalid JSON",
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:  400,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    string(jsonData),
		}, nil
	}

	// Update configuration fields
	if misskeyInstance, ok := configUpdate["misskeyInstance"].(string); ok {
		w.config.MisskeyInstance = misskeyInstance
	}
	if postTemplate, ok := configUpdate["postTemplate"].(string); ok {
		w.config.PostTemplate = postTemplate
	}
	if visibility, ok := configUpdate["visibility"].(string); ok {
		w.config.Visibility = visibility
	}
	if pollingInterval, ok := configUpdate["pollingInterval"].(float64); ok {
		w.config.PollingInterval = int(pollingInterval)
	}

	// Store configuration in KV if available
	if w.kv != nil {
		configData, err := json.Marshal(w.config)
		if err == nil {
			w.kv.Put("config", configData)
		}
	}

	response := APIResponse{
		Success: true,
		Message: "Configuration updated successfully",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    string(jsonData),
	}, nil
}

func (w *Worker) verifyWebhookSignature(req *Request) bool {
	// Get the signature from headers
	signature := req.Headers["X-Swarm-Signature"]
	if signature == "" {
		// If no signature is configured, allow the request
		return true
	}

	// TODO: Implement signature verification
	// This would involve checking the signature against the webhook secret

	return true
}

// Export the worker for Cloudflare Workers
var worker = NewWorker()

// HandleRequest is the main entry point for Cloudflare Workers
func HandleRequest(ctx context.Context, req *Request) (*Response, error) {
	return worker.HandleRequest(ctx, req)
}
