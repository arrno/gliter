package gliter

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunk(t *testing.T) {
	data := make([]int, 138)
	for i := range 138 {
		data[i] = i
	}
	chunks := ChunkBy(data, 20)
	expected := make([][]int, 7)
	idx := -1
	for i := range 7 {
		if i == 6 {
			for _ = range 18 {
				idx++
				expected[i] = append(expected[i], idx)
			}
		} else {
			for _ = range 20 {
				idx++
				expected[i] = append(expected[i], idx)
			}
		}
	}
	assert.True(t, reflect.DeepEqual(expected, chunks))

	flat := Flatten(chunks)
	assert.True(t, reflect.DeepEqual(data, flat))
}
