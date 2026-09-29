//go:build visionanalysis

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"family/vision"

	"github.com/disintegration/imaging"
)

var models *vision.Models

type imageRequest struct {
	ImageData []byte `json:"image_data"`
}

type imageResponse struct {
	Model     string    `json:"model"`
	Embedding []float32 `json:"embedding"`
}

type textRequest struct {
	Model string   `json:"model"`
	Texts []string `json:"texts"`
}

type textResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

type infoResponse struct {
	ImageModel    string `json:"image_model"`
	SentenceModel string `json:"sentence_model"`
}

const maxTextsPerRequest = 256

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func handleImage(w http.ResponseWriter, r *http.Request) {
	var req imageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	img, err := imaging.Decode(bytes.NewReader(req.ImageData), imaging.AutoOrientation(true))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	embedding, err := models.EmbedImage(img)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, imageResponse{Model: vision.ClipModelId, Embedding: embedding})
}

func handleText(w http.ResponseWriter, r *http.Request) {
	var req textRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Texts) > maxTextsPerRequest {
		http.Error(w, "too many texts", http.StatusBadRequest)
		return
	}
	var embeddings [][]float32
	var err error
	var model string
	switch req.Model {
	case "clip":
		model = vision.ClipModelId
		embeddings, err = models.EmbedClipTexts(req.Texts)
	case "sentence":
		model = vision.SentenceModelId
		embeddings, err = models.EmbedSentences(req.Texts)
	default:
		http.Error(w, "model must be clip or sentence", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, textResponse{Model: model, Embeddings: embeddings})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	socketPath := flag.String("socket", envOr("VISION_SOCKET", "/run/family-vision/vision.sock"), "Unix socket path")
	modelsDir := flag.String("models", envOr("VISION_MODELS", ""), "Model directory")
	port := flag.String("port", envOr("PORT", ""), "TCP port for healthz (optional)")
	threads := flag.Int("threads", 0, "Intra-op threads (default VISION_THREADS or 2)")
	benchDir := flag.String("bench", "", "Embed every image in this directory, report timings, and exit")
	prompts := flag.String("prompts", "", "With -bench: file of `label: phrase` lines to score each image against")
	visualFile := flag.String("visual", envOr("VISION_VISUAL", ""), "CLIP visual model file under clip/")
	textFile := flag.String("textual", envOr("VISION_TEXTUAL", ""), "CLIP text model file under clip/")
	minilmFile := flag.String("minilm", envOr("VISION_MINILM", ""), "Sentence model file under minilm/")
	flag.Parse()

	if *modelsDir == "" {
		log.Fatal("--models flag or VISION_MODELS env var is required")
	}
	if *threads == 0 {
		*threads, _ = strconv.Atoi(envOr("VISION_THREADS", "2"))
	}

	var err error
	models, err = vision.LoadModels(vision.ModelOptions{
		Dir:        *modelsDir,
		LibPath:    os.Getenv("ORT_LIB"),
		Threads:    *threads,
		VisualFile: *visualFile,
		TextFile:   *textFile,
		MiniLMFile: *minilmFile,
	})
	if err != nil {
		log.Fatalf("Failed to load models: %v", err)
	}
	defer models.Close()

	if *benchDir != "" {
		if err := runBench(*benchDir, *prompts); err != nil {
			log.Fatal(err)
		}
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /embed/image", handleImage)
	mux.HandleFunc("POST /embed/text", handleText)
	mux.HandleFunc("GET /info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, infoResponse{ImageModel: vision.ClipModelId, SentenceModel: vision.SentenceModelId})
	})
	healthz := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	mux.HandleFunc("/healthz", healthz)

	os.Remove(*socketPath)
	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *socketPath, err)
	}
	defer listener.Close()
	log.Printf("family-vision daemon listening on %s (%d threads)", *socketPath, *threads)

	if *port != "" {
		go func() {
			log.Printf("family-vision healthz on :%s", *port)
			health := http.NewServeMux()
			health.HandleFunc("/healthz", healthz)
			if err := http.ListenAndServe("127.0.0.1:"+*port, health); err != nil {
				log.Printf("TCP server error: %v", err)
			}
		}()
	}

	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Unix socket server error: %v", err)
	}
}
