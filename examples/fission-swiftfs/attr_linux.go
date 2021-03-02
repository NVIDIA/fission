// Copyright (c) 2015-2021, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/NVIDIA/fission"
)

func fixAttr(attr *fission.Attr) {
	attr.Blocks = (attr.Size + uint64(attrBlkSize) - 1) / uint64(attrBlkSize)
	attr.BlkSize = attrBlkSize
}
