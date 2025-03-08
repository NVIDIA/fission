# Copyright (c) 2015-2025, NVIDIA CORPORATION.
# SPDX-License-Identifier: Apache-2.0

EXAMPLE_DIRS = examples/fission-*

all: fmt build

all-tree: fmt-tree build-tree

.PHONY: all build build-tree clean clean-tree fmt fmt-tree lint-update lint lint-tree

build:
	go build .

build-tree:
	@echo "make build in root dir"
	@$(MAKE) --no-print-directory build
	@for example_dir in $(EXAMPLE_DIRS); do \
	  echo "make build in $$example_dir"; \
	  cd $$example_dir && $(MAKE) --no-print-directory build; \
	  cd - >/dev/null; \
	done

clean:
	go clean -i .

clean-tree:
	@echo "make clean in root dir"
	@$(MAKE) --no-print-directory clean
	@for example_dir in $(EXAMPLE_DIRS); do \
	  echo "make clean in $$example_dir"; \
	  cd $$example_dir && $(MAKE) --no-print-directory clean; \
	  cd - >/dev/null; \
	done

fmt:
	go fmt .

fmt-tree:
	@echo "make fmt in root dir"
	@$(MAKE) --no-print-directory fmt
	@for example_dir in $(EXAMPLE_DIRS); do \
	  echo "make fmt in $$example_dir"; \
	  cd $$example_dir && $(MAKE) --no-print-directory fmt; \
	  cd - >/dev/null; \
	done

lint-update:
	rm -f $(GOPATH)/bin/golangci-lint
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin latest

lint:
	golangci-lint run --config .golangci.yml .

lint-tree:
	golangci-lint run --config .golangci.yml `go list -f {{.Dir}} ./...`
