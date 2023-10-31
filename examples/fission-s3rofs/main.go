// Copyright (c) 2023, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/NVIDIA/fission"
)

const (
	fuseSubtype   = "fission-swiftfs"
	httpUserAgent = "fission-swiftfs"

	initOutFlagsReadOnly = uint32(0) |
		fission.InitFlagsAsyncRead |
		fission.InitFlagsFileOps |
		fission.InitFlagsDoReadDirPlus |
		fission.InitFlagsReaddirplusAuto |
		fission.InitFlagsParallelDirops |
		fission.InitFlagsMaxPages |
		fission.InitFlagsNoOpendirSupport |
		fission.InitFlagsExplicitInvalData

	initOutMaxBackgound         = uint16(100)
	initOutCongestionThreshhold = uint16(0)

	maxPages = 256                     // * 4KiB page size == 1MiB... the max read or write size in Linux FUSE at this time
	maxRead  = uint32(maxPages * 4096) //                     1MiB... the max read          size in Linux FUSE at this time
	maxWrite = uint32(maxPages * 4096) //                     1MiB... the max         write size in Linux FUSE at this time

	attrBlkSize = uint32(512)

	entryValidSec  = uint64(10)
	entryValidNSec = uint32(0)

	attrValidSec  = uint64(10)
	attrValidNSec = uint32(0)

	accessROK = syscall.S_IROTH // surprisingly not defined as syscall.R_OK
	accessWOK = syscall.S_IWOTH // surprisingly not defined as syscall.W_OK
	accessXOK = syscall.S_IXOTH // surprisingly not defined as syscall.X_OK

	accessMask       = syscall.S_IRWXO // used to mask Owner, Group, or Other RWX bits
	accessOwnerShift = 6
	accessGroupShift = 3
	accessOtherShift = 0

	dirMode  = uint32(syscall.S_IFDIR | syscall.S_IRUSR | syscall.S_IXUSR | syscall.S_IRGRP | syscall.S_IXGRP | syscall.S_IROTH | syscall.S_IXOTH)
	fileMode = uint32(syscall.S_IFREG | syscall.S_IRUSR | syscall.S_IRGRP | syscall.S_IROTH)

	authModeNoAuthNeeded  = uint8(0)
	authModeTokenProvided = uint8(1)
	authModeURLProvided   = uint8(2)
)

type configStruct struct {
	MountPoint        string
	S3AccessKeyMasked string `json:"S3AccessKey"`
	s3AccessKey       string
	S3SecretKeyMasked string `json:"S3SecretKey"`
	s3SecretKey       string
	S3Endpoint        string
	S3Region          string
	S3Attempts        uint64
	S3Backoff         uint64
	S3Bucket          string
	S3Prefix          string
	CacheDirPath      string
	FileCacheLines    uint64
	NumCacheLines     uint64
	CacheLineSize     uint64
}

type globalsStruct struct {
	config   *configStruct
	s3Client *s3.Client
}

var globals globalsStruct

func main() {
	var (
		configFileContent []byte
		err               error
	)

	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <configFile>\n", os.Args[0])
		fmt.Printf("  where <configFile> is a JSON object of the form:\n")
		fmt.Printf("    {\n")
		fmt.Printf("      \"MountPoint\"     : \"<path to empty dir upon which to mount>\",\n")
		fmt.Printf("      \"S3AccessKey\"    : \"<S3 AccessKey>\",\n")
		fmt.Printf("                           // if empty, fetched from ENV\n")
		fmt.Printf("      \"S3SecretKey\"    : \"<S3 SecretKey>\",\n")
		fmt.Printf("                           // if empty, fetched from ENV\n")
		fmt.Printf("      \"S3Endpoint\"     : \"<S3 Endpoint>\",\n")
		fmt.Printf("      \"S3Region\"       : \"<S3 Region>\",\n")
		fmt.Printf("                           // defaults to \"us-east-1\"\n")
		fmt.Printf("      \"S3Attempts\"     : <S3 MaxAttempts>,\n")
		fmt.Printf("                           // defaults to 5\n")
		fmt.Printf("      \"S3Backoff\"      : <S3 MaxBackoffDeleay in seconds>,\n")
		fmt.Printf("                           // defaults to 60\n")
		fmt.Printf("      \"S3Bucket\"       : \"<S3 Bucket Name>\",\n")
		fmt.Printf("      \"S3Prefix\"       : \"<S3 Object Prefix>\",\n")
		fmt.Printf("      \"CacheDirPath\"   : \"<path to dir for non-RAM cache lines>\",\n")
		fmt.Printf("                           // if FileCacheLines == 0, ignored\n")
		fmt.Printf("                           // if FileCacheLines != 0, emptied at launch\n")
		fmt.Printf("      \"FileCacheLines\" : <number of cache lines in CacheDirPath>,\n")
		fmt.Printf("      \"RAMCacheLines\"  : <number of cache lines in RAM>,\n")
		fmt.Printf("      \"CacheLineSize\"  : <(max) size of each cache line>\n")
		fmt.Printf("    }\n")
		os.Exit(0)
	}

	configFileContent, err = ioutil.ReadFile(os.Args[1])
	if nil != err {
		fmt.Printf("ioutil.ReadFile(\"%s\") failed: %v\n", os.Args[1], err)
		os.Exit(1)
	}

	globals.config = &configStruct{}

	err = json.Unmarshal(configFileContent, globals.config)
	if nil != err {
		fmt.Printf("json.Unmarshal(configFileContent, config) failed: %v\n", err)
		os.Exit(1)
	}

	if globals.config.S3AccessKeyMasked == "" {
		globals.config.s3AccessKey = os.Getenv("S3AccessKey")
	} else {
		globals.config.s3AccessKey = globals.config.S3AccessKeyMasked
	}
	globals.config.S3AccessKeyMasked = "****************"

	if globals.config.S3SecretKeyMasked == "" {
		globals.config.s3SecretKey = os.Getenv("S3SecretKey")
	} else {
		globals.config.s3SecretKey = globals.config.S3SecretKeyMasked
	}
	globals.config.S3SecretKeyMasked = "****************"

	if globals.config.S3Region == "" {
		globals.config.S3Region = "us-east-1"
	}

	if globals.config.S3Attempts == 0 {
		globals.config.S3Attempts = 5
	}

	if globals.config.S3Backoff == 0 {
		globals.config.S3Backoff = 60
	}

	if globals.config.FileCacheLines != 0 {
		err = os.MkdirAll(globals.config.CacheDirPath, 0777)
		if err != nil {
			fmt.Printf("os.MkdirAll(globals.config.CacheDirPath, 0777) [Case 1] failed: %v\n", err)
		}
		err = os.RemoveAll(globals.config.CacheDirPath)
		if err != nil {
			fmt.Printf("os.RemoveAll(globals.config.CacheDirPath) failed: %v\n", err)
		}
		err = os.MkdirAll(globals.config.CacheDirPath, 0777)
		if err != nil {
			fmt.Printf("os.MkdirAll(globals.config.CacheDirPath, 0777) [Case 2] failed: %v\n", err)
		}
	}

	if globals.config.CacheLineSize == 0 {
		fmt.Printf("CacheLineSize must be > 0\n")
		os.Exit(1)
	}

	fmt.Printf("UNDO: lobals.config: %#v\n", globals.config)
}
