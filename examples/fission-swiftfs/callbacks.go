// Copyright (c) 2015-2023, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/NVIDIA/fission"
	"github.com/NVIDIA/sortedmap"
)

func fixAttr(attr *fission.Attr) {
	attr.Blocks = (attr.Size + uint64(attrBlkSize) - 1) / uint64(attrBlkSize)
	attr.BlkSize = attrBlkSize
}

func (fileInode *fileInodeStruct) ensureAttrInCache() {
	var (
		authToken                 string
		contentLength             uint64
		err                       error
		httpRequest               *http.Request
		httpResponse              *http.Response
		mTime                     time.Time
		mTimeNSec                 uint32
		mTimeSec                  uint64
		objectURL                 string
		retryAfterReAuthAttempted bool
	)

	fileInode.RLock()

	if nil != fileInode.cachedAttr {
		fileInode.RUnlock()
		return
	}

	fileInode.RUnlock()

	fileInode.Lock()

	if nil != fileInode.cachedAttr {
		fileInode.Unlock()
		return
	}

	objectURL = globals.config.ContainerURL + "/" + fileInode.objectName

	retryAfterReAuthAttempted = false

RetryAfterReAuth:

	httpRequest, err = http.NewRequest("HEAD", objectURL, nil)
	if nil != err {
		fmt.Printf("http.NewRequest(\"HEAD\", \"%s\", nil) failed: %v\n", objectURL, err)
		os.Exit(1)
	}

	httpRequest.Header["User-Agent"] = []string{httpUserAgent}

	authToken = fetchAuthToken()
	if "" != authToken {
		httpRequest.Header["X-Auth-Token"] = []string{authToken}
	}

	httpResponse, err = globals.httpClient.Do(httpRequest)
	if nil != err {
		fmt.Printf("globals.httpClient.Do(HEAD %s) failed: %v\n", objectURL, err)
		os.Exit(1)
	}

	_, err = ioutil.ReadAll(httpResponse.Body)
	if nil != err {
		fmt.Printf("ioutil.ReadAll(httpResponse.Body) failed: %v\n", err)
		os.Exit(1)
	}
	err = httpResponse.Body.Close()
	if nil != err {
		fmt.Printf("httpResponse.Body.Close() failed: %v\n", err)
		os.Exit(1)
	}

	if http.StatusUnauthorized == httpResponse.StatusCode {
		if retryAfterReAuthAttempted {
			fmt.Printf("Re-authorization failed - exiting\n")
			os.Exit(1)
		}

		forceReAuth()

		retryAfterReAuthAttempted = true

		goto RetryAfterReAuth
	}

	if (200 > httpResponse.StatusCode) || (299 < httpResponse.StatusCode) {
		fmt.Printf("globals.httpClient.Do(HEAD %s) returned unexpected Status: %s\n", objectURL, httpResponse.Status)
		os.Exit(1)
	}

	contentLength, err = strconv.ParseUint(httpResponse.Header.Get("Content-Length"), 10, 64)
	if nil != err {
		fmt.Printf("strconv.ParseUint(httpResponse.Header.Get(\"Content-Length\"), 10, 64) failed: %v\n", err)
		os.Exit(1)
	}

	mTime, err = time.Parse(time.RFC1123, httpResponse.Header.Get("Last-Modified"))
	if nil == err {
		mTimeSec, mTimeNSec = goTimeToUnixTime(mTime)
	} else {
		mTimeSec, mTimeNSec = goTimeToUnixTime(globals.startTime)
	}

	fileInode.cachedAttr = &fission.Attr{
		Ino:       fileInode.inodeNumber,
		Size:      contentLength,
		Blocks:    0,
		ATimeSec:  mTimeSec,
		MTimeSec:  mTimeSec,
		CTimeSec:  mTimeSec,
		ATimeNSec: mTimeNSec,
		MTimeNSec: mTimeNSec,
		CTimeNSec: mTimeNSec,
		Mode:      fileMode,
		NLink:     1,
		UID:       0,
		GID:       0,
		RDev:      0,
		BlkSize:   attrBlkSize,
		Padding:   0,
	}

	fixAttr(fileInode.cachedAttr)

	fileInode.Unlock()
}

