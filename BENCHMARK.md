# Benchmarking

Run benchmarks using the Go test command with the bench flag:

```bash
go test -bench=. -benchmem -count=5       # Run all benchmarks in the project
go test -bench 'Name' -benchmem -count=5  # Filter benchmarks by 'Name'
```

Save benchmark output to a file and compare them using the benchstat tool:

```bash
go tool benchstat bench_before.txt bench_after.txt
```
