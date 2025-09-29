package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// AIConversionRequest contains the data needed for AI conversion
type AIConversionRequest struct {
	UnstructuredText string
	Model            string
	SystemPrompt     string
	UserID           int
	FamilyID         int
}

// AIConversionResult contains the result of AI conversion
type AIConversionResult struct {
	GeneratedJSON string
	TokensUsed    int
	Model         string
	ResponseTime  int64 // Milliseconds
}

// GetDefaultAIModel returns the default AI model
func GetDefaultAIModel() string {
	return "models/gemini-2.5-flash"
}

// ValidateAIConfiguration checks if the AI provider is properly configured
func ValidateAIConfiguration() error {
	if os.Getenv("GEMINI_API_KEY") == "" {
		return errors.New("GEMINI_API_KEY environment variable not set")
	}
	return nil
}

// ListAvailableModels returns the list of available Gemini models
func ListAvailableModels() ([]string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("Gemini API key not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var listResponse struct {
		Models []struct {
			Name             string   `json:"name"`
			DisplayName      string   `json:"displayName"`
			Description      string   `json:"description"`
			SupportedActions []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var models []string
	for _, model := range listResponse.Models {
		// Check if it supports generateContent
		for _, action := range model.SupportedActions {
			if action == "generateContent" {
				models = append(models, model.Name)
				break
			}
		}
	}

	return models, nil
}

// ConvertToJSON calls the AI API to convert unstructured text to JSON
// Currently implemented using Gemini
func ConvertToJSON(request AIConversionRequest) (*AIConversionResult, error) {
	startTime := time.Now()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("Gemini API key not configured")
	}

	// Prepare Gemini API request
	geminiRequest := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{
						"text": request.SystemPrompt + "\n\n" + request.UnstructuredText,
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0.7,
			"maxOutputTokens":  8192,
			"responseMimeType": "application/json",
		},
	}

	jsonBody, err := json.Marshal(geminiRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API request
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:generateContent?key=%s",
		request.Model, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			TotalTokenCount int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResponse.Candidates) == 0 || len(geminiResponse.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("no response from Gemini")
	}

	// Check if response was truncated
	finishReason := geminiResponse.Candidates[0].FinishReason
	if finishReason == "MAX_TOKENS" {
		return nil, errors.New("response truncated - increase maxOutputTokens or simplify input")
	}
	if finishReason != "STOP" && finishReason != "" {
		return nil, fmt.Errorf("unexpected finish reason: %s", finishReason)
	}

	generatedText := geminiResponse.Candidates[0].Content.Parts[0].Text

	return &AIConversionResult{
		GeneratedJSON: generatedText,
		TokensUsed:    geminiResponse.UsageMetadata.TotalTokenCount,
		Model:         request.Model,
		ResponseTime:  time.Since(startTime).Milliseconds(),
	}, nil
}
