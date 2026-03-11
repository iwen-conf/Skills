package calc

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}

// Clamp restricts v to the range [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
