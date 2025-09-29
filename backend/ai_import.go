package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
)

func RegisterAIImportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ProcessAIImport)
	vbeam.RegisterProc(app, ListAIModels)
}

// AI Import Request/Response types
type ProcessAIImportRequest struct {
	PersonId         int    `json:"personId"` // The person to import data for
	UnstructuredText string `json:"unstructuredText"`
	GenerateFile     bool   `json:"generateFile"` // Whether to save intermediate file
}

// AIImportDataStructure represents the simplified AI import format for a single person
type AIImportDataStructure struct {
	PersonId        int               `json:"personId"`
	Heights         []ImportHeight    `json:"heights"`
	Weights         []ImportWeight    `json:"weights"`
	Milestones      []ExportMilestone `json:"milestones"`
	TotalHeights    int               `json:"total_heights"`
	TotalWeights    int               `json:"total_weights"`
	TotalMilestones int               `json:"total_milestones"`
}

type ProcessAIImportResponse struct {
	Success            bool     `json:"success"`
	GeneratedJSON      string   `json:"generatedJSON"`
	FilePath           string   `json:"filePath,omitempty"` // Path to saved file if requested
	ProcessingTime     int64    `json:"processingTime"`     // Milliseconds
	TokensUsed         int      `json:"tokensUsed,omitempty"`
	ModelUsed          string   `json:"modelUsed"`
	ProviderUsed       string   `json:"providerUsed"`
	Error              string   `json:"error,omitempty"`
	ValidationWarnings []string `json:"validationWarnings,omitempty"`
}

// ProcessAIImport handles the AI-powered import conversion
func ProcessAIImport(ctx *vbeam.Context, req ProcessAIImportRequest) (resp ProcessAIImportResponse, err error) {
	startTime := time.Now()

	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if strings.TrimSpace(req.UnstructuredText) == "" {
		resp.Error = "No text provided for AI processing"
		return
	}

	if req.PersonId == 0 {
		resp.Error = "Person ID is required for AI import"
		return
	}

	// Get the person and verify family ownership
	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 {
		resp.Error = "Person not found"
		return
	}

	if person.FamilyId != user.FamilyId {
		resp.Error = "You don't have permission to import data for this person"
		return
	}

	// Validate AI configuration
	if err = ValidateAIConfiguration(); err != nil {
		resp.Error = fmt.Sprintf("AI not configured: %v", err)
		return
	}

	// Get model (use environment override or default)
	modelName := os.Getenv("AI_MODEL")
	if modelName == "" {
		modelName = GetDefaultAIModel()
	}

	// Format person context for the AI
	personContext := formatPersonContext(person)

	// Get default prompt with person context and current date
	currentDate := time.Now().Format("2006-01-02")
	prompt := GetDefaultPrompt(personContext, currentDate)

	// Log the AI import attempt
	LogInfo("IMPORT", "AI import processing started", map[string]interface{}{
		"userId":       user.Id,
		"familyId":     user.FamilyId,
		"model":        modelName,
		"textLength":   len(req.UnstructuredText),
		"generateFile": req.GenerateFile,
	})

	// Process with AI
	aiRequest := AIConversionRequest{
		UnstructuredText: req.UnstructuredText,
		Model:            modelName,
		SystemPrompt:     prompt,
		UserID:           user.Id,
		FamilyID:         user.FamilyId,
	}

	conversionResult, err := ConvertToJSON(aiRequest)
	if err != nil {
		resp.Error = fmt.Sprintf("AI conversion failed: %v", err)
		LogErrorSimple("IMPORT", "AI conversion failed", err, map[string]interface{}{
			"model": modelName,
		})
		return
	}

	// Log the raw AI response for debugging
	LogInfo("IMPORT", "AI response received", map[string]interface{}{
		"userId":       user.Id,
		"familyId":     user.FamilyId,
		"model":        modelName,
		"responseLen":  len(conversionResult.GeneratedJSON),
		"tokensUsed":   conversionResult.TokensUsed,
		"responseTime": conversionResult.ResponseTime,
	})

	// Validate the generated JSON structure
	var aiImportData AIImportDataStructure
	if err = json.Unmarshal([]byte(conversionResult.GeneratedJSON), &aiImportData); err != nil {
		// Get a preview of the response for debugging
		preview := conversionResult.GeneratedJSON
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}

		resp.Error = "AI generated invalid JSON structure"
		resp.ValidationWarnings = append(resp.ValidationWarnings, fmt.Sprintf("JSON parse error: %v", err))
		resp.ValidationWarnings = append(resp.ValidationWarnings, fmt.Sprintf("Response preview: %s", preview))

		LogErrorSimple("IMPORT", "Failed to parse AI JSON response", err, map[string]interface{}{
			"model":       modelName,
			"responseLen": len(conversionResult.GeneratedJSON),
			"preview":     preview,
		})
		return
	}

	// Validate person ID matches
	if aiImportData.PersonId != req.PersonId {
		resp.ValidationWarnings = append(resp.ValidationWarnings,
			fmt.Sprintf("Warning: AI returned PersonId %d, expected %d", aiImportData.PersonId, req.PersonId))
	}

	// Basic data validation
	if len(aiImportData.Heights) == 0 && len(aiImportData.Weights) == 0 && len(aiImportData.Milestones) == 0 {
		resp.ValidationWarnings = append(resp.ValidationWarnings, "No data extracted from text")
	}

	// Convert AIImportDataStructure to full ImportDataStructure format
	// This allows the existing import flow to work without modification
	fullImportData := convertToImportDataStructure(person, aiImportData)

	// Re-serialize to JSON for the import flow
	fullJSON, err := json.MarshalIndent(fullImportData, "", "  ")
	if err != nil {
		resp.Error = "Failed to convert AI data to import format"
		return
	}

	// Generate file if requested
	if req.GenerateFile {
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("ai_import_%s_%d.json", timestamp, user.Id)

		// Create temp directory if it doesn't exist
		tempDir := filepath.Join(os.TempDir(), "family_portal", "ai_imports")
		if err = os.MkdirAll(tempDir, 0755); err != nil {
			resp.ValidationWarnings = append(resp.ValidationWarnings, "Could not create temp directory for file")
		} else {
			filePath := filepath.Join(tempDir, filename)

			if err = os.WriteFile(filePath, fullJSON, 0644); err != nil {
				resp.ValidationWarnings = append(resp.ValidationWarnings, "Could not save generated file")
			} else {
				resp.FilePath = filePath
			}
		}
	}

	// Prepare response
	resp.Success = true
	resp.GeneratedJSON = string(fullJSON)
	resp.ProcessingTime = time.Since(startTime).Milliseconds()
	resp.TokensUsed = conversionResult.TokensUsed
	resp.ModelUsed = modelName
	resp.ProviderUsed = "gemini"

	// Log successful processing
	LogInfo("IMPORT", "AI import processing completed", map[string]interface{}{
		"userId":          user.Id,
		"familyId":        user.FamilyId,
		"personId":        req.PersonId,
		"model":           modelName,
		"processingTime":  resp.ProcessingTime,
		"tokensUsed":      resp.TokensUsed,
		"heightsCount":    len(aiImportData.Heights),
		"weightsCount":    len(aiImportData.Weights),
		"milestonesCount": len(aiImportData.Milestones),
		"fileSaved":       req.GenerateFile,
	})

	return
}

