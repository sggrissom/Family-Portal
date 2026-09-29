package vision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// VISION_MODELS is a directory filled by scripts/fetch-vision-models.sh.
// testdata/reference_tokens.json holds ids from the Hugging Face tokenizers.
func modelsDir(t *testing.T) string {
	dir := os.Getenv("VISION_MODELS")
	if dir == "" {
		t.Skip("VISION_MODELS not set")
	}
	return dir
}

type referenceTokens struct {
	Text   string  `json:"text"`
	Clip   []int64 `json:"clip"`
	MiniLM []int64 `json:"minilm"`
}

func loadReference(t *testing.T) []referenceTokens {
	data, err := os.ReadFile("testdata/reference_tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var refs []referenceTokens
	if err := json.Unmarshal(data, &refs); err != nil {
		t.Fatal(err)
	}
	return refs
}

func TestClipTokenizerMatchesReference(t *testing.T) {
	dir := modelsDir(t)
	tok, err := LoadClipTokenizer(filepath.Join(dir, "clip/vocab.json"), filepath.Join(dir, "clip/merges.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range loadReference(t) {
		if got := tok.Encode(ref.Text); !reflect.DeepEqual(got, ref.Clip) {
			t.Errorf("%q:\n got  %v\n want %v", ref.Text, got, ref.Clip)
		}
	}
}

func TestWordPieceTokenizerMatchesReference(t *testing.T) {
	dir := modelsDir(t)
	tok, err := LoadWordPieceTokenizer(filepath.Join(dir, "minilm/vocab.txt"), 256)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range loadReference(t) {
		if got := tok.Encode(ref.Text); !reflect.DeepEqual(got, ref.MiniLM) {
			t.Errorf("%q:\n got  %v\n want %v", ref.Text, got, ref.MiniLM)
		}
	}
}

func TestBasicTokenize(t *testing.T) {
	got := basicTokenize("Café,  First STEPS!\t日本")
	want := []string{"cafe", ",", "first", "steps", "!", "日", "本"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBytesToUnicodeIsBijective(t *testing.T) {
	table := bytesToUnicode()
	seen := map[rune]bool{}
	for _, r := range table {
		if seen[r] {
			t.Fatalf("rune %q assigned twice", r)
		}
		seen[r] = true
	}
	if table[' '] != 'Ġ' {
		t.Errorf("space maps to %q, want Ġ", table[' '])
	}
}
