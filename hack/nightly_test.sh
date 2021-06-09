#!/bin/bash

./bin/extended-platform-tests run all --dry-run | grep "OLM" | grep -vi "opm" | grep -vi "VMonly" | ./bin/extended-platform-tests run -f -
echo "Run e2e test on the nightly payload: ./bin/extended-platform-tests run all --dry-run | grep "OLM" | grep -vi "opm" | grep -vi "VMonly" | ./bin/extended-platform-tests run -f -"
