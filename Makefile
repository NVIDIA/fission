# Copyright (c) 2015-2021, NVIDIA CORPORATION.
# SPDX-License-Identifier: Apache-2.0

all: fmt build

.PHONY: all build clean fmt

build:
	go build .

clean:
	go clean -i .

fmt:
	go fmt .
