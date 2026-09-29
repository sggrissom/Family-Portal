package vision

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"sync"
)

const (
	clipStartToken = "<|startoftext|>"
	clipEndToken   = "<|endoftext|>"
	ClipContextLen = 77
)

var clipPattern = regexp.MustCompile(`(?i)<\|startoftext\|>|<\|endoftext\|>|'s|'t|'re|'ve|'m|'ll|'d|\p{L}+|\p{N}|[^\s\p{L}\p{N}]+`)

var whitespaceRun = regexp.MustCompile(`\s+`)

type ClipTokenizer struct {
	encoder    map[string]int64
	bpeRanks   map[[2]string]int
	byteToRune [256]rune
	cacheMu    sync.Mutex
	cache      map[string][]string
	StartId    int64
	EndId      int64
}

func LoadClipTokenizer(vocabPath, mergesPath string) (*ClipTokenizer, error) {
	vocabData, err := os.ReadFile(vocabPath)
	if err != nil {
		return nil, err
	}
	encoder := map[string]int64{}
	if err := json.Unmarshal(vocabData, &encoder); err != nil {
		return nil, fmt.Errorf("parse %s: %w", vocabPath, err)
	}

	f, err := os.Open(mergesPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ranks := map[[2]string]int{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#version") || line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			continue
		}
		ranks[[2]string{parts[0], parts[1]}] = len(ranks)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	t := &ClipTokenizer{
		encoder:    encoder,
		bpeRanks:   ranks,
		byteToRune: bytesToUnicode(),
		cache:      map[string][]string{},
	}
	var ok1, ok2 bool
	t.StartId, ok1 = encoder[clipStartToken]
	t.EndId, ok2 = encoder[clipEndToken]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("vocab is missing CLIP special tokens")
	}
	return t, nil
}

func bytesToUnicode() [256]rune {
	var table [256]rune
	var assigned [256]bool
	for b := '!'; b <= '~'; b++ {
		table[b], assigned[b] = b, true
	}
	for b := '¡'; b <= '¬'; b++ {
		table[b], assigned[b] = b, true
	}
	for b := '®'; b <= 'ÿ'; b++ {
		table[b], assigned[b] = b, true
	}
	n := 0
	for b := 0; b < 256; b++ {
		if !assigned[b] {
			table[b] = rune(256 + n)
			n++
		}
	}
	return table
}

func (t *ClipTokenizer) bpe(token string) []string {
	t.cacheMu.Lock()
	cached, ok := t.cache[token]
	t.cacheMu.Unlock()
	if ok {
		return cached
	}
	var word []string
	for _, r := range token {
		word = append(word, string(r))
	}
	if len(word) == 0 {
		return nil
	}
	word[len(word)-1] += "</w>"

	for len(word) > 1 {
		bestRank, bestIdx := -1, -1
		for i := 0; i < len(word)-1; i++ {
			if rank, ok := t.bpeRanks[[2]string{word[i], word[i+1]}]; ok && (bestRank < 0 || rank < bestRank) {
				bestRank, bestIdx = rank, i
			}
		}
		if bestIdx < 0 {
			break
		}
		first, second := word[bestIdx], word[bestIdx+1]
		merged := make([]string, 0, len(word))
		for i := 0; i < len(word); i++ {
			if i < len(word)-1 && word[i] == first && word[i+1] == second {
				merged = append(merged, first+second)
				i++
			} else {
				merged = append(merged, word[i])
			}
		}
		word = merged
	}
	t.cacheMu.Lock()
	t.cache[token] = word
	t.cacheMu.Unlock()
	return word
}

func cleanClipText(text string) string {
	text = html.UnescapeString(html.UnescapeString(text))
	text = whitespaceRun.ReplaceAllString(text, " ")
	return strings.ToLower(strings.TrimSpace(text))
}

// Encode returns start + BPE ids + end, truncated to ClipContextLen.
func (t *ClipTokenizer) Encode(text string) []int64 {
	ids := []int64{t.StartId}
	for _, piece := range clipPattern.FindAllString(cleanClipText(text), -1) {
		if piece == clipStartToken || piece == clipEndToken {
			ids = append(ids, t.encoder[piece])
			continue
		}
		var mapped strings.Builder
		for _, b := range []byte(piece) {
			mapped.WriteRune(t.byteToRune[b])
		}
		for _, sub := range t.bpe(mapped.String()) {
			if id, ok := t.encoder[sub]; ok {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > ClipContextLen-1 {
		ids = ids[:ClipContextLen-1]
	}
	return append(ids, t.EndId)
}
