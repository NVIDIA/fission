// Copyright (c) 2023, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/NVIDIA/sortedmap"

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

	dirMode  = uint32(syscall.S_IFDIR | syscall.S_IRUSR | syscall.S_IXUSR | syscall.S_IRGRP | syscall.S_IXGRP | syscall.S_IROTH | syscall.S_IXOTH)
	fileMode = uint32(syscall.S_IFREG | syscall.S_IRUSR | syscall.S_IRGRP | syscall.S_IROTH)

	rootDirInodeNumber = uint64(1)

	dotDirTableEntryName    = string(".")
	dotDotDirTableEntryName = string("..")
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

type inodeStruct struct {
	inodeNumber uint64
	linkCount   uint64
	mode        uint32             // & syscall.S_IFMT will be either syscall.S_IFDIR or syscall.S_IFREG
	dirTable    sortedmap.LLRBTree // [only for syscall.S_IFDIR] key == string "name"; value == inodeNumber
	fileSize    int64              // [only for syscall.S_IFREG] a value < 0 means not yet fetched
}

type globalsStruct struct {
	config     *configStruct
	logger     *log.Logger
	s3Client   *s3.Client
	inodeTable sortedmap.LLRBTree // key == uint64(inodeNumber); value == *inodeStruct
}

var globals globalsStruct

func main() {
	var (
		configAsJSON                       []byte
		configFileContent                  []byte
		err                                error
		found                              bool
		inode                              *inodeStruct
		listObjectsV2Input                 *s3.ListObjectsV2Input
		listObjectsV2Output                *s3.ListObjectsV2Output
		listObjectsV2OutputContentsElement s3types.Object
		objectNameCutPrefix                string
		ok                                 bool
		s3Config                           aws.Config
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

	globals.logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

	configFileContent, err = ioutil.ReadFile(os.Args[1])
	if nil != err {
		globals.logger.Fatalf("ioutil.ReadFile(\"%s\") failed: %v", os.Args[1], err)
	}

	globals.config = &configStruct{}

	err = json.Unmarshal(configFileContent, globals.config)
	if nil != err {
		globals.logger.Fatalf("json.Unmarshal(configFileContent, config) failed: %v", err)
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
			globals.logger.Fatalf("os.MkdirAll(globals.config.CacheDirPath, 0777) [Case 1] failed: %v", err)
		}
		err = os.RemoveAll(globals.config.CacheDirPath)
		if err != nil {
			globals.logger.Fatalf("os.RemoveAll(globals.config.CacheDirPath) failed: %v", err)
		}
		err = os.MkdirAll(globals.config.CacheDirPath, 0777)
		if err != nil {
			globals.logger.Fatalf("os.MkdirAll(globals.config.CacheDirPath, 0777) [Case 2] failed: %v", err)
		}
	}

	if globals.config.CacheLineSize == 0 {
		globals.logger.Fatalf("CacheLineSize must be > 0")
	}

	configAsJSON, err = json.Marshal(globals.config)
	if err != nil {
		globals.logger.Printf("json.Marshal(globals.config) failed: %v", err)
	}
	globals.logger.Printf("globals.config: %s", string(configAsJSON[:]))

	globals.inodeTable = sortedmap.NewLLRBTree(sortedmap.CompareUint64, &globals)

	inode = &inodeStruct{
		inodeNumber: rootDirInodeNumber,
		linkCount:   2,
		mode:        dirMode,
		dirTable:    nil, // filled in below
		fileSize:    0,   // ignored for .mode == dirMode
	}

	inode.dirTable = sortedmap.NewLLRBTree(sortedmap.CompareString, inode)

	ok, err = inode.dirTable.Put(dotDirTableEntryName, rootDirInodeNumber)
	if err != nil {
		globals.logger.Fatalf("inode.dirTable.Put(dotDirTableEntryName, rootDirInodeNumber) failed %v", err)
	}
	if !ok {
		globals.logger.Fatalf("inode.dirTable.Put(dotDirTableEntryName, rootDirInodeNumber) returned !ok")
	}

	ok, err = inode.dirTable.Put(dotDotDirTableEntryName, rootDirInodeNumber)
	if err != nil {
		globals.logger.Fatalf("inode.dirTable.Put(dotDotDirTableEntryName, rootDirInodeNumber) failed %v", err)
	}
	if !ok {
		globals.logger.Fatalf("inode.dirTable.Put(dotDotDirTableEntryName, rootDirInodeNumber) returned !ok")
	}

	ok, err = globals.inodeTable.Put(inode.inodeNumber, inode)
	if err != nil {
		globals.logger.Fatalf("globals.inodeTable.Put(inode.inodeNumber, inode) failed %v", err)
	}
	if !ok {
		globals.logger.Fatalf("globals.inodeTable.Put(inode.inodeNumber, inode) returned !ok")
	}

	s3Config, err = config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     globals.config.s3AccessKey,
				SecretAccessKey: globals.config.s3SecretKey,
			},
		}),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               globals.config.S3Endpoint,
					SigningRegion:     globals.config.S3Region,
					HostnameImmutable: true,
				}, nil
			})),
		config.WithRegion(globals.config.S3Region),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxBackoffDelay(retry.AddWithMaxAttempts(retry.NewStandard(), int(globals.config.S3Attempts)), time.Duration(int(globals.config.S3Backoff))*time.Second)
		}))
	if err != nil {
		log.Fatalf("config.LoadDefaultConfig() failed: %v", err)
	}

	globals.s3Client = s3.NewFromConfig(s3Config)

	listObjectsV2Input = &s3.ListObjectsV2Input{
		Bucket: aws.String(globals.config.S3Bucket),
		Prefix: aws.String(globals.config.S3Prefix),
	}
	listObjectsV2Output = &s3.ListObjectsV2Output{
		IsTruncated:           true,
		NextContinuationToken: nil,
	}

	for listObjectsV2Output.IsTruncated {
		listObjectsV2Input.ContinuationToken = listObjectsV2Output.NextContinuationToken

		listObjectsV2Output, err = globals.s3Client.ListObjectsV2(context.TODO(), listObjectsV2Input)
		if err != nil {
			log.Fatalf("globals.s3Client.ListObjectsV2() failed: %v", err)
		}

		for _, listObjectsV2OutputContentsElement = range listObjectsV2Output.Contents {
			objectNameCutPrefix, found = strings.CutPrefix(*listObjectsV2OutputContentsElement.Key, globals.config.S3Prefix)
			if !found {
				log.Fatalf("strings.CutPrefix(\"%s\", globals.args.S3Prefix) returned !found", *listObjectsV2OutputContentsElement.Key)
			}

			globals.logger.Printf("UNDO: found objectNameCutPrefix: \"%s\"", objectNameCutPrefix)
		}
	}
}

