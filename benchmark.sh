#!/bin/bash

mkdir benchmark-results 2> /dev/null
mv benchmark-results/latest.txt benchmark-results/previous.txt 2> /dev/null

go test -c -o benchmark-results/test

#export GODEBUG=allocfreetrace=1,gctrace=1
#BENCHMARK=BenchmarkParallelParseStarwarsQuery
BENCHMARK=${1:-.}

results() {
    if [[  "" == "$1" ]] ; then
        benchstat benchmark-results/previous.txt benchmark-results/latest.txt
    else
        benchstat "$1" benchmark-results/latest.txt
    fi
}

./benchmark-results/test  -test.bench=$BENCHMARK | tee benchmark-results/latest.txt
results $1
./benchmark-results/test  -test.bench=$BENCHMARK | tee -a benchmark-results/latest.txt
results $1
./benchmark-results/test  -test.bench=$BENCHMARK | tee -a benchmark-results/latest.txt
results $1

cp "benchmark-results/latest.txt" "benchmark-results/$(date).txt"

