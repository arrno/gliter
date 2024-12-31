package gliter

import "slices"

type Iter[T any] struct {
	items []T
}

func NewList[T any]() *Iter[T] {
	return &Iter[T]{}
}

func MakeList[T any](size uint) *Iter[T] {
	return &Iter[T]{
		items: make([]T, size),
	}
}

func List[T any](vals ...T) *Iter[T] {
	return &Iter[T]{
		items: vals,
	}
}

func SliceToList[T any](s []T) *Iter[T] {
	return &Iter[T]{
		items: s,
	}
}

func (l Iter[T]) Unwrap() []T {
	return l.items
}

func (l *Iter[T]) Iter() []T {
	return l.items
}

func (l *Iter[T]) Push(val T) {
	l.items = append(l.items, val)
}

func (l *Iter[T]) Pop() T {
	var val T
	if len(l.items) > 0 {
		val = l.items[len(l.items)-1]
		l.items = l.items[:len(l.items)-1]
	}
	return val
}

func (l *Iter[T]) FPop() T {
	var val T
	if len(l.items) > 0 {
		val = l.items[0]
		l.items = l.items[1:len(l.items)]
	}
	return val

}

func (l Iter[T]) Map(f func(val T) T) Iter[T] {
	for i, item := range l.items {
		l.items[i] = f(item)
	}
	return l
}

func (l Iter[T]) Filter(f func(val T) bool) *Iter[T] {
	filtered := []T{}
	for _, item := range l.items {
		if f(item) {
			filtered = append(filtered, item)
		}
	}
	return List(filtered...)
}

func (l Iter[T]) Find(f func(val T) bool) (T, bool) {
	for _, item := range l.items {
		if f(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

func (l Iter[T]) Reduce(f func(acc *T, val T)) *T {
	acc := new(T)
	for _, item := range l.items {
		f(acc, item)
	}
	return acc
}

func (l Iter[T]) Reverse() Iter[T] {
	for i, j := 0, len(l.items)-1; i < j; i, j = i+1, j-1 {
		l.items[i], l.items[j] = l.items[j], l.items[i]
	}
	return l
}

func (l Iter[T]) Len() int {
	return len(l.items)
}

func (l Iter[T]) At(index int) T {
	return l.items[index]
}

func (l Iter[T]) Delete(index int) Iter[T] {
	ret := make([]T, 0)
	ret = append(ret, l.items[:index]...)
	l.items = append(ret, l.items[index+1:]...)
	return l
}

func (l Iter[T]) Insert(index int, val T) Iter[T] {
	l.items = slices.Insert(l.items, index, val)
	return l
}

func (l Iter[T]) Slice(start, stop int) Iter[T] {
	l.items = l.items[start:stop]
	return l
}

func Map[T, R any](list Iter[T], f func(T) R) *Iter[R] {
	result := make([]R, len(list.items))
	for i, item := range list.items {
		result[i] = f(item)
	}
	return List(result...)
}
