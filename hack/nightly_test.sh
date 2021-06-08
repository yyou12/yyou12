#!/bin/bash

echo "Run e2e test on the nightly payload"
./bin/extended-platform-tests run all --dry-run | grep "OLM" | grep -vi "opm" | ./bin/extended-platform-tests run -f -
