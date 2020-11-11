#!/bin/bash

echo "Run e2e test on the nightly payload"
./bin/extended-platform-tests run all --dry-run | grep "OLM" | ./bin/extended-platform-tests run -f -
