//go:build visionanalysis

package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"family/vision"

	"github.com/disintegration/imaging"
)

type benchPrompt struct {
	Label  string
	Phrase string
}

func readPrompts(path string) ([]benchPrompt, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var prompts []benchPrompt
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		label, phrase, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("prompt line %q has no label", line)
		}
		prompts = append(prompts, benchPrompt{strings.TrimSpace(label), strings.TrimSpace(phrase)})
	}
	return prompts, scanner.Err()
}

func procStatus(field string) string {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return "?"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, field+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, field+":"))
		}
	}
	return "?"
}

func runBench(dir, promptsPath string) error {
	prompts, err := readPrompts(promptsPath)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	fmt.Printf("after load: rss %s\n", procStatus("VmRSS"))

	type result struct {
		name      string
		embedding []float32
	}
	var results []result
	var total time.Duration
	var slowest time.Duration
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || (ext != ".jpg" && ext != ".jpeg" && ext != ".png") {
			continue
		}
		img, err := imaging.Open(filepath.Join(dir, e.Name()), imaging.AutoOrientation(true))
		if err != nil {
			fmt.Printf("skip %s: %v\n", e.Name(), err)
			continue
		}
		start := time.Now()
		embedding, err := models.EmbedImage(img)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		elapsed := time.Since(start)
		total += elapsed
		slowest = max(slowest, elapsed)
		results = append(results, result{e.Name(), embedding})
	}
	if len(results) == 0 {
		return fmt.Errorf("no images in %s", dir)
	}
	fmt.Printf("images: %d, mean %v, slowest %v (first includes warm-up)\n",
		len(results), (total / time.Duration(len(results))).Round(time.Millisecond), slowest.Round(time.Millisecond))

	sentences := []string{"First steps", "Started walking", "took a few steps today", "First word: mama", "Lost first tooth"}
	start := time.Now()
	sentenceVecs, err := models.EmbedSentences(sentences)
	if err != nil {
		return err
	}
	fmt.Printf("sentence batch of %d: %v\n", len(sentences), time.Since(start).Round(time.Millisecond))
	for i := 1; i < len(sentences); i++ {
		fmt.Printf("  %q ~ %q: %.3f\n", sentences[0], sentences[i], vision.Dot(sentenceVecs[0], sentenceVecs[i]))
	}

	if len(prompts) > 0 {
		phrases := make([]string, len(prompts))
		for i, p := range prompts {
			phrases[i] = p.Phrase
		}
		start := time.Now()
		textVecs, err := models.EmbedClipTexts(phrases)
		if err != nil {
			return err
		}
		fmt.Printf("clip text batch of %d: %v\n\n", len(phrases), time.Since(start).Round(time.Millisecond))

		for _, r := range results {
			type scored struct {
				label string
				cos   float64
				prob  float64
			}
			scores := make([]scored, len(prompts))
			var denom float64
			for i, p := range prompts {
				cos := float64(vision.Dot(r.embedding, textVecs[i]))
				scores[i] = scored{p.Label, cos, math.Exp(100 * cos)}
				denom += scores[i].prob
			}
			sort.Slice(scores, func(a, b int) bool { return scores[a].cos > scores[b].cos })
			fmt.Printf("%-28s", r.name)
			for _, s := range scores[:min(3, len(scores))] {
				fmt.Printf("  %s %.3f (%.0f%%)", s.label, s.cos, 100*s.prob/denom)
			}
			fmt.Println()
		}
	}

	fmt.Printf("\npeak rss %s\n", procStatus("VmHWM"))
	return nil
}
