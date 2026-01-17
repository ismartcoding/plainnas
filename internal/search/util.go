package search

func intersectSorted(a, b []uint64) []uint64 {
	i, j := 0, 0
	out := make([]uint64, 0, min(len(a), len(b)))
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			out = append(out, a[i])
			i++
			j++
			continue
		}
		if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	return out
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// unionSorted merges two sorted unique uint64 slices into a sorted unique union
func unionSorted(a, b []uint64) []uint64 {
	out := make([]uint64, 0, len(a)+len(b))
	i, j := 0, 0
	var last uint64
	hasLast := false
	for i < len(a) && j < len(b) {
		var v uint64
		if a[i] == b[j] {
			v = a[i]
			i++
			j++
		} else if a[i] < b[j] {
			v = a[i]
			i++
		} else {
			v = b[j]
			j++
		}
		if !hasLast || v != last {
			out = append(out, v)
			last = v
			hasLast = true
		}
	}
	for i < len(a) {
		v := a[i]
		i++
		if !hasLast || v != last {
			out = append(out, v)
			last = v
			hasLast = true
		}
	}
	for j < len(b) {
		v := b[j]
		j++
		if !hasLast || v != last {
			out = append(out, v)
			last = v
			hasLast = true
		}
	}
	return out
}
