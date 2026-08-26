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

type ProcessAIImportRequest struct {
	PersonId         int    `json:"personId"`
	UnstructuredText string `json:"unstructuredText"`
	GenerateFile     bool   `json:"generateFile"`
}

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
	FilePath           string   `json:"filePath,omitempty"`
	ProcessingTime     int64    `json:"processingTime"`
	TokensUsed         int      `json:"tokensUsed,omitempty"`
	ModelUsed          string   `json:"modelUsed"`
	ProviderUsed       string   `json:"providerUsed"`
	Error              string   `json:"error,omitempty"`
	ValidationWarnings []string `json:"validationWarnings,omitempty"`
}

func ProcessAIImport(ctx *vbeam.Context, req ProcessAIImportRequest) (resp ProcessAIImportResponse, err error) {
	startTime := time.Now()

	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if strings.TrimSpace(req.UnstructuredText) == "" {
		resp.Error = "No text provided for AI processing"
		return
	}

	if req.PersonId == 0 {
		resp.Error = "Person ID is required for AI import"
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 {
		resp.Error = "Person not found"
		return
	}

	if !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessContribute) {
		resp.Error = "You don't have permission to import data for this person"
		return
	}

	if configErr := ValidateAIConfiguration(); configErr != nil {
		LogWarn("IMPORT", "AI import attempted without a configured provider", map[string]interface{}{
			"userId": user.Id,
			"error":  configErr.Error(),
		})
		resp.Error = "AI import is not available on this server. You can still add records by hand."
		return
	}

	modelName := os.Getenv("AI_MODEL")
	if modelName == "" {
		modelName = GetDefaultAIModel()
	}

	personContext := formatPersonContext(person)

	currentDate := time.Now().Format("2006-01-02")
	prompt := GetDefaultPrompt(personContext, currentDate)

	LogInfo("IMPORT", "AI import processing started", map[string]interface{}{
		"userId":       user.Id,
		"familyId":     person.FamilyId,
		"model":        modelName,
		"textLength":   len(req.UnstructuredText),
		"generateFile": req.GenerateFile,
	})

	aiRequest := AIConversionRequest{
		UnstructuredText: req.UnstructuredText,
		Model:            modelName,
		SystemPrompt:     prompt,
		UserID:           user.Id,
		FamilyID:         person.FamilyId,
	}

	conversionResult, conversionErr := ConvertToJSON(aiRequest)
	if conversionErr != nil {
		reference := NewRequestID()
		LogErrorSimple("IMPORT", "AI conversion failed", conversionErr, map[string]interface{}{
			"model":     modelName,
			"requestId": reference,
		})
		resp.Error = "The AI service could not be reached. Please try again in a few minutes, or add the records by hand. " +
			ReferencePrefix + reference
		return
	}

	LogInfo("IMPORT", "AI response received", map[string]interface{}{
		"userId":       user.Id,
		"familyId":     person.FamilyId,
		"model":        modelName,
		"responseLen":  len(conversionResult.GeneratedJSON),
		"tokensUsed":   conversionResult.TokensUsed,
		"responseTime": conversionResult.ResponseTime,
	})

	var aiImportData AIImportDataStructure
	if err = json.Unmarshal([]byte(conversionResult.GeneratedJSON), &aiImportData); err != nil {
		preview := conversionResult.GeneratedJSON
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}

		resp.Error = "The AI returned something this app could not read. Try rewording the text, or add the records by hand."
		resp.ValidationWarnings = append(resp.ValidationWarnings, fmt.Sprintf("JSON parse error: %v", err))
		resp.ValidationWarnings = append(resp.ValidationWarnings, fmt.Sprintf("Response preview: %s", preview))

		LogErrorSimple("IMPORT", "Failed to parse AI JSON response", err, map[string]interface{}{
			"model":       modelName,
			"responseLen": len(conversionResult.GeneratedJSON),
		})
		return
	}

	if aiImportData.PersonId != req.PersonId {
		resp.ValidationWarnings = append(resp.ValidationWarnings,
			fmt.Sprintf("Warning: AI returned PersonId %d, expected %d", aiImportData.PersonId, req.PersonId))
	}

	if len(aiImportData.Heights) == 0 && len(aiImportData.Weights) == 0 && len(aiImportData.Milestones) == 0 {
		resp.ValidationWarnings = append(resp.ValidationWarnings, "No data extracted from text")
	}

	fullImportData := convertToImportDataStructure(person, aiImportData)

	fullJSON, err := json.MarshalIndent(fullImportData, "", "  ")
	if err != nil {
		resp.Error = "Failed to convert AI data to import format"
		return
	}

	if req.GenerateFile {
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("ai_import_%s_%d.json", timestamp, user.Id)

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

	resp.Success = true
	resp.GeneratedJSON = string(fullJSON)
	resp.ProcessingTime = time.Since(startTime).Milliseconds()
	resp.TokensUsed = conversionResult.TokensUsed
	resp.ModelUsed = modelName
	resp.ProviderUsed = "gemini"

	LogInfo("IMPORT", "AI import processing completed", map[string]interface{}{
		"userId":          user.Id,
		"familyId":        person.FamilyId,
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

func ListAIModels(ctx *vbeam.Context, req ListAIModelsRequest) (resp ListAIModelsResponse, err error) {
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

func ReadTextFile(reader io.Reader, maxSize int64) (string, error) {
	limitedReader := io.LimitReader(reader, maxSize)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

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

func formatPersonType(t PersonType) string {
	if t == Parent {
		return "Parent"
	}
	return "Child"
}

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

func convertToImportDataStructure(person Person, aiData AIImportDataStructure) ImportDataStructure {
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

const (
	MaxTextSize = 1024 * 1024
	MaxFileSize = 5 * 1024 * 1024
)
