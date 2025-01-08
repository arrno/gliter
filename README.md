# GLiter ✨

**Go Lang iter tools!** This package has two utilities:

- **Async iter tools:** Simple utilities for wrapping regular functions in light async wrappers to do fan-out/fan-in and async-pipeline patterns.
- **Iter tools:** Light generic slice wrapper for alternative functional/chaining syntax similar to Rust/Typescript.

ʕ◔ϖ◔ʔ

## Async iter tools

### In parallel

AKA Fan-in/Fan-out

Run a series of functions in parallel and collect results **preserving order at no cost.**.

```go
tasks := []func() (string, error){
    func() (string, error) {
        return "Hello", nil
    },
    func() (string, error) {
        return ", ", nil
    },
    func() (string, error) {
        return "Async!", nil
    },
}
// Run all tasks at the same time and collect results/err
results, err := gliter.InParallel(tasks)
if err != nil {
    panic(err)
}

// Always prints "Hello, Async!"
for _, result := range results {
    fmt.Print(result)
}
```

### Pipelines

Orchestrate a series of functions into a branching async pipeline with the `Pipeline` type.

Example of regular functions including a generator:

```go
func exampleGen() func() (int, bool, error) {
    data := []int{1, 2, 3, 4, 5}
    index := -1
    return func() (int, bool, error) {
        index++
        if index == len(data) {
            return 0, false, nil
        }
        return data[index], true, nil
    }
}

func exampleMid(i int) (int, error) {
    return i * 2, nil
}

func exampleEnd(i int) (int, error) {
    return i * i, nil
}
```

Assemble the functions into a new async pipeline with automatic branching using `NewPipeline`, `Stage`, and `Run`:

```go
gliter.NewPipeline(exampleGen()).
    Stage(
        []func(i int) (int, error){
            exampleMid, // branch A
            exampleMid, // branch B
        },
    ).
    Stage(
        []func(i int) (int, error){
            exampleEnd, // Downstream of fork, exists in both branches
        },
    ).
    Run()
```

#### Behavior

Any time we choose to add multiple handlers in a single stage, we are forking the pipeline that many times. If for example we add two stages, each containing two functions, we will produce four output streams at the end of the pipeline.

> Although in the example, we forked into two branches with the same func `exampleMid`, in production, you will likely fork with unique functions that each represent a distinct mutation. If you are passing a pointer through the pipeline like a slice or map, you should clone the object within each transform function that alters it.

Data always flows downstream from generator through stages sequentially. When a fork occurs, all downstream stages are implicitly duplicated to exist in each stream.

There is no distinct end stage. Any side-effects/outputs like db writes or API posts should be handled inside a Stage function wherever appropriate.

#### Log

It may be helpful during testing to audit what is happening inside a pipeline.

To do so, optionally set pipeline logging via one of the following modes in `pipeline.Config(gliter.PLConfig{...})`:

- `LogAll` - Log emit and count
- `LogEmit` - Only emit
- `LogCount` - Only count
- `LogStep` - Interactive CLI stepper

```
Emit -> 4
Emit -> 16
...
Emit -> 100
+-------+-------+-------+
| node  | count | value |
+-------+-------+-------+
| GEN   | 5     | 5     |
| 0:0:0 | 5     | 10    |
| 0:0:1 | 5     | 10    |
| 1:0:0 | 5     | 100   |
| 1:1:0 | 5     | 100   |
+-------+-------+-------+
```

The node id shown in the table is constructed as stage-index:parent-index:func-index. For example, node id `4:1:2` would indicate the third function (idx 2) of the fifth stage (idx 4) branched from the second parent node (idx 1).

"The second parent node" in this context can also be called the func at index 1 of the fourth stage.

Throttle stages will not be logged but will increment the node id indexes.

#### Throttle

What if our end stage results in a high number of concurrent output streams that overwhelms a destination DB or API? Use the throttle stage to rein in concurrent streams like this:

```go
// With concurrency throttling
gliter.NewPipeline(exampleGen()).
    Stage(
        []func(i int) (int, error){
            exampleMid, // branch A
            exampleMid, // branch B
        },
    ).
    Stage(
        []func(i int) (int, error){
            exampleMid, // branches A.C, B.C
            exampleMid, // branches A.D, B.D
            exampleMid, // branches A.E, B.E
        },
    ).
    Throttle(2). // merge into branches X, Z
    Stage(
        []func(i int) (int, error){
            exampleEnd,
        },
    ).
    Run()
```

Here is a visual diagram of the pipeline the code produces:

![Alt text](./diag/small-chart.png?raw=true "Title")

### Misc

Other async helpers in this library include:

- Do your own throttling with `ThrottleBy`
- Do your own channel forking with `TeeBy`
- `ReadOrDone` + `WriteOrDone`
- `Any` - take one from any, consolidating "done" channels

## Iter tools

Synchronous iter tools that may be helpful.

The `List` Type

```go
list := gliter.List(0, 1, 2, 3, 4)
list.Pop() // removes/returns `4`
list.Push(8) // appends `8`
```

Chaining functions on a list

```go
value := gliter.
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
- `list.FPop()`
- `list.Slice(start, stop)`
- `list.Delete(index)`
- `list.Insert(index, "value")`

Unwrap back into slice via `list.Iter()` or `list.Unwrap()`

```go
list := gliter.List(0, 1, 2, 3, 4)
for _, item := range list.Iter() {
    fmt.Println(item)
}
```

Map to alt type via `Map(list, func(in) out)`

Something missing? Open a PR. **Contributions welcome!**
