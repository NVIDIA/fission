// Copyright (c) 2023, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"golang.org/x/sys/unix"

	"github.com/NVIDIA/fission"
	"github.com/NVIDIA/sortedmap"
)

const (
	fileCacheDirPattern = "fission-s3rofs_cache"

	fuseSubtype = "fission-s3rofs"

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
	maxWrite = 0                       // indicates the volume is to be mounted ReadOnly

	attrUID = uint32(0)
	attrGID = uint32(0)

	attrRDev = uint32(0)

	attrBlkSize = uint32(512)

	entryGeneration = uint64(0)

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

	rootDirInodeNumber = uint64(1)

	dotDirTableEntryName    = string(".")
	dotDotDirTableEntryName = string("..")
)

type configStruct struct {
	Verbose           bool
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
	FileCacheLines    uint64
	RAMCacheLines     uint64
	CacheLineSize     uint64
}

type inodeStruct struct {
	inodeNumber  uint64             //
	size         uint64             // [only for syscall.S_IFREG] Attr.Size (Attr.Blocks = round_up(size/attrBlkSize))
	lastModified time.Time          // converted to Attr.{A|M|C}Time{Sec|NSec}
	mode         uint32             // & syscall.S_IFMT will be either syscall.S_IFDIR or syscall.S_IFREG
	linkCount    uint64             //
	dirTable     sortedmap.LLRBTree // [only for syscall.S_IFDIR] key == string "basename"; value == inodeNumber
	objectKey    string             // [only for syscall.S_IFREG] S3 Key (includes Prefix)
}

type cacheLineTagStruct struct {
	inodeNumber uint64 // ...of fileInode
	lineNumber  uint64 // starting offset within fileInode's contents == .lineNumber * globals.config.CacheLineSize
}

type fileCacheLineStruct struct {
	listElement    *list.Element
	tag            cacheLineTagStruct
	sync.WaitGroup      // used to signal those awaiting contentReady to become true or purge of this fileCacheLine
	contentReady   bool // if true, content available at file path fmt.Sprintf("%s/%08X_%08X", globals.fileCacheDir, tag.inodeNumber, tag.lineNumber)
}

type ramCacheLineStruct struct {
	listElement    *list.Element
	tag            cacheLineTagStruct
	sync.WaitGroup        // used to signal those awaiting content to be non-nil
	content        []byte // len() <= globals.config.CacheLineSize; nil if being populated
}

type globalsStruct struct {
	config       *configStruct
	logger       *log.Logger
	s3Client     *s3.Client
	inodeTable   []*inodeStruct // index == uint64(inodeNumber - 1)
	blocks       uint64
	sync.Mutex   // protects {file|ram}Cache{LRU|Map}
	fileCacheDir string
	fileCacheLRU *list.List                                  // fileCacheLineStruct.listElement linked LRU
	fileCacheMap map[cacheLineTagStruct]*fileCacheLineStruct // key == fileCacheLineStruct.tag; value == *fileCacheLineStruct
	ramCacheLRU  *list.List                                  // ramCacheLineStruct.listElement linked LRU
	ramCacheMap  map[cacheLineTagStruct]*ramCacheLineStruct  // key == ramCacheLineStruct.tag; value == *ramCacheLineStruct
	errChan      chan error
	volume       fission.Volume
}

var globals globalsStruct

