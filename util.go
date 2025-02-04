package gliter

import "strings"

const MAX_PAD int = 20

func pad(val string, dimension int) string {
	if len(val) == dimension {
		return val
	} else if len(val) > dimension {
		return string([]rune(val)[0:dimension-2]) + ".."
	}
	return val + strings.Repeat(" ", dimension-len(val))
}

func sep(dimension int) string {
	return strings.Repeat("-", dimension)
}

func ChunkBy[T any](items []T, by int) [][]T {
	if by >= len(items) {
		return [][]T{items}
	} else if by < 1 {
		return nil
	}
	chunks := make([][]T, 0, int(len(items)/by))
	// chunks = append(chunks, make([]T, 0, by))
	chunkIndex := -1

	for i, item := range items {
		if i%by == 0 {
			// new chunk
			chunks = append(chunks, make([]T, 0, by))
			chunkIndex++
		}
		chunks[chunkIndex] = append(chunks[chunkIndex], item)
	}
	return chunks
}

func Flatten[T any](chunks [][]T) []T {
	if len(chunks) == 0 {
		return chunks[0]
	}
	results := make([]T, 0, len(chunks)*len(chunks[0]))
	for _, chunk := range chunks {
		results = append(results, chunk...)
	}
	return results
}
