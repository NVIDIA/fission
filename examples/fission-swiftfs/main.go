// Copyright (c) 2015-2021, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/NVIDIA/sortedmap"
	"golang.org/x/sys/unix"

	"github.com/NVIDIA/fission"
)

const (
	fuseSubtype   = "fission-swiftfs"
	httpUserAgent = "fission-swiftfs"

	initOutFlagsReadOnly = uint32(0) |
		fission.InitFlagsAsyncRead |
		fission.InitFlagsFileOps |
		fission.InitFlagsParallelDirops |
		fission.InitFlagsMaxPages |
		fission.InitFlagsNoOpendirSupport |
		fission.InitFlagsExplicitInvalData

	initOutMaxBackgound         = uint16(100)
	initOutCongestionThreshhold = uint16(0)
	initOutMaxWrite             = uint32(128 * 1024) // 128KiB... the max write size in Linux FUSE at this time

	attrBlkSize = uint32(4096)

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
)

type dirEntryStruct struct {
	name        string // Either ".", "..", or fileInodeStruct.objectName
	inodeNumber uint64 // If name is "." or "..", then 1; otherwise fileInodeStruct.inodeNumber
	isRootDir   bool   // if name is "." or "..", then true; othersize false
}

type fileInodeStruct struct {
	sync.RWMutex
	objectName  string // No hard link support... so this is 1:1 mapped
	inodeNumber uint64 // Will match cachedStat.Ino if available
	cachedStat  *fission.Attr
}

type configStruct struct {
	MountPoint              string
	ContainerURL            string
	AuthToken               string
	SwiftTimeout            string
	SwiftConnectionPoolSize uint64
	NumCacheLines           uint64
	CacheLineSize           uint64
}

type globalsStruct struct {
	sync.RWMutex
	config       *configStruct
	volumeName   string
	swiftTimeout time.Duration
	httpClient   *http.Client
	rootDirAttr  *fission.Attr
	rootDirMap   sortedmap.LLRBTree          // key=dirEntryStruct.name; value=*dirEntryStruct
	fileInodeMap map[uint64]*fileInodeStruct // key=fileInodeStruct.inodeNumber; value=*fileInodeStruct
	logger       *log.Logger
	errChan      chan error
	volume       fission.Volume
}

var globals globalsStruct

