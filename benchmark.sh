#!/bin/bash

mkdir benchmark-results 2> /dev/null
mv benchmark-results/latest.txt benchmark-results/previous.txt 2> /dev/null

go test -c -o benchmark-results/test

#export GODEBUG=allocfreetrace=1,gctrace=1

BENCHMARK=${1:-.}
PREV_BENCHMARK=${2:-benchmark-results/previous.txt}

./benchmark-results/test  -test.bench=$BENCHMARK | tee benchmark-results/latest.txt
./benchmark-results/test  -test.bench=$BENCHMARK | tee -a benchmark-results/latest.txt
./benchmark-results/test  -test.bench=$BENCHMARK | tee -a benchmark-results/latest.txt
benchcmp "$PREV_BENCHMARK" benchmark-results/latest.txt
benchstat "$PREV_BENCHMARK" benchmark-results/latest.txt

cp "benchmark-results/latest.txt" "benchmark-results/$(date).txt"