func (dummy *globalsStruct) DumpKey(key sortedmap.Key) (keyAsString string, err error) {
	var (
		keyAsUint64 uint64
		ok          bool
	)

	keyAsUint64, ok = key.(uint64)
	if !ok {
		err = fmt.Errorf("key.(uint64) returned !ok")
		return
	}

	keyAsString = fmt.Sprintf("%08X", keyAsUint64)

	err = nil
	return
}

func (dummy *globalsStruct) DumpValue(value sortedmap.Value) (valueAsString string, err error) {
	var (
		ok                 bool
		valueAsInodeStruct *inodeStruct
	)

	valueAsInodeStruct, ok = value.(*inodeStruct)
	if !ok {
		err = fmt.Errorf("value.(*inodeStruct) returned !ok")
		return
	}

	valueAsString = fmt.Sprintf("{inodeNumber:%08X,linkCount=%d,mode=%04X}", valueAsInodeStruct.inodeNumber, valueAsInodeStruct.linkCount, valueAsInodeStruct.mode)

	err = nil
	return
}

func (inode *inodeStruct) DumpKey(key sortedmap.Key) (keyAsString string, err error) {
	var (
		ok bool
	)

	keyAsString, ok = key.(string)
	if !ok {
		err = fmt.Errorf("key.(string) returned !ok")
		return
	}

	err = nil
	return
}

func (inode *inodeStruct) DumpValue(value sortedmap.Value) (valueAsString string, err error) {
	var (
		ok            bool
		valueAsUint64 uint64
	)

	valueAsUint64, ok = value.(uint64)
	if !ok {
		err = fmt.Errorf("value.(uint64) returned !ok")
		return
	}

	valueAsString = fmt.Sprintf("%08X", valueAsUint64)

	err = nil
	return
}
