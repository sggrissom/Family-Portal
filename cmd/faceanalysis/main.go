//go:build faceanalysis

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"

	face "github.com/Kagami/go-face"
)

var recognizer *face.Recognizer

type recognizeRequest struct {
	ImagePath string `json:"image_path"`
}

type recognizeResponse struct {
	Descriptors [][]float32 `json:"descriptors"`
}

type embedRequest struct {
	ImagePath string `json:"image_path"`
}

type embedResponse struct {
	Descriptor []float32 `json:"descriptor"`
}

func handleRecognize(w http.ResponseWriter, r *http.Request) {
	var req recognizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	faces, err := recognizer.RecognizeFile(req.ImagePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	descriptors := make([][]float32, len(faces))
	for i, f := range faces {
		desc := make([]float32, 128)
		copy(desc, f.Descriptor[:])
		descriptors[i] = desc
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recognizeResponse{Descriptors: descriptors})
}

func handleEmbed(w http.ResponseWriter, r *http.Request) {
	var req embedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	faces, err := recognizer.RecognizeFile(req.ImagePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var resp embedResponse
	if len(faces) > 0 {
		desc := make([]float32, 128)
		copy(desc, faces[0].Descriptor[:])
		resp.Descriptor = desc
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	socketPath := flag.String("socket", "/run/family/face.sock", "Unix socket path")
	modelsDir := flag.String("models", "", "Path to dlib models directory")
	flag.Parse()

	if *modelsDir == "" {
		log.Fatal("--models flag is required")
	}

	var err error
	recognizer, err = face.NewRecognizer(*modelsDir)
	if err != nil {
		log.Fatalf("Failed to initialize face recognizer: %v", err)
	}
	defer recognizer.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/recognize", handleRecognize)
	mux.HandleFunc("/embed", handleEmbed)

	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *socketPath, err)
	}
	defer listener.Close()

	log.Printf("family-face daemon listening on %s", *socketPath)
	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
