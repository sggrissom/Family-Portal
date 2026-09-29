package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/disintegration/imaging"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const (
	clipModelId      = "clip-vit-b32"
	embeddingVersion = 1
	embedSourceSize  = 600
)

type PhotoEmbedding struct {
	PhotoId  int
	FamilyId int
	Model    string
	Version  int
	Vector   []float32
}

func PackPhotoEmbedding(self *PhotoEmbedding, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Model, buf)
	vpack.Int(&self.Version, buf)
	packFloat32Slice(&self.Vector, buf)
}

var PhotoEmbeddingBkt = vbolt.Bucket(&cfg.Info, "photo_embeddings", vpack.FInt, PackPhotoEmbedding)
var EmbeddingByFamilyIndex = vbolt.Index(&cfg.Info, "embedding_by_family", vpack.FInt, vpack.FInt)

func GetPhotoEmbedding(tx *vbolt.Tx, photoId int) (e PhotoEmbedding, ok bool) {
	ok = vbolt.Read(tx, PhotoEmbeddingBkt, photoId, &e)
	return
}

func familyEmbeddings(tx *vbolt.Tx, familyId int) (embeddings []PhotoEmbedding) {
	var ids []int
	vbolt.ReadTermTargets(tx, EmbeddingByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoEmbeddingBkt, ids, &embeddings)
	return
}

func writePhotoEmbeddingTx(tx *vbolt.Tx, e *PhotoEmbedding) {
	vbolt.Write(tx, PhotoEmbeddingBkt, e.PhotoId, e)
	vbolt.SetTargetSingleTerm(tx, EmbeddingByFamilyIndex, e.PhotoId, e.FamilyId)
	invalidateFamilyVectors(e.FamilyId)
}

func deletePhotoEmbeddingTx(tx *vbolt.Tx, photoId int) {
	if e, ok := GetPhotoEmbedding(tx, photoId); ok {
		invalidateFamilyVectors(e.FamilyId)
	}
	vbolt.Delete(tx, PhotoEmbeddingBkt, photoId)
	vbolt.DeleteTargetTerms(tx, EmbeddingByFamilyIndex, photoId)
}

func embeddingOutdated(e PhotoEmbedding, found bool) bool {
	return !found || e.Model != clipModelId || e.Version < embeddingVersion || len(e.Vector) == 0
}

// errVisionUnavailable means the daemon could not be reached at all, as
// opposed to rejecting one request.
var errVisionUnavailable = errors.New("vision daemon unavailable")

type visionClient struct {
	http *http.Client
}

func newVisionClient(socket string) *visionClient {
	return &visionClient{http: &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
		},
	}}
}

func (c *visionClient) post(path string, req any, resp any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r, err := c.http.Post("http://vision"+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", errVisionUnavailable, err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(r.Body, 512))
		return fmt.Errorf("vision daemon returned %d: %s", r.StatusCode, strings.TrimSpace(string(msg)))
	}
	return json.NewDecoder(r.Body).Decode(resp)
}

func (c *visionClient) embedImage(data []byte) ([]float32, string, error) {
	var resp struct {
		Model     string    `json:"model"`
		Embedding []float32 `json:"embedding"`
	}
	err := c.post("/embed/image", map[string]any{"image_data": data}, &resp)
	return resp.Embedding, resp.Model, err
}

func (c *visionClient) embedTexts(model string, texts []string) ([][]float32, error) {
	var resp struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	err := c.post("/embed/text", map[string]any{"model": model, "texts": texts}, &resp)
	return resp.Embeddings, err
}

var (
	globalVisionClient *visionClient
	globalVisionWorker *backlogWorker[int]
	visionReachable    atomic.Bool
)

func noteVisionReachable(ok bool) {
	if visionReachable.Swap(ok) != ok {
		if ok {
			LogInfo(LogCategoryWorker, "Vision daemon reachable", map[string]interface{}{"socket": cfg.VisionAnalysisSocket})
		} else {
			LogWarn(LogCategoryWorker, "Vision daemon unreachable; embeddings paused", map[string]interface{}{"socket": cfg.VisionAnalysisSocket})
		}
	}
}