func main() {
	var (
		configFileContent []byte
		customTransport   *http.Transport
		defaultTransport  *http.Transport
		dirEntry          *dirEntryStruct
		err               error
		fileInode         *fileInodeStruct
		httpRequest       *http.Request
		httpResponse      *http.Response
		httpResponseBody  []byte
		lastInodeNumber   uint64
		objectName        string
		objectNameList    []string
		ok                bool
		rootDirMTime      time.Time
		rootDirMTimeNSec  uint32
		rootDirMTimeSec   uint64
		signalChan        chan os.Signal
	)

	if 2 != len(os.Args) {
		fmt.Printf("Usage: %s <configFile>\n", os.Args[0])
		fmt.Printf("  where <configFile> is a JSON object of the form:\n")
		fmt.Printf("    {\n")
		fmt.Printf("      \"MountPoint\"              : \"<path to empty dir where the file system will be FUSE mounted>\",\n")
		fmt.Printf("      \"ContainerURL\"            : \"<storage URL with account name set as desired and container name appended>\",\n")
		fmt.Printf("      \"AuthToken\"               : \"<auth token as returned during Swift Auth>\",\n")
		fmt.Printf("      \"SwiftTimeout\"            : \"<time.Duration string>\",\n")
		fmt.Printf("      \"SwiftConnectionPoolSize\" : <max # of connections to Swift>,\n")
		fmt.Printf("      \"NumCacheLines\"           : <number of cache lines to enable>,\n")
		fmt.Printf("      \"CacheLineSize\"           : <(max) size of each cache line>\n")
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

	globals.volumeName = path.Base(globals.config.MountPoint)

	globals.swiftTimeout, err = time.ParseDuration(globals.config.SwiftTimeout)
	if nil != err {
		fmt.Printf("time.ParseDuration(\"%s\") failed: %v\n", globals.config.SwiftTimeout, err)
		os.Exit(1)
	}

	defaultTransport, ok = http.DefaultTransport.(*http.Transport)
	if !ok {
		fmt.Printf("http.DefaultTransport.(*http.Transport) returned !ok\n")
		os.Exit(1)
	}

	customTransport = &http.Transport{ // Up-to-date as of Golang 1.11
		Proxy:                  defaultTransport.Proxy,
		DialContext:            defaultTransport.DialContext,
		Dial:                   defaultTransport.Dial,
		DialTLS:                defaultTransport.DialTLS,
		TLSClientConfig:        defaultTransport.TLSClientConfig,
		TLSHandshakeTimeout:    globals.swiftTimeout,
		DisableKeepAlives:      false,
		DisableCompression:     defaultTransport.DisableCompression,
		MaxIdleConns:           int(globals.config.SwiftConnectionPoolSize),
		MaxIdleConnsPerHost:    int(globals.config.SwiftConnectionPoolSize),
		MaxConnsPerHost:        int(globals.config.SwiftConnectionPoolSize),
		IdleConnTimeout:        globals.swiftTimeout,
		ResponseHeaderTimeout:  globals.swiftTimeout,
		ExpectContinueTimeout:  globals.swiftTimeout,
		TLSNextProto:           defaultTransport.TLSNextProto,
		ProxyConnectHeader:     defaultTransport.ProxyConnectHeader,
		MaxResponseHeaderBytes: defaultTransport.MaxResponseHeaderBytes,
	}

	globals.httpClient = &http.Client{
		Transport: customTransport,
		Timeout:   globals.swiftTimeout,
	}

	httpRequest, err = http.NewRequest("GET", globals.config.ContainerURL, nil)
	if nil != err {
		fmt.Printf("http.NewRequest(\"GET\", \"%s\", nil) failed: %v\n", globals.config.ContainerURL, err)
		os.Exit(1)
	}

	httpRequest.Header["User-Agent"] = []string{httpUserAgent}

	if "" != globals.config.AuthToken {
		httpRequest.Header["X-Auth-Token"] = []string{globals.config.AuthToken}
	}

	httpResponse, err = globals.httpClient.Do(httpRequest)
	if nil != err {
		fmt.Printf("globals.httpClient.Do(GET %s) failed: %v\n", globals.config.ContainerURL, err)
		os.Exit(1)
	}

	httpResponseBody, err = ioutil.ReadAll(httpResponse.Body)
	if nil != err {
		fmt.Printf("ioutil.ReadAll(httpResponse.Body) failed: %v\n", err)
		os.Exit(1)
	}
	err = httpResponse.Body.Close()
	if nil != err {
		fmt.Printf("httpResponse.Body.Close() failed: %v\n", err)
		os.Exit(1)
	}

	if (200 > httpResponse.StatusCode) || (299 < httpResponse.StatusCode) {
		fmt.Printf("globals.httpClient.Do(GET %s) returned unexpected Status: %s\n", globals.config.ContainerURL, httpResponse.Status)
		os.Exit(1)
	}

	rootDirMTime, err = time.Parse(time.RFC1123, httpResponse.Header.Get("Last-Modified"))
	if nil == err {
		rootDirMTimeSec, rootDirMTimeNSec = goTimeToUnixTime(rootDirMTime)
	} else {
		rootDirMTimeSec, rootDirMTimeNSec = goTimeToUnixTime(time.Now())
	}

	globals.rootDirAttr = &fission.Attr{
		Ino:       1,
		Size:      0,
		Blocks:    0,
		ATimeSec:  rootDirMTimeSec,
		MTimeSec:  rootDirMTimeSec,
		CTimeSec:  rootDirMTimeSec,
		ATimeNSec: rootDirMTimeNSec,
		MTimeNSec: rootDirMTimeNSec,
		CTimeNSec: rootDirMTimeNSec,
		Mode:      dirMode,
		NLink:     2,
		UID:       0,
		GID:       0,
		RDev:      0,
		BlkSize:   attrBlkSize,
		Padding:   0,
	}

	fixAttr(globals.rootDirAttr)

	globals.rootDirMap = sortedmap.NewLLRBTree(sortedmap.CompareString, &globals)

	dirEntry = &dirEntryStruct{
		name:        ".",
		inodeNumber: 1,
		isRootDir:   true,
	}

	ok, err = globals.rootDirMap.Put(dirEntry.name, dirEntry)
	if nil != err {
		fmt.Printf("globals.rootDirMap.Put(\"%s\", %#v) failed: %v\n", dirEntry.name, dirEntry, err)
		os.Exit(1)
	}
	if !ok {
		fmt.Printf("globals.rootDirMap.Put(\"%s\", %#v) returned !ok\n", dirEntry.name, dirEntry)
		os.Exit(1)
	}

	dirEntry = &dirEntryStruct{
		name:        "..",
		inodeNumber: 1,
		isRootDir:   true,
	}

	ok, err = globals.rootDirMap.Put(dirEntry.name, dirEntry)
	if nil != err {
		fmt.Printf("globals.rootDirMap.Put(\"%s\", %#v) failed: %v\n", dirEntry.name, dirEntry, err)
		os.Exit(1)
	}
	if !ok {
		fmt.Printf("globals.rootDirMap.Put(\"%s\", %#v) returned !ok\n", dirEntry.name, dirEntry)
		os.Exit(1)
	}

	objectNameList = strings.Split(string(httpResponseBody[:]), "\n")
	if 0 < len(objectNameList) {
		if "" == objectNameList[len(objectNameList)-1] {
			objectNameList = objectNameList[:len(objectNameList)-1]
		}
	}

	lastInodeNumber = 1

	globals.fileInodeMap = make(map[uint64]*fileInodeStruct)

	for _, objectName = range objectNameList {
		lastInodeNumber++

		dirEntry = &dirEntryStruct{
			name:        objectName,
			inodeNumber: lastInodeNumber,
			isRootDir:   false,
		}

		ok, err = globals.rootDirMap.Put(dirEntry.name, dirEntry)
		if nil != err {
			fmt.Printf("globals.rootDirMap.Put(\"%s\", %#v) failed: %v\n", dirEntry.name, dirEntry, err)
			os.Exit(1)
		}
		if !ok {
			fmt.Printf("globals.rootDirMap.Put(\"%s\", %#v) returned !ok\n", dirEntry.name, dirEntry)
			os.Exit(1)
		}

		fileInode = &fileInodeStruct{
			objectName:  dirEntry.name,
			inodeNumber: dirEntry.inodeNumber,
			cachedStat:  nil,
		}

		globals.fileInodeMap[fileInode.inodeNumber] = fileInode
	}

	globals.logger = log.New(os.Stdout, "", log.Ldate|log.Ltime) // |log.Lmicroseconds|log.Lshortfile

	globals.errChan = make(chan error, 1)

	// UNDO ??? globals.alreadyLoggedIgnoring.setAttrInValidFH = false

	globals.volume = fission.NewVolume(globals.volumeName, globals.config.MountPoint, fuseSubtype, initOutMaxWrite, &globals, globals.logger, globals.errChan)

	err = globals.volume.DoMount()
	if nil != err {
		globals.logger.Printf("fission.DoMount() failed: %v", err)
		os.Exit(1)
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
		globals.logger.Printf("fission.DoUnmount() failed: %v", err)
		os.Exit(1)
	}
}

func goTimeToUnixTime(goTime time.Time) (unixTimeSec uint64, unixTimeNSec uint32) {
	var (
		unixTime uint64
	)
	unixTime = uint64(goTime.UnixNano())
	unixTimeSec = unixTime / 1e9
	unixTimeNSec = uint32(unixTime - (unixTimeSec * 1e9))
	return
}

func cloneByteSlice(inBuf []byte) (outBuf []byte) {
	outBuf = make([]byte, len(inBuf))
	if 0 != len(inBuf) {
		_ = copy(outBuf, inBuf)
	}
	return
}

func (dummy *globalsStruct) DumpKey(key sortedmap.Key) (keyAsString string, err error) {
	var (
		ok bool
	)

	keyAsString, ok = key.(string)
	if ok {
		err = nil
	} else {
		err = fmt.Errorf("keyAsString, ok = key.(string) returned !ok")
	}

	return
}

func (dummy *globalsStruct) DumpValue(value sortedmap.Value) (valueAsString string, err error) {
	var (
		ok              bool
		valueAsDirEntry *dirEntryStruct
	)

	valueAsDirEntry, ok = value.(*dirEntryStruct)
	if ok {
		valueAsString = fmt.Sprintf("%#v", valueAsDirEntry)
		err = nil
	} else {
		err = fmt.Errorf("valueAsDirEntry, ok = key.(*dirEntryStruct) returned !ok")
	}

	return
}
