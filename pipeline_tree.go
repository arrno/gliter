package gliter

import (
	"fmt"
)

// PLNode is a representation of a pipeline node for logging/insight-tracking.
type PLNode[T any] struct {
	id       string
	count    uint
	val      T
	children []*PLNode[T]
}

func NewPLNode[T any]() *PLNode[T] {
	return new(PLNode[T])
}

func NewPLNodeAs[T any](id string, val T) *PLNode[T] {
	return &PLNode[T]{
		id:  id,
		val: val,
	}
}

func (n *PLNode[T]) Print() {
	fmt.Printf("| %s | +%d | --> %v\n", n.id, n.count, n.val)
}

func (n *PLNode[T]) State() (string, uint, T) {
	return n.id, n.count, n.val
}

func (n *PLNode[T]) StateArr() []string {
	return []string{n.id, fmt.Sprintf("%d", n.count), fmt.Sprintf("%v", n.val)}
}

func (n *PLNode[T]) Count() PLNodeCount {
	return PLNodeCount{
		NodeID: n.id,
		Count:  int(n.count),
	}
}

// PrintFullBF will print the full PLNode tree breadth-first to stdout.
func (n *PLNode[T]) PrintFullBF() {
	q := List[*PLNode[T]]()
	results := [][]string{
		{"node", "count", "value"},
	}
	dimensions := []int{len(results[0][0]), len(results[0][1]), len(results[0][2])}
	var collect func()
	collect = func() {
		if q.Len() == 0 {
			return
		}
		next := q.FPop()
		state := next.StateArr()
		for i := range len(dimensions) {
			dimensions[i] = min(max(dimensions[i], len(state[i])), MAX_PAD)
		}
		results = append(results, state)
		for _, child := range next.children {
			q.Push(child)
		}
		collect()
	}
	q.Push(n)
	collect()
	printSep := func() {
		fmt.Printf(
			"+%s+%s+%s+\n",
			sep(dimensions[0]+2),
			sep(dimensions[1]+2),
			sep(dimensions[2]+2),
		)
	}
	printSep()
	for i, result := range results {
		fmt.Printf(
			"| %s | %s | %s |\n",
			pad(result[0], dimensions[0]),
			pad(result[1], dimensions[1]),
			pad(result[2], dimensions[2]),
		)
		if i == 0 {
			printSep()
		}
	}
	printSep()
}

func (n *PLNode[T]) CollectCount() []PLNodeCount {
	q := List[*PLNode[T]]()
	results := []PLNodeCount{}
	var collect func()
	collect = func() {
		if q.Len() == 0 {
			return
		}
		next := q.FPop()
		results = append(results, next.Count())
		for _, child := range next.children {
			q.Push(child)
		}
		collect()
	}
	q.Push(n)
	collect()
	return results
}

func (n *PLNode[T]) Set(val T) {
	n.val = val
}

func (n *PLNode[T]) Inc() {
	n.count++
}

func (n *PLNode[T]) IncAs(val T) {
	n.val = val
	n.count++
}

func (n *PLNode[T]) IncAsBatch(val []T) {
	if len(val) > 0 {
		n.val = val[len(val)-1]
	}
	n.count++
}

func (n *PLNode[T]) Spawn() *PLNode[T] {
	child := NewPLNode[T]()
	n.children = append(n.children, child)
	return child
}

func (n *PLNode[T]) SpawnAs(child *PLNode[T]) {
	n.children = append(n.children, child)
}

type PLNodeCount struct {
	NodeID string
	Count  int
}
