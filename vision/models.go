//go:build visionanalysis

package vision

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	ClipModelId     = "clip-vit-b32"
	SentenceModelId = "all-minilm-l6-v2"
	sentenceMaxLen  = 256
	sentenceDim     = 384
)

type Models struct {
	mu        sync.Mutex
	visual    *ort.DynamicAdvancedSession
	textual   *ort.DynamicAdvancedSession
	sentence  *ort.DynamicAdvancedSession
	clipTok   *ClipTokenizer
	wordPiece *WordPieceTokenizer
}

type ModelOptions struct {
	Dir        string
	LibPath    string
	Threads    int
	VisualFile string
	TextFile   string
	MiniLMFile string
}

func (o ModelOptions) path(parts ...string) string {
	return filepath.Join(append([]string{o.Dir}, parts...)...)
}

func LoadModels(opts ModelOptions) (*Models, error) {
	if opts.LibPath == "" {
		opts.LibPath = opts.path("libonnxruntime.so")
	}
	if opts.VisualFile == "" {
		opts.VisualFile = "visual.onnx"
	}
	if opts.TextFile == "" {
		opts.TextFile = "textual.onnx"
	}
	if opts.MiniLMFile == "" {
		opts.MiniLMFile = "model.onnx"
	}
	if _, err := os.Stat(opts.LibPath); err != nil {
		return nil, fmt.Errorf("onnxruntime library: %w", err)
	}
	ort.SetSharedLibraryPath(opts.LibPath)
	if err := ort.InitializeEnvironment(ort.WithLogLevelWarning()); err != nil {
		return nil, fmt.Errorf("initialize onnxruntime: %w", err)
	}

	sessionOpts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer sessionOpts.Destroy()
	if opts.Threads > 0 {
		if err := sessionOpts.SetIntraOpNumThreads(opts.Threads); err != nil {
			return nil, err
		}
		if err := sessionOpts.SetInterOpNumThreads(1); err != nil {
			return nil, err
		}
	}

	m := &Models{}
	if m.visual, err = ort.NewDynamicAdvancedSession(opts.path("clip", opts.VisualFile),
		[]string{"pixel_values"}, []string{"image_embeds"}, sessionOpts); err != nil {
		return nil, fmt.Errorf("load clip visual: %w", err)
	}
	if m.textual, err = ort.NewDynamicAdvancedSession(opts.path("clip", opts.TextFile),
		[]string{"input_ids"}, []string{"text_embeds"}, sessionOpts); err != nil {
		return nil, fmt.Errorf("load clip textual: %w", err)
	}
	if m.sentence, err = ort.NewDynamicAdvancedSession(opts.path("minilm", opts.MiniLMFile),
		[]string{"input_ids", "attention_mask", "token_type_ids"}, []string{"last_hidden_state"}, sessionOpts); err != nil {
		return nil, fmt.Errorf("load minilm: %w", err)
	}
	if m.clipTok, err = LoadClipTokenizer(opts.path("clip", "vocab.json"), opts.path("clip", "merges.txt")); err != nil {
		return nil, fmt.Errorf("load clip tokenizer: %w", err)
	}
	if m.wordPiece, err = LoadWordPieceTokenizer(opts.path("minilm", "vocab.txt"), sentenceMaxLen); err != nil {
		return nil, fmt.Errorf("load minilm tokenizer: %w", err)
	}
	return m, nil
}

func (m *Models) Close() {
	for _, s := range []*ort.DynamicAdvancedSession{m.visual, m.textual, m.sentence} {
		if s != nil {
			s.Destroy()
		}
	}
	ort.DestroyEnvironment()
}

func runSingleOutput(session *ort.DynamicAdvancedSession, inputs []ort.Value) ([]float32, ort.Shape, error) {
	outputs := []ort.Value{nil}
	if err := session.Run(inputs, outputs); err != nil {
		return nil, nil, err
	}
	defer outputs[0].Destroy()
	tensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, nil, fmt.Errorf("unexpected output type %T", outputs[0])
	}
	data := append([]float32(nil), tensor.GetData()...)
	return data, tensor.GetShape(), nil
}

func (m *Models) EmbedImage(img image.Image) ([]float32, error) {
	pixels := ClipPixels(img)
	input, err := ort.NewTensor(ort.NewShape(1, 3, ClipImageSize, ClipImageSize), pixels)
	if err != nil {
		return nil, err
	}
	defer input.Destroy()

	m.mu.Lock()
	defer m.mu.Unlock()
	data, _, err := runSingleOutput(m.visual, []ort.Value{input})
	if err != nil {
		return nil, err
	}
	return Normalize(data), nil
}

func padBatch(batch [][]int64, pad int64) ([]int64, []int64, int) {
	width := 0
	for _, ids := range batch {
		width = max(width, len(ids))
	}
	ids := make([]int64, 0, len(batch)*width)
	mask := make([]int64, 0, len(batch)*width)
	for _, row := range batch {
		for i := 0; i < width; i++ {
			if i < len(row) {
				ids = append(ids, row[i])
				mask = append(mask, 1)
			} else {
				ids = append(ids, pad)
				mask = append(mask, 0)
			}
		}
	}
	return ids, mask, width
}

func (m *Models) EmbedClipTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	batch := make([][]int64, len(texts))
	for i, t := range texts {
		batch[i] = m.clipTok.Encode(t)
	}
	ids, _, width := padBatch(batch, m.clipTok.EndId)
	input, err := ort.NewTensor(ort.NewShape(int64(len(texts)), int64(width)), ids)
	if err != nil {
		return nil, err
	}
	defer input.Destroy()

	m.mu.Lock()
	defer m.mu.Unlock()
	data, shape, err := runSingleOutput(m.textual, []ort.Value{input})
	if err != nil {
		return nil, err
	}
	dim := int(shape[1])
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = Normalize(data[i*dim : (i+1)*dim])
	}
	return out, nil
}

func (m *Models) EmbedSentences(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	batch := make([][]int64, len(texts))
	for i, t := range texts {
		batch[i] = m.wordPiece.Encode(t)
	}
	ids, mask, width := padBatch(batch, 0)
	shape := ort.NewShape(int64(len(texts)), int64(width))
	idsT, err := ort.NewTensor(shape, ids)
	if err != nil {
		return nil, err
	}
	defer idsT.Destroy()
	maskT, err := ort.NewTensor(shape, mask)
	if err != nil {
		return nil, err
	}
	defer maskT.Destroy()
	typesT, err := ort.NewTensor(shape, make([]int64, len(ids)))
	if err != nil {
		return nil, err
	}
	defer typesT.Destroy()

	m.mu.Lock()
	defer m.mu.Unlock()
	data, _, err := runSingleOutput(m.sentence, []ort.Value{idsT, maskT, typesT})
	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	stride := width * sentenceDim
	for i := range out {
		out[i] = Normalize(MeanPool(data[i*stride:(i+1)*stride], mask[i*width:(i+1)*width], sentenceDim))
	}
	return out, nil
}
