package glitter

type List[T any] struct {
	items []T
}

func NewList[T any]() *List[T] {
	return &List[T]{}
}

func MakeList[T any](size uint) *List[T] {
	return &List[T]{
		items: make([]T, size),
	}
}

func IntoList[T any](slice []T) *List[T] {
	return &List[T]{
		items: slice,
	}
}

func (l *List[T]) Unwrap() []T {
	return l.items
}

func (l *List[T]) Push(val T) {
	l.items = append(l.items, val)
}

func (l *List[T]) Pop() T {
	var val T
	if len(l.items) > 0 {
		val = l.items[len(l.items)-1]
		l.items = l.items[:len(l.items)-1]
	}
	return val

}

func (l List[T]) Map(f func(val T) T) List[T] {
	for i, item := range l.items {
		l.items[i] = f(item)
	}
	return l
}

func (l List[T]) Filter(f func(val T) bool) *List[T] {
	filtered := []T{}
	for _, item := range l.items {
		if f(item) {
			filtered = append(filtered, item)
		}
	}
	return IntoList(filtered)
}

func (l List[T]) Find(f func(val T) bool) (T, bool) {
	for _, item := range l.items {
		if f(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

func (l List[T]) Reduce(f func(acc *T, val T)) *T {
	acc := new(T)
	for _, item := range l.items {
		f(acc, item)
	}
	return acc
}

func (l List[T]) Len() int {
	return len(l.items)
}

func Map[T, R any](list List[T], f func(T) R) *List[R] {
	result := make([]R, len(list.items))
	for i, item := range list.items {
		result[i] = f(item)
	}
	return IntoList(result)
}
