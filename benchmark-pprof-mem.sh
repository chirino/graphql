#!/bin/bash

mkdir benchmark-results 2> /dev/null
go test -c -o benchmark-results/test

#export GODEBUG=allocfreetrace=1,gctrace=1
#BENCHMARK=BenchmarkParallelParseStarwarsQuery
BENCHMARK=${1:-BenchmarkParallelParseStarwarsQuery}

./benchmark-results/test -test.bench=$BENCHMARK -test.benchmem -test.memprofile test.benchmark-results/latest.mem  | tee b benchmark-results/latest-memprofile.txt

go tool pprof -alloc_objects -lines -top benchmark-results/latest.mem | head -n 25
go tool pprof -alloc_space -lines -top benchmark-results/latest.mem | head -n 25