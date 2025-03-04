# Copyright (c) 2025, NVIDIA CORPORATION.
# SPDX-License-Identifier: Apache-2.0

all: fmt build

.PHONY: all build clean fmt lint-update lint

build:
	go build .

clean:
	go clean -i .

fmt:
	go fmt .

lint-update:
	rm -f $(GOPATH)/bin/golangci-lint
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin latest

lint:
	golangci-lint run --config ../../.golangci.yml .
