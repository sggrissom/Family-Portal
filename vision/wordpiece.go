package vision

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const wordPieceMaxChars = 100

type WordPieceTokenizer struct {
	vocab  map[string]int64
	ClsId  int64
	SepId  int64
	UnkId  int64
	MaxLen int
}

func LoadWordPieceTokenizer(vocabPath string, maxLen int) (*WordPieceTokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	vocab := map[string]int64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		vocab[strings.TrimRight(scanner.Text(), "\r")] = int64(len(vocab))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	t := &WordPieceTokenizer{vocab: vocab, MaxLen: maxLen}
	var ok1, ok2, ok3 bool
	t.ClsId, ok1 = vocab["[CLS]"]
	t.SepId, ok2 = vocab["[SEP]"]
	t.UnkId, ok3 = vocab["[UNK]"]
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("vocab is missing BERT special tokens")
	}
	return t, nil
}

func isBertPunct(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r)
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) || (r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) || (r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x2F800 && r <= 0x2FA1F)
}

func basicTokenize(text string) []string {
	var cleaned strings.Builder
	for _, r := range text {
		switch {
		case r == 0 || r == 0xFFFD || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r'):
			continue
		case unicode.IsSpace(r):
			cleaned.WriteRune(' ')
		case isCJK(r):
			cleaned.WriteRune(' ')
			cleaned.WriteRune(r)
			cleaned.WriteRune(' ')
		default:
			cleaned.WriteRune(r)
		}
	}

	var tokens []string
	for _, word := range strings.Fields(cleaned.String()) {
		word = strings.ToLower(word)
		var stripped strings.Builder
		for _, r := range norm.NFD.String(word) {
			if !unicode.Is(unicode.Mn, r) {
				stripped.WriteRune(r)
			}
		}
		var current strings.Builder
		for _, r := range stripped.String() {
			if isBertPunct(r) {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, string(r))
				continue
			}
			current.WriteRune(r)
		}
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
		}
	}
	return tokens
}

func (t *WordPieceTokenizer) wordPiece(word string) []int64 {
	runes := []rune(word)
	if len(runes) > wordPieceMaxChars {
		return []int64{t.UnkId}
	}
	var ids []int64
	for start := 0; start < len(runes); {
		end := len(runes)
		found := int64(-1)
		for start < end {
			sub := string(runes[start:end])
			if start > 0 {
				sub = "##" + sub
			}
			if id, ok := t.vocab[sub]; ok {
				found = id
				break
			}
			end--
		}
		if found < 0 {
			return []int64{t.UnkId}
		}
		ids = append(ids, found)
		start = end
	}
	return ids
}

// Encode returns [CLS] + word pieces + [SEP], truncated to MaxLen.
func (t *WordPieceTokenizer) Encode(text string) []int64 {
	ids := []int64{t.ClsId}
	for _, word := range basicTokenize(text) {
		ids = append(ids, t.wordPiece(word)...)
	}
	if t.MaxLen > 0 && len(ids) > t.MaxLen-1 {
		ids = ids[:t.MaxLen-1]
	}
	return append(ids, t.SepId)
}
