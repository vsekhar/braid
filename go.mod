module github.com/vsekhar/braid

go 1.25.6

require google.golang.org/protobuf v1.36.11

require (
	github.com/aclements/go-moremath v0.0.0-20210112150236-f10218a38794 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/perf v0.0.0-20260312031701-16a31bc5fbd0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/telemetry v0.0.0-20260311193753-579e4da9a98c // indirect
	golang.org/x/tools v0.43.0 // indirect
)

tool (
	golang.org/x/perf/cmd/benchstat
	golang.org/x/tools/cmd/deadcode
)
