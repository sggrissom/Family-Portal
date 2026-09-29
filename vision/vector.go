package vision

import "math"

func Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * inv
	}
	return out
}

func Dot(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// MeanPool averages token vectors (seqLen x dim, row-major) over positions
// where mask is nonzero.
func MeanPool(hidden []float32, mask []int64, dim int) []float32 {
	out := make([]float32, dim)
	var count float32
	for t, m := range mask {
		if m == 0 {
			continue
		}
		row := hidden[t*dim : (t+1)*dim]
		for i, x := range row {
			out[i] += x
		}
		count++
	}
	if count > 0 {
		for i := range out {
			out[i] /= count
		}
	}
	return out
}
