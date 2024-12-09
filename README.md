# Glitter ✨
**Iter tools for Go**

**Push-Pop**
```go
list := glitter.List(0, 1, 2, 3, 4)
list.Pop() // removes/returns `4`
list.Push(8) // appends `8`
```

**More**
```go
val := glitter.
    List(0, 1, 2, 3, 4).
    Filter(func(i int) bool { 
        return i%2 == 0 
    }).
    Map(func(val int) int {
        return val * 2
    }). // []int{0, 4, 8}
    Reduce(func(acc *int, val int) {
        *acc += val
    }) // 12
```