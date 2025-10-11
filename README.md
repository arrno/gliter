# Gliter ✨

[![Go Reference](https://pkg.go.dev/badge/github.com/arrno/gliter.svg)](https://pkg.go.dev/github.com/arrno/gliter)
[![Go Build](https://github.com/arrno/gliter/actions/workflows/go.yml/badge.svg)](https://github.com/arrno/gliter/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/arrno/gliter)](https://goreportcard.com/report/github.com/arrno/gliter)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

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
    -   [WorkPool](#workpool-stage)
    -   [MixPool](#mixpool)
    -   [Throttle](#throttle)
    -   [Merge](#merge)
    -   [ForkOutIn](#ForkOutIn)
    -   [Batch](#batch)
    -   [Option](#option)
    -   [Buffer](#buffer)
    -   [Context / Cancel](#context--cancel)
    -   [Count / Tally](#count--tally)
    -   [Insight](#insight)
-   [InParallel](#inparallel)
-   [Worker Pool (standalone)](#worker-pool-standalone)
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

Options:

-   [Insight options](#insight)
-   [Cancel options](#context--cancel)

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

For branching without duplicating streams, use [`WorkPool Stage`](#workpool-stage), [`Option Stage`](#option), or [`InParallel`](#inparallel).

---

### WorkPool Stage

Fans out a handler function into `N` concurrent workers.  
Each record is processed by exactly one worker (no cloning or duplication), then multiplexed onto the single downstream stream.

Configure behavior with options:

-   `WithBuffer(M)` → buffered channel capacity between upstream and workers
-   `WithRetry(R)` → automatic retries on failure

Allows fine-grained control over throughput, backpressure, and fault tolerance.

```go
gliter.NewPipeline(exampleGen()).
    WorkPool(
        func(item int) (int, error) { return 1 + item, nil },
        3, // numWorkers
        WithBuffer(6),
        WithRetry(2),
    ).
    WorkPool(
        func(item int) (int, error) { return 2 + item, nil },
        6, // numWorkers
        WithBuffer(12),
        WithRetry(2),
    ).
    Run()
```

---

### MixPool

Use when a worker pool needs distinct handlers but you still want automatic backpressure.  
It's a convenience wrapper around `WorkPool`: pass a slice of handlers and Gliter will fan the stream across them while keeping worker semantics.

```go
gliter.NewPipeline(exampleGen()).
    MixPool([]func(int) (int, error){
        func(item int) (int, error) { return item + 1, nil },
        func(item int) (int, error) { return item * 2, nil },
    },
        WithRetry(1),
    ).
    Run()
```

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

### ForkOutIn

Shortcut for "fork, do a little work, then merge" flows.  
Under the hood it's just a `Fork` followed by `Merge`, so keep it brief for small aggregations.

```go
gliter.NewPipeline(exampleGen()).
    ForkOutIn(
        []func(int) (int, error){
            func(item int) (int, error) { return item + 1, nil },
            func(item int) (int, error) { return item - 1, nil },
        },
        func(items []int) ([]int, error) { return []int{items[0] - items[1]}, nil },
    ).
    Run()
```

Stick with the dedicated `Fork`/`Merge` stages when you need more complex fan-out trees.

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

### Context / Cancel

Use the `WithContext` option for timeout/cancel:

```go
ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
defer cancel()

gliter.NewPipeline(
    exampleGen(),
    gliter.WithContext(ctx),
).
    Stage(exampleMid).
    Stage(exampleEnd).
    Run()
```

---

### Count / Tally

Count items processed, either via config:

```go
counts, err := gliter.NewPipeline(
    exampleGen(),
    gliter.WithReturnCount(),
).Run()
```

Or with a tally channel:

```go
pipeline := NewPipeline(exampleGen())
tally := pipeline.Tally()
```

---

### Insight

Enable logging for debugging:

-   `WithLogCount` — summary counts
-   `WithLogEmit` — every emission
-   `WithLogErr` — errors
-   `WithLogAll` — all above
-   `WithLogStep` — interactive stepper

```go
gliter.NewPipeline(
    exampleGen(),
    gliter.WithLogAll(),
).
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

## Worker Pool (standalone)

Generic worker pools in one line:

```go
results, errors := gliter.NewWorkerPool(3, handler).
    Push(0, 1, 2, 3, 4).
    Close().
    Collect()
```

Supported WorkerPool Options:

-   `WithRetry`
-   `WithBuffer`

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
-   [Pipeline with fan-out](./examples/pipeline_fan_out.go)
-   [WorkerPool](./examples/worker_pool_example.go)
-   [Benchmarks](https://github.com/arrno/benchmark-gliter)

---

## Contributing

PRs welcome! 🚀  
If something feels missing, broken, or unclear, open an issue or submit a fix.