func (dummy *globalsStruct) DoLookup(inHeader *fission.InHeader, lookupIn *fission.LookupIn) (lookupOut *fission.LookupOut, errno syscall.Errno) {
	var (
		dirEntry        *dirEntryStruct
		dirEntryAsValue sortedmap.Value
		err             error
		fileInode       *fileInodeStruct
		ok              bool
	)

	if 1 != inHeader.NodeID {
		errno = syscall.ENOENT
		return
	}

	dirEntryAsValue, ok, err = globals.rootDirMap.GetByKey(string(lookupIn.Name[:]))
	if nil != err {
		fmt.Printf("globals.rootDirMap.GetByKey(\"%s\") failed: %v\n", string(lookupIn.Name[:]), err)
		os.Exit(1)
	}
	if !ok {
		errno = syscall.ENOENT
		return
	}

	dirEntry, ok = dirEntryAsValue.(*dirEntryStruct)
	if !ok {
		fmt.Printf("dirEntryAsValue.(*dirEntryStruct) returned !ok\n")
		os.Exit(1)
	}

	fileInode = globals.fileInodeMap[dirEntry.inodeNumber]

	fileInode.ensureAttrInCache()

	lookupOut = &fission.LookupOut{
		EntryOut: fission.EntryOut{
			NodeID:         fileInode.cachedAttr.Ino,
			Generation:     0,
			EntryValidSec:  0,
			AttrValidSec:   0,
			EntryValidNSec: 0,
			AttrValidNSec:  0,
			Attr: fission.Attr{
				Ino:       fileInode.cachedAttr.Ino,
				Size:      fileInode.cachedAttr.Size,
				Blocks:    fileInode.cachedAttr.Blocks,
				ATimeSec:  fileInode.cachedAttr.ATimeSec,
				MTimeSec:  fileInode.cachedAttr.MTimeSec,
				CTimeSec:  fileInode.cachedAttr.CTimeSec,
				ATimeNSec: fileInode.cachedAttr.ATimeNSec,
				MTimeNSec: fileInode.cachedAttr.MTimeNSec,
				CTimeNSec: fileInode.cachedAttr.CTimeNSec,
				Mode:      fileInode.cachedAttr.Mode,
				NLink:     fileInode.cachedAttr.NLink,
				UID:       fileInode.cachedAttr.UID,
				GID:       fileInode.cachedAttr.GID,
				RDev:      fileInode.cachedAttr.RDev,
				BlkSize:   fileInode.cachedAttr.BlkSize,
				Padding:   fileInode.cachedAttr.Padding,
			},
		},
	}

	fixAttr(&lookupOut.EntryOut.Attr)

	errno = 0
	return
}

func (dummy *globalsStruct) DoForget(inHeader *fission.InHeader, forgetIn *fission.ForgetIn) {
	return
}

func (dummy *globalsStruct) DoGetAttr(inHeader *fission.InHeader, getAttrIn *fission.GetAttrIn) (getAttrOut *fission.GetAttrOut, errno syscall.Errno) {
	var (
		fileInode *fileInodeStruct
		inodeAttr *fission.Attr
		ok        bool
	)

	if 1 == inHeader.NodeID {
		inodeAttr = globals.rootDirAttr
	} else {
		fileInode, ok = globals.fileInodeMap[inHeader.NodeID]
		if !ok {
			errno = syscall.ENOENT
			return
		}

		fileInode.ensureAttrInCache()

		inodeAttr = fileInode.cachedAttr
	}

	getAttrOut = &fission.GetAttrOut{
		AttrValidSec:  0,
		AttrValidNSec: 0,
		Dummy:         0,
		Attr: fission.Attr{
			Ino:       inodeAttr.Ino,
			Size:      inodeAttr.Size,
			Blocks:    inodeAttr.Blocks,
			ATimeSec:  inodeAttr.ATimeSec,
			MTimeSec:  inodeAttr.MTimeSec,
			CTimeSec:  inodeAttr.CTimeSec,
			ATimeNSec: inodeAttr.ATimeNSec,
			MTimeNSec: inodeAttr.MTimeNSec,
			CTimeNSec: inodeAttr.CTimeNSec,
			Mode:      inodeAttr.Mode,
			NLink:     inodeAttr.NLink,
			UID:       inodeAttr.UID,
			GID:       inodeAttr.GID,
			RDev:      inodeAttr.RDev,
			BlkSize:   inodeAttr.BlkSize,
			Padding:   inodeAttr.Padding,
		},
	}

	fixAttr(&getAttrOut.Attr)

	errno = 0
	return
}

