//go:build faceanalysis

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	face "github.com/Kagami/go-face"
)

var recognizer *face.Recognizer

type recognizeRequest struct {
	ImagePath string `json:"image_path"`
	ImageData []byte `json:"image_data,omitempty"`
}

type recognizedFace struct {
	Descriptor []float32 `json:"descriptor"`
	Rect       [4]int    `json:"rect"`
}

type recognizeResponse struct {
	Descriptors [][]float32      `json:"descriptors"`
	Faces       []recognizedFace `json:"faces"`
	Source      string           `json:"source"`
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
	var faces []face.Face
	var err error
	source := "path"
	if len(req.ImageData) > 0 {
		source = "data"
		faces, err = recognizer.Recognize(req.ImageData)
	} else {
		faces, err = recognizer.RecognizeFile(req.ImagePath)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := recognizeResponse{
		Descriptors: make([][]float32, len(faces)),
		Faces:       make([]recognizedFace, len(faces)),
		Source:      source,
	}
	for i, f := range faces {
		desc := make([]float32, 128)
		copy(desc, f.Descriptor[:])
		r := f.Rectangle
		resp.Descriptors[i] = desc
		resp.Faces[i] = recognizedFace{Descriptor: desc, Rect: [4]int{r.Min.X, r.Min.Y, r.Max.X, r.Max.Y}}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	socketPath := flag.String("socket", envOr("FACE_SOCKET", "/run/family/face.sock"), "Unix socket path")
	modelsDir := flag.String("models", envOr("FACE_MODELS", ""), "Path to dlib models directory")
	port := flag.String("port", envOr("PORT", ""), "TCP port for healthz (optional)")
	flag.Parse()

	if *modelsDir == "" {
		log.Fatal("--models flag or FACE_MODELS env var is required")
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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Serve the RPC endpoints over Unix socket
	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *socketPath, err)
	}
	defer listener.Close()

	log.Printf("family-face daemon listening on %s", *socketPath)

	// Optionally also serve healthz over TCP for deploy health checks
	if *port != "" {
		go func() {
			log.Printf("family-face healthz on :%s", *port)
			if err := http.ListenAndServe(":"+*port, mux); err != nil {
				log.Printf("TCP server error: %v", err)
			}
		}()
	}

	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Unix socket server error: %v", err)
	}
}
