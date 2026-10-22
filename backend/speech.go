package backend

import (
	"context"
	"family/cfg"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	speechModelFile = "ggml-small.en.bin"
	speechVADFile   = "ggml-silero-v6.2.0.bin"
	speechThreads   = 2
	audioTimeout    = 5 * time.Minute
)

type speechEngine struct {
	binary string
	model  string
	vad    string
}

var speech *speechEngine

func speechAvailable() bool { return speech != nil }

func speechModelName() string {
	return "whisper-" + strings.TrimSuffix(strings.TrimPrefix(speechModelFile, "ggml-"), ".bin")
}

// findSpeechEngine looks for whisper-cli and its model in dir. The binary may
// sit beside the model or under bin/, as a whisper.cpp build leaves it.
func findSpeechEngine(dir string) (*speechEngine, error) {
	if dir == "" {
		return nil, fmt.Errorf("no speech models directory configured")
	}
	e := &speechEngine{model: filepath.Join(dir, speechModelFile)}
	for _, candidate := range []string{filepath.Join(dir, "whisper-cli"), filepath.Join(dir, "bin", "whisper-cli")} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			e.binary = candidate
			break
		}
	}
	if e.binary == "" {
		return nil, fmt.Errorf("whisper-cli not found in %s", dir)
	}
	if _, err := os.Stat(e.model); err != nil {
		return nil, fmt.Errorf("speech model: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, speechVADFile)); err == nil {
		e.vad = filepath.Join(dir, speechVADFile)
	}
	return e, nil
}

func InitializeSpeech() {
	e, err := findSpeechEngine(cfg.SpeechModelsDir)
	if err != nil {
		if cfg.SpeechModelsDir != "" {
			LogWarn(LogCategoryWorker, "Video transcription disabled", map[string]interface{}{"error": err.Error()})
		}
		return
	}
	speech = e
	LogInfo(LogCategoryWorker, "Video transcription available", map[string]interface{}{"model": speechModelName(), "vad": e.vad != ""})
}

func speechTimeout(duration time.Duration) time.Duration {
	return 3*duration + time.Minute
}

// transcribe returns the cleaned transcript of source, or one with no speech.
func (e *speechEngine) transcribe(ctx context.Context, tools mediaTools, image Image, source, workDir string) (VideoTranscript, error) {
	t := VideoTranscript{
		ImageId: image.Id, FamilyId: image.FamilyId,
		Model: speechModelName(), Version: transcriptVersion,
		Status: TranscriptNoSpeech, CreatedAt: time.Now(),
	}
	if image.AudioCodec == "" {
		return t, nil
	}
	wav := filepath.Join(workDir, "audio.wav")
	if _, err := tools.run(ctx, audioTimeout, tools.ffmpeg, "-nostdin", "-v", "error", "-y",
		"-i", source, "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", wav); err != nil {
		return t, err
	}
	base := filepath.Join(workDir, "transcript")
	args := []string{"-m", e.model, "-f", wav, "-l", "en", "-t", strconv.Itoa(speechThreads),
		"-bs", "1", "-bo", "1", "-np", "-ojf", "-of", base}
	if e.vad != "" {
		args = append(args, "--vad", "-vm", e.vad)
	}
	duration := time.Duration(image.DurationMs) * time.Millisecond
	if _, err := tools.run(ctx, speechTimeout(duration), e.binary, args...); err != nil {
		return t, err
	}
	data, err := os.ReadFile(base + ".json")
	if err != nil {
		return t, err
	}
	language, segments, err := parseWhisperOutput(data)
	if err != nil {
		return t, err
	}
	t.Language = language
	if segments = cleanTranscript(segments); len(segments) > 0 {
		t.Status, t.Segments = TranscriptDone, segments
	}
	return t, nil
}
