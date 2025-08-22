# Gliter ✨

[![Go Reference](https://pkg.go.dev/badge/github.com/arrno/gliter.svg)](https://pkg.go.dev/github.com/arrno/gliter)  
[![Go Build](https://github.com/arrno/gliter/actions/workflows/go.yml/badge.svg)](https://github.com/arrno/gliter/actions/workflows/go.yml)

> **Composable async & concurrency patterns for Go.**  
> Write pipelines, worker pools, and async utilities without worrying about race conditions, deadlocks, or goroutine leaks.

---

## Quick Start

Install:

```bash
go get github.com/arrno/gliter
```

Import:

```go
import "github.com/arrno/gliter"
```

---

## Table of Contents

-   [Overview](#overview)
-   [Pipeline](#pipeline)
    -   [Fork](#fork)
    -   [Throttle](#throttle)
    -   [Merge](#merge)
    -   [Batch](#batch)
    -   [Option](#option)
    -   [Buffer](#buffer)
    -   [Tally](#tally)
    -   [Insight](#insight)
-   [InParallel](#inparallel)
-   [Worker Pool](#worker-pool)
-   [Misc Utilities](#misc-utilities)
-   [Examples](#examples)
-   [Contributing](#contributing)

---

## Overview

Gliter makes it easy to assemble **normal business logic** into **complex async flows**.

Instead of spending time debugging goroutines, channels, or leaks, you define your data flow declaratively and let Gliter handle the concurrency patterns.

---

## Pipeline

Compose stages of functions into a branching async pipeline:

```go
gliter.NewPipeline(streamTransactionsFromKafka).
    Stage(
        preprocessFeatures,
    ).
    Stage(
        runFraudModel,
        checkBusinessRules,
    ).
    Merge(aggregateResults).
    Stage(
        sendToAlertSystem,
        storeInDatabase,
    ).
    Run()
```

Key properties:

-   Data always flows downstream from the generator through each stage.
-   Side effects (DB writes, API calls, etc.) belong inside stage functions.
-   Errors short-circuit the pipeline automatically.

---

### Fork

Add multiple handlers in one stage to fork the pipeline:

```go
gliter.NewPipeline(exampleGen()).
    Stage(
        exampleMid, // branch A
        exampleAlt, // branch B
    ).
    Stage(exampleEnd).
    Run()
```

👉 Each downstream stage is duplicated for each branch.  
⚠️ If processing pointers, clone before mutating downstream.

For branching without duplicating streams, use [`Option`](#option) or [`InParallel`](#inparallel).

---

### Throttle

Control concurrency when downstream stages overwhelm your DB or API:

```go
gliter.NewPipeline(exampleGen()).
    Stage(exampleMid, exampleMid).
    Stage(exampleMid, exampleMid, exampleMid).
    Throttle(2).
    Stage(exampleEnd).
    Run()
```

---

### Merge

Combine multiple branches into one:

```go
gliter.NewPipeline(exampleGen()).
    Stage(exampleMid, exampleMid).
    Merge(func(items []int) ([]int, error) {
        sum := 0
        for _, item := range items {
            sum += item
        }
        return []int{sum}, nil
    }).
    Stage(exampleEnd).
    Run()
```

---

### Batch

Batch records for bulk operations:

```go
func exampleBatch(items []int) ([]int, error) {
    if err := storeToDB(items); err != nil {
        return nil, err
    }
    return items, nil
}

gliter.NewPipeline(exampleGen()).
    Stage(exampleMid).
    Batch(100, exampleBatch).
    Stage(exampleEnd).
    Run()
```

---

### Option

Route each record to exactly one handler (no cloning):

```go
gliter.NewPipeline(exampleGen()).
    Stage(exampleMid).
    Option(
        func(item int) (int, error) { return 1 + item, nil },
        func(item int) (int, error) { return 2 + item, nil },
        func(item int) (int, error) { return 3 + item, nil },
    ).
    Stage(exampleEnd).
    Run()
```

---

### Buffer

Insert a buffer before a slow stage:

```go
gliter.NewPipeline(exampleGen()).
    Stage(exampleMid).
    Buffer(5).
    Stage(exampleEnd).
    Run()
```

---

### Tally

Count items processed, either via config:

```go
counts, err := gliter.NewPipeline(exampleGen()).
    Config(gliter.PLConfig{ReturnCount: true}).
    Run()
```

Or with a tally channel:

```go
pipeline := NewPipeline(exampleGen())
tally := pipeline.Tally()
```

---

### Insight

Enable logging for debugging:

-   `LogCount` — summary counts
-   `LogEmit` — every emission
-   `LogAll` — both
-   `LogStep` — interactive stepper

```go
gliter.NewPipeline(exampleGen()).
    Config(gliter.PLConfig{LogAll: true}).
    Stage(exampleMid, exampleAlt).
    Stage(exampleEnd).
    Run()
```

---

## InParallel

Fan-out tasks, run concurrently, and collect results in order:

```go
tasks := []func() (string, error){
    func() (string, error) { return "Hello", nil },
    func() (string, error) { return ", ", nil },
    func() (string, error) { return "Async!", nil },
}

results, err := gliter.InParallel(tasks)
```

Also available: `InParallelThrottle` for token-bucket concurrency.

---

## Worker Pool

Generic worker pools in one line:

```go
results, errors := gliter.NewWorkerPool(3, handler).
    Push(0, 1, 2, 3, 4).
    Close().
    Collect()
```

See `./examples/worker_pool_example.go` for more.

---

## Misc Utilities

-   `ThrottleBy` — custom throttling
-   `TeeBy` — channel forking
-   `ReadOrDone`, `WriteOrDone`, `IterDone`
-   `Any` — consolidate “done” channels
-   `Multiplex` — merge streams

### List Type (sync helpers)

```go
val := gliter.
    List(0, 1, 2, 3, 4).
    Filter(func(i int) bool { return i%2 == 0 }).
    Map(func(val int) int { return val * 2 }).
    Reduce(func(acc *int, val int) { *acc += val }) // 12
```

Includes `Filter`, `Map`, `Reduce`, `Find`, `Len`, `Reverse`, `At`, `Slice`, etc.

---

## Examples

-   [Pre-built generators](./examples/generators)
-   [Pipeline example](./examples/pipeline_example.go)
-   [Fan-out / Fan-in](./examples/pipeline_fan_out.go)
-   [Benchmarks](https://github.com/arrno/benchmark-gliter)

---

## Contributing

PRs welcome! 🚀  
If something feels missing, broken, or unclear, open an issue or submit a fix.
