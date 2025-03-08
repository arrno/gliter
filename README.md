# Gliter ✨

[![Go Reference](https://pkg.go.dev/badge/github.com/arrno/gliter.svg)](https://pkg.go.dev/github.com/arrno/gliter)
[![Go Build](https://github.com/arrno/gliter/actions/workflows/go.yml/badge.svg)](https://github.com/arrno/gliter/actions/workflows/go.yml)

**Go lang iter tools! ʕ◔ϖ◔ʔ**

Download:

```
go get github.com/arrno/gliter
```

Import:

```go
import "github.com/arrno/gliter"
```

> The mission of this project is to make it easy to compose normal business logic into complex async patterns. Ideally, we should spend most our mental energy solving our core problems instead of worrying about race conditions, deadlocks, channel states, and go-routine leaks. The patterns published here are all intended to serve that goal.

## Pipelines

Orchestrate a series of functions into a branching async pipeline with the Pipeline type.

```go
gliter.NewPipeline(streamTransactionsFromKafka).
    Stage(
        preprocessFeatures, // Normalize, extract relevant fields
    ).
    Stage(
        runFraudModel, // Model inference on transactions
        checkBusinessRules, // Non-ML heuristic checks
    ).
    Merge(
        aggregateResults, // Combine outputs from ML & rules
    ).
    Stage(
        sendToAlertSystem, // Notify if fraud detected
        storeInDatabase,   // Log for later analysis
    ).
    Run()
```

### Example

Start with a generator function and some simple transformers:

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
        exampleMid, // branch A
        exampleMid, // branch B
    ).
    Stage(
        exampleEnd, // Downstream of fork, exists in both branches
    ).
    Run()
```

Note that:

-   Data always flows downstream from generator through stages sequentially.
-   There is no distinct end stage. Any side-effects/outputs like db writes or API posts should be handled inside a Stage function wherever appropriate.
-   Any error encountered at any stage will short-circuit the pipeline.

### Fork

**Any time we choose to add multiple handlers in a single stage, we are forking the pipeline that many times.** When a fork occurs, all downstream stages are implicitly duplicated to exist in each stream and each transiting record is emitted on all available downstream paths doubling the number of processed records/streams.

**If you fork a stream that processes pointers, you should clone the record in downstream branches before mutating it.**

**What if I want to branch out a stage without duplicating records/streams?** In that case, check out the **Option** stage or the **InParallel** utility documented below. InParallel also demonstrated in `./cmd/pipeline_fan_out.go`.

### Throttle

What if our end stage results in a high number of concurrent output streams that overwhelms a destination DB or API? Use the throttle stage to rein in concurrent streams like this:

```go
// With concurrency throttling
gliter.NewPipeline(exampleGen()).
    Stage(
        exampleMid, // branch A
        exampleMid, // branch B
    ).
    Stage(
        exampleMid, // branches A.C, B.C
        exampleMid, // branches A.D, B.D
        exampleMid, // branches A.E, B.E
    ).
    Throttle(2). // throttle into branches X, Z
    Stage(
        exampleEnd,
    ).
    Run()
```

Here is a visual diagram of the pipeline the code produces:

![Alt text](./diag/small-chart.png?raw=true "Title")

### Batch

What if one of our stages does something slow, like a DB write, that could be optimized with batching? Use a special batch stage to pool items together for bulk processing:

```go
func exampleBatch(items []int) ([]int, error) {
    // A slow/expensive operation
    if err := storeToDB(items); err != nil {
        return nil, err
    }
    return items, nil
}

gliter.NewPipeline(exampleGen()).
    Config(gliter.PLConfig{LogCount: true}).
    Stage(exampleMid).
    Batch(100, exampleBatch).
    Stage(exampleEnd).
    Run()
```

gliter will handle converting the input-stream to batch and output-batch to stream for you which means batch stages are composable with normal stages.

### Merge

A merge stage is like a throttle but with a transform function to handle merging records from upstream sibling branches. Once all upstream branches are ready to emit, the merge handler receives one element from each via the slice argument. The handler can emit one or more resulting records which gliter converts back into a stream.

```go
gliter.NewPipeline(exampleGen()).
    Stage(
        exampleMid, // branch A
        exampleMid, // branch B
    ).
    Stage(
        exampleMid, // branches A.C, B.C
        exampleMid, // branches A.D, B.D
        exampleMid, // branches A.E, B.E
    ).
    Merge( // merges into branch Z
        func(items []int) ([]int, error) {
            sum := 0
            for _, item := range items {
                sum += item
            }
            return []int{sum}, nil
        },
    ).
    Stage(
        exampleEnd,
    ).
    Run()
```

### Option

An option stage resembles a regular forking stage but **does not fork/clone pipeline streams.** Each handler is an optional route for an inbound record and only one is selected. Multiple handlers can process records concurrently which means an option stage is also effectively a buffer. **A record has an equal chance of being emitted on any handler that is available.** That means faster handlers will process more records than slower handlers.

```go
gliter.NewPipeline(exampleGen()).
    Stage(exampleMid).
    Option( // each record will enter one of these
        func(item int) (int, error) {
            return 1 + item, nil
        },
        func(item int) (int, error) {
            return 2 + item, nil
        },
        func(item int) (int, error) {
            return 3 + item, nil
        },
    ).
    Stage(exampleEnd).
    Run()
```

### Buffer

In certain situations, you may want to introduce a buffer before a slow/expensive stage. Doing so **will not increase the overall performance of your pipeline** but may aid in faster response signalling to an upstream caller. Add a buffer like this:

```go
gliter.NewPipeline(exampleGen()).
    Stage(exampleMid).
    Buffer(5).
    Stage(exampleEnd).
    Run()
```

In this example, `exampleMid` can process/emit up to 5 results while exampleEnd is busy. Once the buffer is full, `exampleMid` is blocked again until `exampleEnd` pulls from the buffer.

### Tally

There are two ways to tally items processed by the pipeline.

-   Toggle on config to get all the node counts returned:

```go
counts, err := gliter.
    NewPipeline(exampleGen()).
    Config(gliter.PLConfig{ReturnCount: true}).
    Run()

if err != nil {
    panic(err)
}

for _, count := range counts {
    fmt.Printf("Node: %s\nCount: %d\n\n", count.NodeID, count.Count)
}
```

-   For more granular control, use the `Tally` channel like this:

```go
pipeline := NewPipeline(exampleGen())
tally := pipeline.Tally()

endWithTally := func(i int) (int, error) {
    tally <- 1 // any integer
    return exampleEnd(i)
}

// Produces one `PLNodeCount` for node "tally"
count, err := pipeline.
    Stage(endWithTally).
    Run()

if err != nil {
    panic(err)
}

// All integers sent to tally are summed
fmt.Printf("Node: %s\nCount: %d\n", count[0].NodeID, count[0].Count)
```

This is helpful if slices/maps are passed through the pipeline and you want to tally the total number or individual records processed. **Note that the tally channel is listened to while the pipeline is running and is closed when all pipeline stages exit.** For that reason, tally can always be written to freely within a stage function. Writes to tally outside of a stage function however is not recommended.

### Insight

It may be helpful during testing to audit what is happening inside a pipeline.

To do so, optionally set pipeline logging via one of the following modes in `pipeline.Config(gliter.PLConfig{...})`:

-   `LogCount` - Log result count table (start here)
-   `LogEmit` - Log every emit
-   `LogAll` - Log emit and count
-   `LogStep` - Interactive CLI stepper

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

The node id shown in the table is constructed as **stage-index** : **parent-index** : **func-index**. For example, node id `4:1:2` would indicate the third function (idx 2) of the fifth stage (idx 4) branched from the second parent node (idx 1).

"The second parent node" in this context can also be called the func at index 1 of the fourth stage.

Throttle, buffer, and merge stages are logged a bit differently as **[THROTTLE]**, **[BUFFER]**, and **[MERGE]** respectively.

> Note, you only pay for what you use. If logging it not enabled, these states are not tracked.

### Examples

-   For a more realistic pipeline example, see [./cmd/pipeline_example.go](https://github.com/arrno/gliter/blob/main/cmd/pipeline_example.go)
-   For an example of composing pipeline patterns with `InParallel` AKA Fan-in/Fan-out, see [./cmd/pipeline_fan_out.go](https://github.com/arrno/gliter/blob/main/cmd/pipeline_fan_out.go)
-   For an example with pipeline benchmarking, see [this repository](https://github.com/arrno/benchmark-gliter)

## In parallel

A Fan-in/Fan-out utility to run a series of functions in parallel and collect results **preserving order at no cost.**.

InParallel complement the Pipeline pattern in the following ways:

-   Use `InParallel` to run a set of unique pipelines concurrently
-   Call `InParallel` from inside a slow pipeline stage to fan-out/fan-in the expensive task

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

## Misc

Other async helpers in this library include:

-   Do your own throttling with `ThrottleBy`
-   Do your own channel forking with `TeeBy`
-   `ReadOrDone` + `WriteOrDone`
-   `IterDone` - iterate until read or done channel is closed
-   `Any` - take one from any, consolidating "done" channels

Also included are some synchronous iter tools that may be helpful:

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

Includes:

-   `list.Filter(func (i int) bool { return i%2 == 0 })`
-   `list.Map(func (i int) int { return i * 2 })`
-   `list.Reduce(func(acc *int, val int) { *acc += val })`
-   `list.Find(func (i int) bool { return i%2 == 0 })`
-   `list.Len()`
-   `list.Reverse()`
-   `list.At(i)`
-   `list.FPop()`
-   `list.Slice(start, stop)`
-   `list.Delete(index)`
-   `list.Insert(index, "value")`

Unwrap back into slice via `list.Iter()` or `list.Unwrap()`

```go
list := gliter.List(0, 1, 2, 3, 4)
for _, item := range list.Iter() {
    fmt.Println(item)
}
```

Map to alt type via `Map(list, func(in) out)`

Something missing? Open a PR. **Contributions welcome!**
