#!/bin/bash

mkdir benchmark-results 2> /dev/null
go test -bench="$1" -benchmem -memprofile benchmark-results/latest.mem > benchmark-results/latest.txt
go tool pprof -alloc_objects -lines -top benchmark-results/latest.mem | head -n 25
go tool pprof -alloc_space -lines -top benchmark-results/latest.mem | head -n 25