func (dummy *globalsStruct) DoSetAttr(inHeader *fission.InHeader, setAttrIn *fission.SetAttrIn) (setAttrOut *fission.SetAttrOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoReadLink(inHeader *fission.InHeader) (readLinkOut *fission.ReadLinkOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoSymLink(inHeader *fission.InHeader, symLinkIn *fission.SymLinkIn) (symLinkOut *fission.SymLinkOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoMkNod(inHeader *fission.InHeader, mkNodIn *fission.MkNodIn) (mkNodOut *fission.MkNodOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoMkDir(inHeader *fission.InHeader, mkDirIn *fission.MkDirIn) (mkDirOut *fission.MkDirOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoUnlink(inHeader *fission.InHeader, unlinkIn *fission.UnlinkIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoRmDir(inHeader *fission.InHeader, rmDirIn *fission.RmDirIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoRename(inHeader *fission.InHeader, renameIn *fission.RenameIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoLink(inHeader *fission.InHeader, linkIn *fission.LinkIn) (linkOut *fission.LinkOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoOpen(inHeader *fission.InHeader, openIn *fission.OpenIn) (openOut *fission.OpenOut, errno syscall.Errno) {
	var (
		fileInode *fileInodeStruct
		ok        bool
	)

	if 1 == inHeader.NodeID {
		errno = syscall.EINVAL
		return
	}

	fileInode, ok = globals.fileInodeMap[inHeader.NodeID]
	if !ok {
		errno = syscall.ENOENT
		return
	}

	fileInode.ensureAttrInCache()

	openOut = &fission.OpenOut{
		FH:        0,
		OpenFlags: fission.FOpenResponseDirectIO,
		Padding:   0,
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoRead(inHeader *fission.InHeader, readIn *fission.ReadIn) (readOut *fission.ReadOut, errno syscall.Errno) {
	var (
		authToken                  string
		cacheLine                  *cacheLineStruct
		cacheLineBufLimitOffset    uint64
		cacheLineBufStartOffset    uint64
		cacheLineObjectLimitOffset uint64
		cacheLineObjectStartOffset uint64
		cacheLineTag               cacheLineTagStruct
		err                        error
		fileCurrentOffset          uint64
		fileInode                  *fileInodeStruct
		fileLimitOffset            uint64
		httpRequest                *http.Request
		httpResponse               *http.Response
		objectOffsetStart          uint64
		objectOffsetLimit          uint64
		listElement                *list.Element
		objectURL                  string
		ok                         bool
		retryAfterReAuthAttempted  bool
	)

	fileInode, ok = globals.fileInodeMap[inHeader.NodeID]
	if !ok {
		errno = syscall.ENOENT
		return
	}

	fileInode.ensureAttrInCache()

	fileCurrentOffset = readIn.Offset
	if fileCurrentOffset > fileInode.cachedAttr.Size {
		fileCurrentOffset = fileInode.cachedAttr.Size
	}

	fileLimitOffset = fileCurrentOffset + uint64(readIn.Size)
	if fileLimitOffset > fileInode.cachedAttr.Size {
		fileLimitOffset = fileInode.cachedAttr.Size
	}

	objectURL = globals.config.ContainerURL + "/" + fileInode.objectName

	readOut = &fission.ReadOut{
		Data: make([]byte, 0, readIn.Size),
	}

	for fileCurrentOffset < fileLimitOffset {
		cacheLineTag.inodeNumber = fileInode.inodeNumber
		cacheLineTag.lineNumber = fileCurrentOffset / globals.config.CacheLineSize

		globals.Lock()

		cacheLine, ok = globals.readCacheMap[cacheLineTag]

		if ok {
			globals.readCacheLRU.MoveToBack(cacheLine.listElement)

			globals.Unlock()

			cacheLine.Wait()
		} else {
			for uint64(globals.readCacheLRU.Len()) >= globals.config.NumCacheLines {
				listElement = globals.readCacheLRU.Front()
				cacheLine, ok = listElement.Value.(*cacheLineStruct)
				if !ok {
					fmt.Printf("cacheLine, ok = listElement.Value.(*cacheLineStruct) returned !ok\n")
					os.Exit(1)
				}

				_ = globals.readCacheLRU.Remove(listElement)
				delete(globals.readCacheMap, cacheLine.tag)
			}

			cacheLine = &cacheLineStruct{
				tag: cacheLineTag,
				buf: nil,
			}

			cacheLine.Add(1)

			cacheLine.listElement = globals.readCacheLRU.PushBack(cacheLine)
			globals.readCacheMap[cacheLineTag] = cacheLine

			globals.Unlock()

			retryAfterReAuthAttempted = false

		RetryAfterReAuth:

			httpRequest, err = http.NewRequest("GET", objectURL, nil)
			if nil != err {
				fmt.Printf("http.NewRequest(\"GET\", \"%s\", nil) failed: %v\n", objectURL, err)
				os.Exit(1)
			}

			httpRequest.Header["User-Agent"] = []string{httpUserAgent}

			authToken = fetchAuthToken()
			if "" != authToken {
				httpRequest.Header["X-Auth-Token"] = []string{authToken}
			}

			objectOffsetStart = cacheLine.tag.lineNumber * globals.config.CacheLineSize

			objectOffsetLimit = objectOffsetStart + globals.config.CacheLineSize
			if objectOffsetLimit > fileInode.cachedAttr.Size {
				objectOffsetLimit = fileInode.cachedAttr.Size
			}

			httpRequest.Header["Range"] = []string{fmt.Sprintf("bytes=%d-%d", objectOffsetStart, objectOffsetLimit-1)}

			httpResponse, err = globals.httpClient.Do(httpRequest)
			if nil != err {
				fmt.Printf("globals.httpClient.Do(GET %s) failed: %v\n", objectURL, err)
				os.Exit(1)
			}

			cacheLine.buf, err = ioutil.ReadAll(httpResponse.Body)
			if nil != err {
				fmt.Printf("ioutil.ReadAll(httpResponse.Body) failed: %v\n", err)
				os.Exit(1)
			}
			err = httpResponse.Body.Close()
			if nil != err {
				fmt.Printf("httpResponse.Body.Close() failed: %v\n", err)
				os.Exit(1)
			}

			if http.StatusUnauthorized == httpResponse.StatusCode {
				if retryAfterReAuthAttempted {
					fmt.Printf("Re-authorization failed - exiting\n")
					os.Exit(1)
				}

				forceReAuth()

				retryAfterReAuthAttempted = true

				goto RetryAfterReAuth
			}

			if (200 > httpResponse.StatusCode) || (299 < httpResponse.StatusCode) {
				fmt.Printf("globals.httpClient.Do(GET %s) returned unexpected Status: %s\n", objectURL, httpResponse.Status)
				os.Exit(1)
			}

			cacheLine.Done()
		}

		cacheLineObjectStartOffset = cacheLine.tag.lineNumber * globals.config.CacheLineSize
		cacheLineObjectLimitOffset = cacheLineObjectStartOffset + uint64(len(cacheLine.buf))

		cacheLineBufStartOffset = fileCurrentOffset - cacheLineObjectStartOffset

		if cacheLineObjectLimitOffset > fileLimitOffset {
			cacheLineBufLimitOffset = cacheLineBufStartOffset + (fileLimitOffset - fileCurrentOffset)
		} else {
			cacheLineBufLimitOffset = uint64(len(cacheLine.buf))
		}

		readOut.Data = append(readOut.Data, cacheLine.buf[cacheLineBufStartOffset:cacheLineBufLimitOffset]...)

		fileCurrentOffset += cacheLineBufLimitOffset - cacheLineBufStartOffset
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoWrite(inHeader *fission.InHeader, writeIn *fission.WriteIn) (writeOut *fission.WriteOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoStatFS(inHeader *fission.InHeader) (statFSOut *fission.StatFSOut, errno syscall.Errno) {
	statFSOut = &fission.StatFSOut{
		KStatFS: fission.KStatFS{
			Blocks:  0,
			BFree:   0,
			BAvail:  0,
			Files:   0,
			FFree:   0,
			BSize:   0,
			NameLen: 0,
			FRSize:  0,
			Padding: 0,
			Spare:   [6]uint32{0, 0, 0, 0, 0, 0},
		},
	}

	// TODO: Fill in the StatFSOut.KStatFS above correctly

	errno = 0
	return
}

func (dummy *globalsStruct) DoRelease(inHeader *fission.InHeader, releaseIn *fission.ReleaseIn) (errno syscall.Errno) {
	errno = 0
	return
}

func (dummy *globalsStruct) DoFSync(inHeader *fission.InHeader, fSyncIn *fission.FSyncIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoSetXAttr(inHeader *fission.InHeader, setXAttrIn *fission.SetXAttrIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoGetXAttr(inHeader *fission.InHeader, getXAttrIn *fission.GetXAttrIn) (getXAttrOut *fission.GetXAttrOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoListXAttr(inHeader *fission.InHeader, listXAttrIn *fission.ListXAttrIn) (listXAttrOut *fission.ListXAttrOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoRemoveXAttr(inHeader *fission.InHeader, removeXAttrIn *fission.RemoveXAttrIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoFlush(inHeader *fission.InHeader, flushIn *fission.FlushIn) (errno syscall.Errno) {
	errno = 0
	return
}

func (dummy *globalsStruct) DoInit(inHeader *fission.InHeader, initIn *fission.InitIn) (initOut *fission.InitOut, errno syscall.Errno) {
	initOut = &fission.InitOut{
		Major:                initIn.Major,
		Minor:                initIn.Minor,
		MaxReadAhead:         initIn.MaxReadAhead,
		Flags:                initOutFlagsReadOnly,
		MaxBackground:        initOutMaxBackgound,
		CongestionThreshhold: initOutCongestionThreshhold,
		MaxWrite:             maxWrite,
		TimeGran:             0, // accept default
		MaxPages:             maxPages,
		MapAlignment:         0, // accept default
		Flags2:               0,
		Unused:               [7]uint32{0, 0, 0, 0, 0, 0, 0},
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoOpenDir(inHeader *fission.InHeader, openDirIn *fission.OpenDirIn) (openDirOut *fission.OpenDirOut, errno syscall.Errno) {
	if 1 != inHeader.NodeID {
		errno = syscall.ENOENT
		return
	}

	openDirOut = &fission.OpenDirOut{
		FH:        0,
		OpenFlags: 0,
		Padding:   0,
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoReadDir(inHeader *fission.InHeader, readDirIn *fission.ReadDirIn) (readDirOut *fission.ReadDirOut, errno syscall.Errno) {
	var (
		dirEntNameLenAligned    uint32
		dirEntSize              uint32
		dirEntry                *dirEntryStruct
		dirEntryAsValue         sortedmap.Value
		dirEntryIndex           int
		dirEntryNameAsByteSlice []byte
		dirEntryType            uint32
		err                     error
		numDirEntries           int
		ok                      bool
		totalSize               uint32
	)

	if 1 != inHeader.NodeID {
		errno = syscall.ENOENT
		return
	}

	numDirEntries, err = globals.rootDirMap.Len()
	if nil != err {
		fmt.Printf("globals.rootDirMap.Len() failed: %v\n", err)
		os.Exit(1)
	}

	readDirOut = &fission.ReadDirOut{
		DirEnt: make([]fission.DirEnt, 0, numDirEntries),
	}

	totalSize = 0

	for dirEntryIndex = int(readDirIn.Offset); dirEntryIndex < numDirEntries; dirEntryIndex++ {
		_, dirEntryAsValue, ok, err = globals.rootDirMap.GetByIndex(dirEntryIndex)
		if nil != err {
			fmt.Printf("globals.rootDirMap.GetByIndex(%d) failed: %v\n", dirEntryIndex, err)
			os.Exit(1)
		}
		if nil != err {
			fmt.Printf("globals.rootDirMap.GetByIndex(%d) returned !ok\n", dirEntryIndex)
			os.Exit(1)
		}

		dirEntry, ok = dirEntryAsValue.(*dirEntryStruct)
		if !ok {
			fmt.Printf("dirEntryAsValue.(*dirEntryStruct) returned !ok\n")
			os.Exit(1)
		}

		dirEntryNameAsByteSlice = []byte(dirEntry.name)

		dirEntNameLenAligned = (uint32(len(dirEntryNameAsByteSlice)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
		dirEntSize = fission.DirEntFixedPortionSize + dirEntNameLenAligned

		if (totalSize + dirEntSize) > readDirIn.Size {
			errno = 0
			return
		}

		if dirEntry.isRootDir {
			dirEntryType = syscall.S_IFDIR
		} else {
			dirEntryType = syscall.S_IFREG
		}

		readDirOut.DirEnt = append(readDirOut.DirEnt, fission.DirEnt{
			Ino:     dirEntry.inodeNumber,
			Off:     uint64(dirEntryIndex) + 1,
			NameLen: uint32(len(dirEntryNameAsByteSlice)), // unnecessary
			Type:    dirEntryType,
			Name:    dirEntryNameAsByteSlice,
		})

		totalSize += dirEntSize
	}

	if 0 == len(readDirOut.DirEnt) {
		errno = syscall.ENOENT
	} else {
		errno = 0
	}

	return
}

func (dummy *globalsStruct) DoReleaseDir(inHeader *fission.InHeader, releaseDirIn *fission.ReleaseDirIn) (errno syscall.Errno) {
	if 1 != inHeader.NodeID {
		errno = syscall.EINVAL
		return
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoFSyncDir(inHeader *fission.InHeader, fSyncDirIn *fission.FSyncDirIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoGetLK(inHeader *fission.InHeader, getLKIn *fission.GetLKIn) (getLKOut *fission.GetLKOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoSetLK(inHeader *fission.InHeader, setLKIn *fission.SetLKIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoSetLKW(inHeader *fission.InHeader, setLKWIn *fission.SetLKWIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoAccess(inHeader *fission.InHeader, accessIn *fission.AccessIn) (errno syscall.Errno) {
	var (
		fileInode *fileInodeStruct
		ok        bool
	)

	if 0 != (accessIn.Mask & accessWOK) {
		errno = syscall.EACCES
	} else {
		if 1 == inHeader.NodeID {
			errno = 0
		} else {
			fileInode, ok = globals.fileInodeMap[inHeader.NodeID]
			if ok {
				fileInode.ensureAttrInCache()

				if 0 != (accessIn.Mask & accessXOK) {
					errno = syscall.EACCES
				} else {
					errno = 0
				}
			} else {
				errno = syscall.ENOENT
			}
		}
	}

	return
}

func (dummy *globalsStruct) DoCreate(inHeader *fission.InHeader, createIn *fission.CreateIn) (createOut *fission.CreateOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoInterrupt(inHeader *fission.InHeader, interruptIn *fission.InterruptIn) {
	return
}

func (dummy *globalsStruct) DoBMap(inHeader *fission.InHeader, bMapIn *fission.BMapIn) (bMapOut *fission.BMapOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoDestroy(inHeader *fission.InHeader) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoPoll(inHeader *fission.InHeader, pollIn *fission.PollIn) (pollOut *fission.PollOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoBatchForget(inHeader *fission.InHeader, batchForgetIn *fission.BatchForgetIn) {
	return
}

func (dummy *globalsStruct) DoFAllocate(inHeader *fission.InHeader, fAllocateIn *fission.FAllocateIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoReadDirPlus(inHeader *fission.InHeader, readDirPlusIn *fission.ReadDirPlusIn) (readDirPlusOut *fission.ReadDirPlusOut, errno syscall.Errno) {
	var (
		asyncFillAttrWG         sync.WaitGroup
		dirEntNameLenAligned    uint32
		dirEntSize              uint32
		dirEntry                *dirEntryStruct
		dirEntryAsValue         sortedmap.Value
		dirEntryIndex           int
		dirEntryNameAsByteSlice []byte
		err                     error
		fileInode               *fileInodeStruct
		numDirEntries           int
		ok                      bool
		totalSize               uint32
	)

	if 1 != inHeader.NodeID {
		errno = syscall.ENOENT
		return
	}

	numDirEntries, err = globals.rootDirMap.Len()
	if nil != err {
		fmt.Printf("globals.rootDirMap.Len() failed: %v\n", err)
		os.Exit(1)
	}

	readDirPlusOut = &fission.ReadDirPlusOut{
		DirEntPlus: make([]fission.DirEntPlus, 0, numDirEntries),
	}

	totalSize = 0

	for dirEntryIndex = int(readDirPlusIn.Offset); dirEntryIndex < numDirEntries; dirEntryIndex++ {
		_, dirEntryAsValue, ok, err = globals.rootDirMap.GetByIndex(dirEntryIndex)
		if nil != err {
			fmt.Printf("globals.rootDirMap.GetByIndex(%d) failed: %v\n", dirEntryIndex, err)
			os.Exit(1)
		}
		if nil != err {
			fmt.Printf("globals.rootDirMap.GetByIndex(%d) returned !ok\n", dirEntryIndex)
			os.Exit(1)
		}

		dirEntry, ok = dirEntryAsValue.(*dirEntryStruct)
		if !ok {
			fmt.Printf("dirEntryAsValue.(*dirEntryStruct) returned !ok\n")
			os.Exit(1)
		}

		dirEntryNameAsByteSlice = []byte(dirEntry.name)

		dirEntNameLenAligned = (uint32(len(dirEntryNameAsByteSlice)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
		dirEntSize = fission.DirEntPlusFixedPortionSize + dirEntNameLenAligned

		if (totalSize + dirEntSize) > readDirPlusIn.Size {
			errno = 0
			return
		}

		totalSize += dirEntSize

		if dirEntry.isRootDir {
			readDirPlusOut.DirEntPlus = append(readDirPlusOut.DirEntPlus, fission.DirEntPlus{
				EntryOut: fission.EntryOut{
					NodeID:         1,
					Generation:     0,
					EntryValidSec:  0,
					AttrValidSec:   0,
					EntryValidNSec: 0,
					AttrValidNSec:  0,
					Attr: fission.Attr{
						Ino:       globals.rootDirAttr.Ino,
						Size:      globals.rootDirAttr.Size,
						Blocks:    globals.rootDirAttr.Blocks,
						ATimeSec:  globals.rootDirAttr.ATimeSec,
						MTimeSec:  globals.rootDirAttr.MTimeSec,
						CTimeSec:  globals.rootDirAttr.CTimeSec,
						ATimeNSec: globals.rootDirAttr.ATimeNSec,
						MTimeNSec: globals.rootDirAttr.MTimeNSec,
						CTimeNSec: globals.rootDirAttr.CTimeNSec,
						Mode:      globals.rootDirAttr.Mode,
						NLink:     globals.rootDirAttr.NLink,
						UID:       globals.rootDirAttr.UID,
						GID:       globals.rootDirAttr.GID,
						RDev:      globals.rootDirAttr.RDev,
						BlkSize:   globals.rootDirAttr.BlkSize,
						Padding:   globals.rootDirAttr.Padding,
					},
				},
				DirEnt: fission.DirEnt{
					Ino:     1,
					Off:     uint64(dirEntryIndex) + 1,
					NameLen: uint32(len(dirEntryNameAsByteSlice)), // unnecessary
					Type:    syscall.S_IFDIR,
					Name:    dirEntryNameAsByteSlice,
				},
			})

			fixAttr(&readDirPlusOut.DirEntPlus[len(readDirPlusOut.DirEntPlus)-1].EntryOut.Attr)
		} else {
			readDirPlusOut.DirEntPlus = append(readDirPlusOut.DirEntPlus, fission.DirEntPlus{
				EntryOut: fission.EntryOut{
					NodeID:         dirEntry.inodeNumber,
					Generation:     0,
					EntryValidSec:  0,
					AttrValidSec:   0,
					EntryValidNSec: 0,
					AttrValidNSec:  0,
				},
				DirEnt: fission.DirEnt{
					Ino:     1,
					Off:     uint64(dirEntryIndex) + 1,
					NameLen: uint32(len(dirEntryNameAsByteSlice)), // unnecessary
					Type:    syscall.S_IFREG,
					Name:    dirEntryNameAsByteSlice,
				},
			})

			fileInode, ok = globals.fileInodeMap[dirEntry.inodeNumber]
			if !ok {
				fmt.Printf("globals.fileInodeMap[%v] returned !ok\n", dirEntry.inodeNumber)
				os.Exit(1)
			}

			asyncFillAttrWG.Add(1)

			go func(fileInode *fileInodeStruct, attr *fission.Attr, wg *sync.WaitGroup) {
				fileInode.ensureAttrInCache()

				attr.Ino = fileInode.cachedAttr.Ino
				attr.Size = fileInode.cachedAttr.Size
				attr.Blocks = fileInode.cachedAttr.Blocks
				attr.ATimeSec = fileInode.cachedAttr.ATimeSec
				attr.MTimeSec = fileInode.cachedAttr.MTimeSec
				attr.CTimeSec = fileInode.cachedAttr.CTimeSec
				attr.ATimeNSec = fileInode.cachedAttr.ATimeNSec
				attr.MTimeNSec = fileInode.cachedAttr.MTimeNSec
				attr.CTimeNSec = fileInode.cachedAttr.CTimeNSec
				attr.Mode = fileInode.cachedAttr.Mode
				attr.NLink = fileInode.cachedAttr.NLink
				attr.UID = fileInode.cachedAttr.UID
				attr.GID = fileInode.cachedAttr.GID
				attr.RDev = fileInode.cachedAttr.RDev
				attr.BlkSize = fileInode.cachedAttr.BlkSize
				attr.Padding = fileInode.cachedAttr.Padding

				fixAttr(attr)

				wg.Done()
			}(fileInode, &readDirPlusOut.DirEntPlus[len(readDirPlusOut.DirEntPlus)-1].EntryOut.Attr, &asyncFillAttrWG)
		}
	}

	asyncFillAttrWG.Wait()

	if 0 == len(readDirPlusOut.DirEntPlus) {
		errno = syscall.ENOENT
	} else {
		errno = 0
	}

	return
}

func (dummy *globalsStruct) DoRename2(inHeader *fission.InHeader, rename2In *fission.Rename2In) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (dummy *globalsStruct) DoLSeek(inHeader *fission.InHeader, lSeekIn *fission.LSeekIn) (lSeekOut *fission.LSeekOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}
