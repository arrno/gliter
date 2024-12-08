# Glitter ✨
**Iter tools for Go**

**Push-Pop**
```go
list := glitter.IntoList([]uint{0, 1, 2, 3, 4})
list.Pop() // removes/returns `4`
list.Push(8) // appends `8`
```

**More**
```go
val := glitter.
    IntoList([]uint{0, 1, 2, 3, 4}).
    Filter(func(i uint) bool { return i%2 == 0 }).
    Map(func(val uint) uint {
        return val * 2
    }). // []uint{0, 4, 8}
    Reduce(func(acc *uint, val uint) {
        *acc += val
    }) // 12
```