#!/bin/bash

mkdir benchmark-results 2> /dev/null
mv benchmark-results/latest.txt benchmark-results/previous.txt 2> /dev/null


go test -bench=. > benchmark-results/latest.txt
cp "benchmark-results/latest.txt" "benchmark-results/$(date).txt"

if [[  "" == "$1" ]] ; then
    benchstat benchmark-results/previous.txt benchmark-results/latest.txt
else
    benchstat "$1" benchmark-results/latest.txt
fi