// VisionStatus is "off" when no socket is configured, otherwise whether the
// last call reached the daemon.
func VisionStatus() string {
	switch {
	case globalVisionClient == nil:
		return "off"
	case visionReachable.Load():
		return "up"
	default:
		return "down"
	}
}

func embeddingSourceBytes(img Image) ([]byte, error) {
	base := strings.TrimSuffix(filepath.Join(cfg.StaticDir, img.FilePath), filepath.Ext(img.FilePath))
	if data, err := os.ReadFile(base + "_medium.jpg"); err == nil {
		return data, nil
	}
	decoded, err := featureSourceImage(img)
	if err != nil {
		return nil, err
	}
	decoded = imaging.Fit(decoded, embedSourceSize, embedSourceSize, imaging.Lanczos)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, decoded, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func embedPhoto(db *vbolt.DB, client *visionClient, photoId int) error {
	var img Image
	var current PhotoEmbedding
	var found bool
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		img = GetImageById(tx, photoId)
		current, found = GetPhotoEmbedding(tx, photoId)
	})
	if img.Id == 0 || img.Status != 0 || !embeddingOutdated(current, found) {
		return nil
	}

	data, err := embeddingSourceBytes(img)
	if err != nil {
		log.Printf("[VISION] No readable image for photo %d: %v", photoId, err)
		return nil
	}
	vector, model, err := client.embedImage(data)
	if errors.Is(err, errVisionUnavailable) {
		noteVisionReachable(false)
		return errRetryLater
	}
	noteVisionReachable(true)
	if err != nil {
		log.Printf("[VISION] Embedding failed for photo %d: %v", photoId, err)
		return nil
	}
	if model != clipModelId {
		log.Printf("[VISION] Daemon returned model %q, want %q; skipping photo %d", model, clipModelId, photoId)
		return nil
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, photoId)
		if img.Id == 0 {
			return
		}
		writePhotoEmbeddingTx(tx, &PhotoEmbedding{
			PhotoId: img.Id, FamilyId: img.FamilyId,
			Model: clipModelId, Version: embeddingVersion, Vector: vector,
		})
		vbolt.TxCommit(tx)
	})
	invalidateFamilyVectors(img.FamilyId)
	return nil
}

func photosNeedingEmbedding(db *vbolt.DB) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, img Image) bool {
			if img.Status == 0 {
				e, found := GetPhotoEmbedding(tx, img.Id)
				if embeddingOutdated(e, found) {
					ids = append(ids, img.Id)
				}
			}
			return true
		})
	})
	if len(ids) > 0 {
		LogInfo(LogCategoryWorker, "Queued photos for image embedding", map[string]interface{}{"count": len(ids)})
	}
	return
}

func InitializeVisionWorker(db *vbolt.DB) {
	if cfg.VisionAnalysisSocket == "" {
		LogInfo(LogCategoryWorker, "Vision analysis not configured, skipping")
		return
	}
	if globalVisionWorker != nil {
		return
	}
	globalVisionClient = newVisionClient(cfg.VisionAnalysisSocket)
	client := globalVisionClient
	globalVisionWorker = newBacklogWorker("Vision worker", func(photoId int) error {
		if err := embedPhoto(db, client, photoId); err != nil {
			return err
		}
		return suggestTagsForPhoto(db, client, photoId)
	})
	globalVisionWorker.run(func() []int {
		ids := photosNeedingEmbedding(db)
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) { ids = append(ids, photosNeedingSuggestions(tx)...) })
		return ids
	})
	startMilestoneWorker(db, client)
	LogInfo(LogCategoryWorker, "Vision worker started", map[string]interface{}{"socket": cfg.VisionAnalysisSocket})
}

func QueuePhotoEmbedding(photoId int) {
	if globalVisionWorker != nil {
		globalVisionWorker.add(photoId)
	}
}

func VisionQueueLength() int {
	if globalVisionWorker == nil {
		return 0
	}
	return globalVisionWorker.length()
}

func StopVisionWorker(ctx context.Context) bool {
	if globalVisionWorker == nil {
		return true
	}
	return globalVisionWorker.stopWait(ctx)
}