type ListAIModelsRequest struct{}

type ListAIModelsResponse struct {
	Models []string `json:"models"`
	Error  string   `json:"error,omitempty"`
}

// ListAIModels returns the list of available AI models
func ListAIModels(ctx *vbeam.Context, req ListAIModelsRequest) (resp ListAIModelsResponse, err error) {
	// Check authentication
	_, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	models, err := ListAvailableModels()
	if err != nil {
		resp.Error = fmt.Sprintf("Failed to list models: %v", err)
		return
	}

	resp.Models = models
	return
}

// Helper function to read text files
func ReadTextFile(reader io.Reader, maxSize int64) (string, error) {
	// Limit file size to prevent abuse
	limitedReader := io.LimitReader(reader, maxSize)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// formatPersonContext formats a single person's information into a context string for the AI
func formatPersonContext(person Person) string {
	return fmt.Sprintf(`PERSON CONTEXT:
- Person ID: %d
- Name: "%s"
- Birthday: %s
- Age: %s
- Type: %s (%d)
- Gender: %s (%d)
- Family ID: %d`,
		person.Id,
		person.Name,
		person.Birthday.Format(time.RFC3339),
		person.Age,
		formatPersonType(person.Type),
		person.Type,
		formatGender(person.Gender),
		person.Gender,
		person.FamilyId,
	)
}

// formatPersonType returns human-readable person type
func formatPersonType(t PersonType) string {
	if t == Parent {
		return "Parent"
	}
	return "Child"
}

// formatGender returns human-readable gender
func formatGender(g GenderType) string {
	switch g {
	case Male:
		return "Male"
	case Female:
		return "Female"
	default:
		return "Unknown"
	}
}

// convertToImportDataStructure converts AI import format to full import format
func convertToImportDataStructure(person Person, aiData AIImportDataStructure) ImportDataStructure {
	// Create ImportPerson from existing person
	importPerson := ImportPerson{
		Id:       person.Id,
		FamilyId: person.FamilyId,
		Type:     int(person.Type),
		Gender:   int(person.Gender),
		Name:     person.Name,
		Birthday: person.Birthday,
		Age:      person.Age,
		ImageId:  person.ProfilePhotoId,
	}

	// Build full import structure
	return ImportDataStructure{
		People:          []ImportPerson{importPerson},
		Heights:         aiData.Heights,
		Weights:         aiData.Weights,
		Milestones:      aiData.Milestones,
		ExportDate:      time.Now(),
		TotalHeights:    aiData.TotalHeights,
		TotalWeights:    aiData.TotalWeights,
		TotalPeople:     1,
		TotalMilestones: aiData.TotalMilestones,
	}
}

// Constants for AI processing
const (
	MaxTextSize = 1024 * 1024     // 1MB max for unstructured text
	MaxFileSize = 5 * 1024 * 1024 // 5MB max for uploaded files
)