func main() {
	var (
		childInode                         *inodeStruct
		childInodeNumber                   uint64
		childInodeNumberAsValue            sortedmap.Value
		configAsJSON                       []byte
		configFileContent                  []byte
		err                                error
		fileInodeBlocks                    uint64
		found                              bool
		listObjectsV2Input                 *s3.ListObjectsV2Input
		listObjectsV2Output                *s3.ListObjectsV2Output
		listObjectsV2OutputContentsElement s3types.Object
		objectKey                          string
		objectKeyCutPrefix                 string
		objectKeyCutPrefixSlice            []string
		objectKeyCutPrefixSliceElement     string
		ok                                 bool
		parentInode                        *inodeStruct
		s3Config                           aws.Config
		signalChan                         chan os.Signal
		timeAtLaunch                       time.Time = time.Now()
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
		fmt.Printf("      \"FileCacheLines\" : <number of cache lines in CacheDirPath>,\n")
		fmt.Printf("                           // if == 0, file caching disabled\n")
		fmt.Printf("                           // if >  0, file cache lines overflowing RAM\n")
		fmt.Printf("                           //          stored in files in a TempDir\n")
		fmt.Printf("      \"RAMCacheLines\"  : <number of cache lines in RAM> (must be > 0),\n")
		fmt.Printf("                           // must be > 0\n")
		fmt.Printf("      \"CacheLineSize\"  : <(max) size of each cache line> (must be > 0)\n")
		fmt.Printf("                           // must be > 0\n")
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

	if globals.config.RAMCacheLines == 0 {
		globals.logger.Fatalf("RAMCacheLines must be > 0")
	}

	if globals.config.CacheLineSize == 0 {
		globals.logger.Fatalf("CacheLineSize must be > 0")
	}

	if globals.config.Verbose {
		configAsJSON, err = json.Marshal(globals.config)
		if err != nil {
			globals.logger.Printf("json.Marshal(globals.config) failed: %v", err)
		}

		globals.logger.Printf("globals.config: %s", string(configAsJSON[:]))
	}

	globals.inodeTable = make([]*inodeStruct, 0)

	globals.blocks = 0

	childInode = &inodeStruct{
		inodeNumber:  rootDirInodeNumber,
		lastModified: timeAtLaunch,
		mode:         dirMode,
		linkCount:    2,
	}

	childInode.dirTable = sortedmap.NewLLRBTree(sortedmap.CompareString, childInode)

	ok, err = childInode.dirTable.Put(dotDirTableEntryName, childInode.inodeNumber)
	if err != nil {
		globals.logger.Fatalf("childInode.dirTable.Put(dotDirTableEntryName, childInode.inodeNumber) failed %v", err)
	}
	if !ok {
		globals.logger.Fatalf("childInode.dirTable.Put(dotDirTableEntryName, childInode.inodeNumber) returned !ok")
	}

	ok, err = childInode.dirTable.Put(dotDotDirTableEntryName, childInode.inodeNumber)
	if err != nil {
		globals.logger.Fatalf("childInode.dirTable.Put(dotDotDirTableEntryName, childInode.inodeNumber) failed %v", err)
	}
	if !ok {
		globals.logger.Fatalf("childInode.dirTable.Put(dotDotDirTableEntryName, childInode.inodeNumber) returned !ok")
	}

	globals.inodeTable = append(globals.inodeTable, childInode)

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
			objectKey = *listObjectsV2OutputContentsElement.Key

			objectKeyCutPrefix, found = strings.CutPrefix(objectKey, globals.config.S3Prefix)
			if !found {
				log.Fatalf("strings.CutPrefix(\"%s\", globals.args.S3Prefix) returned !found", objectKey)
			}

			objectKeyCutPrefixSlice = strings.Split(objectKeyCutPrefix, "/")

			parentInode = globals.inodeTable[rootDirInodeNumber-1]

			for _, objectKeyCutPrefixSliceElement = range objectKeyCutPrefixSlice[:len(objectKeyCutPrefixSlice)-1] {
				childInodeNumberAsValue, ok, err = parentInode.dirTable.GetByKey(objectKeyCutPrefixSliceElement)
				if err != nil {
					log.Fatalf("parentInode.dirTable.GetByKey(objectKeyCutPrefixSliceElement) failed: %v", err)
				}

				if ok {
					childInodeNumber, ok = childInodeNumberAsValue.(uint64)
					if !ok {
						log.Fatalf("childInodeNumberAsValue.(uint64) returned !ok")
					}

					parentInode = globals.inodeTable[childInodeNumber-1]
				} else {
					childInode = &inodeStruct{
						inodeNumber:  uint64(len(globals.inodeTable) + 1),
						lastModified: timeAtLaunch,
						mode:         dirMode,
						linkCount:    2,
					}

					childInode.dirTable = sortedmap.NewLLRBTree(sortedmap.CompareString, childInode)

					ok, err = childInode.dirTable.Put(dotDirTableEntryName, childInode.inodeNumber)
					if err != nil {
						globals.logger.Fatalf("childInode.dirTable.Put(dotDirTableEntryName, childInode.inodeNumber) failed %v", err)
					}
					if !ok {
						globals.logger.Fatalf("childInode.dirTable.Put(dotDirTableEntryName, childInode.inodeNumber) returned !ok")
					}

					ok, err = childInode.dirTable.Put(dotDotDirTableEntryName, parentInode.inodeNumber)
					if err != nil {
						globals.logger.Fatalf("childInode.dirTable.Put(dotDotDirTableEntryName, parentInode.inodeNumber) failed %v", err)
					}
					if !ok {
						globals.logger.Fatalf("childInode.dirTable.Put(dotDotDirTableEntryName, parentInode.inodeNumber) returned !ok")
					}

					parentInode.linkCount++

					ok, err = parentInode.dirTable.Put(objectKeyCutPrefixSliceElement, childInode.inodeNumber)
					if err != nil {
						globals.logger.Fatalf("parentInode.dirTable.Put(objectKeyCutPrefixSliceElement, childInode.inodeNumber) failed %v", err)
					}
					if !ok {
						globals.logger.Fatalf("parentInode.dirTable.Put(objectKeyCutPrefixSliceElement, childInode.inodeNumber) returned !ok")
					}

					globals.inodeTable = append(globals.inodeTable, childInode)

					parentInode = childInode
				}
			}

			objectKeyCutPrefixSliceElement = objectKeyCutPrefixSlice[len(objectKeyCutPrefixSlice)-1]

			childInode = &inodeStruct{
				inodeNumber:  uint64(len(globals.inodeTable) + 1),
				size:         uint64(listObjectsV2OutputContentsElement.Size),
				lastModified: *listObjectsV2OutputContentsElement.LastModified,
				mode:         fileMode,
				linkCount:    1,
				objectKey:    objectKey,
			}

			fileInodeBlocks = childInode.size + (uint64(attrBlkSize) - 1)
			fileInodeBlocks /= uint64(attrBlkSize)

			globals.blocks += fileInodeBlocks

			ok, err = parentInode.dirTable.Put(objectKeyCutPrefixSliceElement, childInode.inodeNumber)
			if err != nil {
				globals.logger.Fatalf("parentInode.dirTable.Put(objectKeyCutPrefixSliceElement, childInode.inodeNumber) failed %v", err)
			}
			if !ok {
				globals.logger.Fatalf("parentInode.dirTable.Put(objectKeyCutPrefixSliceElement, childInode.inodeNumber) returned !ok")
			}

			globals.inodeTable = append(globals.inodeTable, childInode)
		}
	}

	if globals.config.FileCacheLines > 0 {
		globals.fileCacheDir, err = os.MkdirTemp("", fileCacheDirPattern)
		if err != nil {
			globals.logger.Fatalf("os.MkdirTemp(\"\", fileCacheDirPattern) failed: %v", err)
		}

		globals.fileCacheLRU = list.New()
		globals.fileCacheMap = make(map[cacheLineTagStruct]*fileCacheLineStruct)
	} else {
		globals.fileCacheDir = ""

		globals.fileCacheLRU = nil
		globals.fileCacheMap = nil
	}

	globals.ramCacheLRU = list.New()
	globals.ramCacheMap = make(map[cacheLineTagStruct]*ramCacheLineStruct)

	globals.errChan = make(chan error, 1)

	globals.volume = fission.NewVolume(path.Base(globals.config.MountPoint), globals.config.MountPoint, fuseSubtype, maxRead, maxWrite, false, false, &globals, globals.logger, globals.errChan)

	err = globals.volume.DoMount()
	if err != nil {
		globals.logger.Fatalf("globals.volume.DoMount() failed: %v", err)
	}

	signalChan = make(chan os.Signal, 1)
	signal.Notify(signalChan, unix.SIGINT, unix.SIGTERM, unix.SIGHUP)

	select {
	case _ = <-signalChan:
		// Normal termination due to one of the above registered signals
	case err = <-globals.errChan:
		// Unexpected exit of /dev/fuse read loop since it's before we call DoUnmount()
		globals.logger.Printf("unexpected exit of /dev/fuse read loop: %v", err)
	}

	err = globals.volume.DoUnmount()
	if nil != err {
		globals.logger.Fatalf("fission.DoUnmount() failed: %v", err)
	}

	if globals.fileCacheDir != "" {
		err = os.RemoveAll(globals.fileCacheDir)
		if err != nil {
			globals.logger.Fatalf("os.RemoveAll(globals.fileCacheDir) failed: %v", err)
		}
	}
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
