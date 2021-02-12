// Copyright (c) 2015-2021, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/NVIDIA/fission"
)

func fixAttr(attr *fission.Attr) {
	attr.CrTimeSec = attr.CTimeSec
	attr.CrTimeNSec = attr.CTimeNSec
	attr.Flags = 0
}
