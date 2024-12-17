# Glitter ✨
**Iter tools for Go Lang**

## Iter tools

The `List` Type

```go
list := glitter.List(0, 1, 2, 3, 4)
list.Pop() // removes/returns `4`
list.Push(8) // appends `8`
```

Chaining functions on a list

```go
value := glitter.
    List(0, 1, 2, 3, 4).
    Filter(func(i int) bool { 
        return i%2 == 0 
    }). // []int{0, 2, 4}
    Map(func(val int) int {
        return val * 2
    }). // []int{0, 4, 8}
    Reduce(func(acc *int, val int) {
        *acc += val
    }) // 12
```

Also...
- `list.Find(func (i int) { return i%2 == 0 })`
- `list.Len()`
- `list.Reverse()`
- `list.At(i)`
- `list.Slice(start, stop)`
- `list.Delete(index)`
- `list.Insert("value", index)`

Unwrap back into slice via `list.Iter()` or `list.Unwrap()`

```go
list := glitter.List(0, 1, 2, 3, 4)
for _, item := range list.Iter() {
    fmt.Println(item)
}
```

Map to alt type via `Map(list, func(in) out)`

## Async iter tools

**In parallel**

Run a series of functions in parallel and collect results **preserving order at no cost!**.

```go
tasks := []func() (string, error){
    func() (string, error) {
        return "Hello", nil
    },
    func() (string, error) {
        return ",", nil
    },
    func() (string, error) {
        return "Async!", nil
    },
}

results, err := glitter.InParallel(tasks)
if err != nil {
    panic(err)
}

// Always prints "Hello, Async!"
for _, result := range results {
    fmt.Print(result)
}
```

**Pipelines**

Orchestrate a series of functions into an branching async pipeline with the `Pipeline` type.

Regular functions including a generator:

```go
func example_gen() func() (int, bool) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (int, bool) {
		index++
		if index == len(data) {
			return 0, false
		}
		return data[index], true
	}
}

func example_mid(i int) (int, error) {
	return i * 2, nil
}

func example_end(i int) (int, error) {
	return i * i, nil
}
```

Assemble into async pipeline with automatic branching:

```go
glitter.NewPipeline(example_gen()).
    Stage(
        []func(i int) (int, error){
            example_mid, // branch A
            example_mid, // branch B
        },
    ).
    Stage(
        []func(i int) (int, error){
            example_end,
        },
    ).
    Run()
```