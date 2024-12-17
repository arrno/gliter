# Glitter ✨
**Iter tools for Go Lang!** This package has two utilities:

- **Iter tools:** Light generic slice wrapper for alternative functional/chaining syntax similar to Rust/Typescript.
- **Async iter tools:** Simple utilities for wrapping regular functions in light async wrappers to do fan-out/fan-in and async-pipeline patterns.

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

Something missing? Open a PR. **Contributions welcome!**

## Async iter tools

**In parallel**

Run a series of functions in parallel and collect results **preserving order at no cost.**.

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
// Run all tasks at the same time and collect results/err
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

Orchestrate a series of functions into a branching async pipeline with the `Pipeline` type.

Example of regular functions including a generator:

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

Assemble the functions into a new async pipeline with automatic branching using `NewPipeline`, `Stage`, and `Run`:

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