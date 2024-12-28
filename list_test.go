package gliter

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Thing struct {
	Num  int
	Char rune
}

func TestList(t *testing.T) {
	list := List(0, 1, 2, 3, 4)
	list.Pop()   // removes/returns `4`
	list.Push(8) // appends `8`
	assert.True(t, reflect.DeepEqual([]int{0, 1, 2, 3, 8}, list.items))

	value := List(0, 1, 2, 3, 4).
		Filter(func(i int) bool {
			return i%2 == 0
		}). // []int{0, 2, 4}
		Map(func(val int) int {
			return val * 2
		}). // []int{0, 4, 8}
		Reduce(func(acc *int, val int) {
			*acc += val
		}) // 12
	assert.Equal(t, 12, *value)

	item, ok := List(Thing{1, 'a'}, Thing{2, 'b'}, Thing{3, 'c'}).Find(func(val Thing) bool {
		return val.Num > 1
	})
	assert.True(t, ok)
	assert.True(t, reflect.DeepEqual(Thing{2, 'b'}, item))

	_, ok = List(Thing{1, 'a'}, Thing{2, 'b'}, Thing{3, 'c'}).Find(func(val Thing) bool {
		return val.Num == 7
	})
	assert.False(t, ok)

	assert.Equal(t, 3, List(1, 2, 3, 4, 5).At(2))

	assert.True(t, reflect.DeepEqual(
		List(1, 2, 3, 4, 5).Reverse().Unwrap(),
		[]int{5, 4, 3, 2, 1}))

	assert.True(t, reflect.DeepEqual(
		List(1, 2, 3, 4, 5).Delete(1).Unwrap(),
		[]int{1, 3, 4, 5}))

	assert.True(t, reflect.DeepEqual(
		List(1, 2, 3, 4, 5).
			Insert(1, 7).
			Insert(0, 0).
			Unwrap(),
		[]int{0, 1, 7, 2, 3, 4, 5}))

	newList := Map(*List(1, 2, 3), func(i int) Thing {
		return Thing{
			Num:  i,
			Char: 'a',
		}
	})
	assert.True(t, reflect.DeepEqual([]Thing{{1, 'a'}, {2, 'a'}, {3, 'a'}}, newList.Unwrap()))
}
