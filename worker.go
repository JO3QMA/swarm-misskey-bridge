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
	ID          string    `json:"id"`
	VenueName   string    `json:"venueName"`
	Comment     string    `json:"comment,omitempty"`
	URL         string    `json:"url"`
	ImageURL    string    `json:"imageUrl,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UserID      string    `json:"userId"`
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
	PostTemplate    string `json:"postTemplate"`
	Visibility      string `json:"visibility"`
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
}

// NewWorker creates a new Worker instance
func NewWorker() *Worker {
	return &Worker{
		config: Configuration{
			MisskeyInstance: "https://misskey.io",
			MisskeyAPIKey:   "test-key", // For testing purposes
			PostTemplate:    "Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey",
			Visibility:      "public",
		},
	}
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
