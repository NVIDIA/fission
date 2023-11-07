// Copyright (c) 2023, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/NVIDIA/fission"
	"github.com/NVIDIA/sortedmap"
)

func goTimeToUnixTime(goTime time.Time) (unixTimeSec uint64, unixTimeNSec uint32) {
	var (
		unixTime uint64
	)
	unixTime = uint64(goTime.UnixNano())
	unixTimeSec = unixTime / 1e9
	unixTimeNSec = uint32(unixTime - (unixTimeSec * 1e9))
	return
}

func populateAttr(attr *fission.Attr, inode *inodeStruct) {
	if inode.linkCount > uint64(math.MaxUint32) {
		globals.logger.Fatalf("inode.linkCount [0x%08X] > uint64(math.MaxUint32) [0x%04X]", inode.linkCount, math.MaxUint32)
	}

	attr.Ino = inode.inodeNumber
	attr.ATimeSec, attr.ATimeNSec = goTimeToUnixTime(inode.lastModified)
	attr.MTimeSec, attr.MTimeNSec = attr.ATimeSec, attr.ATimeNSec
	attr.CTimeSec, attr.CTimeNSec = attr.ATimeSec, attr.ATimeNSec
	attr.Mode = inode.mode
	attr.NLink = uint32(inode.linkCount)
	attr.UID = attrUID
	attr.GID = attrGID
	attr.RDev = attrRDev
	attr.Padding = 0

	if (inode.mode & syscall.S_IFMT) == syscall.S_IFDIR {
		attr.Size = 0
		attr.Blocks = 0
		attr.BlkSize = 0
	} else if (inode.mode & syscall.S_IFMT) == syscall.S_IFREG {
		attr.Size = inode.size
		attr.Blocks = attr.Size + (uint64(attrBlkSize) - 1)
		attr.Blocks /= uint64(attrBlkSize)
		attr.BlkSize = attrBlkSize
	} else {
		globals.logger.Fatalf("(inode.mode & syscall.S_IFMT) [0x%04X] must be either syscall.S_ISDIR [0x%04X] or syscall.S_ISREG [0x%04X]", (inode.mode & syscall.S_IFMT), syscall.S_IFDIR, syscall.S_IFDIR)
	}
}

func fetchInode(inodeNumber uint64) (inode *inodeStruct) {
	if (inodeNumber == 0) || (inodeNumber > uint64(len(globals.inodeTable))) {
		inode = nil
	} else {
		inode = globals.inodeTable[inodeNumber-1]
	}

	return
}

func (dummy *globalsStruct) DoLookup(inHeader *fission.InHeader, lookupIn *fission.LookupIn) (lookupOut *fission.LookupOut, errno syscall.Errno) {
	var (
		dirEntryAsValue     sortedmap.Value
		dirEntryInode       *inodeStruct
		dirEntryInodeNumber uint64
		dirInode            *inodeStruct
		err                 error
		ok                  bool
	)

	dirInode = fetchInode(inHeader.NodeID)
	if dirInode == nil {
		errno = syscall.ENOENT
		return
	}

	dirEntryAsValue, ok, err = dirInode.dirTable.GetByKey(string(lookupIn.Name[:]))
	if nil != err {
		globals.logger.Fatalf("globals.rootDirMap.GetByKey(\"%s\") failed: %v\n", string(lookupIn.Name[:]), err)
	}
	if !ok {
		errno = syscall.ENOENT
		return
	}

	dirEntryInodeNumber, ok = dirEntryAsValue.(uint64)
	if !ok {
		globals.logger.Fatalf("dirEntryAsValue.(uint64) returned !ok\n")
	}

	dirEntryInode = fetchInode(dirEntryInodeNumber)
	if dirEntryInode == nil {
		globals.logger.Fatalf("fetchInode(dirEntryInodeNumber) returned nil")
	}

	lookupOut = &fission.LookupOut{
		EntryOut: fission.EntryOut{
			NodeID:         dirEntryInode.inodeNumber,
			Generation:     entryGeneration,
			EntryValidSec:  entryValidSec,
			AttrValidSec:   attrValidSec,
			EntryValidNSec: entryValidNSec,
			AttrValidNSec:  attrValidNSec,
		},
	}

	populateAttr(&lookupOut.EntryOut.Attr, dirEntryInode)

	errno = 0
	return
}

func (dummy *globalsStruct) DoForget(inHeader *fission.InHeader, forgetIn *fission.ForgetIn) {
	return
}

func (dummy *globalsStruct) DoGetAttr(inHeader *fission.InHeader, getAttrIn *fission.GetAttrIn) (getAttrOut *fission.GetAttrOut, errno syscall.Errno) {
	var (
		inode *inodeStruct
	)

	inode = fetchInode(inHeader.NodeID)
	if inode == nil {
		errno = syscall.ENOENT
		return
	}

	getAttrOut = &fission.GetAttrOut{
		AttrValidSec:  attrValidSec,
		AttrValidNSec: attrValidNSec,
		Dummy:         0,
	}

	populateAttr(&getAttrOut.Attr, inode)

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
		fileInode *inodeStruct
	)

	fileInode = fetchInode(inHeader.NodeID)
	if fileInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (fileInode.mode & syscall.S_IFMT) != syscall.S_IFREG {
		errno = syscall.EINVAL
		return
	}

	openOut = &fission.OpenOut{
		FH:        0,
		OpenFlags: 0,
		Padding:   0,
	}

	errno = 0
	return
}

/*
	cacheLineTag.inodeNumber = fileInode.inodeNumber
	cacheLineTag.lineNumber = fileOffsetCurrent / globals.config.CacheLineSize
*/

func (fileInode *inodeStruct) fetchCacheLine(lineNumber uint64) (objectCacheLine []byte, err error) {
	var (
		getObjectInput   *s3.GetObjectInput
		getObjectOutput  *s3.GetObjectOutput
		rangeOffsetFirst uint64
		rangeOffsetLast  uint64
	)

	rangeOffsetFirst = lineNumber * globals.config.CacheLineSize
	rangeOffsetLast = rangeOffsetFirst + globals.config.CacheLineSize - 1
	if rangeOffsetLast > (fileInode.size - 1) {
		rangeOffsetLast = fileInode.size - 1
	}

	getObjectInput = &s3.GetObjectInput{
		Bucket: aws.String(globals.config.S3Bucket),
		Key:    aws.String(fileInode.objectKey),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", rangeOffsetFirst, rangeOffsetLast)),
	}

	getObjectOutput, err = globals.s3Client.GetObject(context.TODO(), getObjectInput)
	if err != nil {
		return
	}

	objectCacheLine, err = io.ReadAll(getObjectOutput.Body)
	if err != nil {
		return
	}

	if uint64(len(objectCacheLine)) == (rangeOffsetLast - rangeOffsetFirst + 1) {
		err = nil
	} else {
		err = fmt.Errorf("uint64(len(objectCacheLine)) [%v] != (rangeOffsetLast - rangeOffsetFirst + 1) [%v]", len(objectCacheLine), rangeOffsetLast-rangeOffsetFirst+1)
	}

	return
}

func (dummy *globalsStruct) DoRead(inHeader *fission.InHeader, readIn *fission.ReadIn) (readOut *fission.ReadOut, errno syscall.Errno) {
	var (
		cacheLineTag                 cacheLineTagStruct
		err                          error
		fileCacheLine                *fileCacheLineStruct
		fileInode                    *inodeStruct
		fileOffsetCurrent            uint64
		fileOffsetLimit              uint64
		listElement                  *list.Element
		ok                           bool
		ramCacheLine                 *ramCacheLineStruct
		ramCacheLineContent          []byte
		ramCacheLineContentLimit     uint64
		ramCacheLineContentOffset    uint64
		ramCacheLineContentRemaining uint64
	)

	fileInode = fetchInode(inHeader.NodeID)
	if fileInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (fileInode.mode & syscall.S_IFMT) != syscall.S_IFREG {
		errno = syscall.EINVAL
		return
	}

	fileOffsetCurrent = readIn.Offset
	if fileOffsetCurrent > fileInode.size {
		fileOffsetCurrent = fileInode.size
	}

	fileOffsetLimit = fileOffsetCurrent + uint64(readIn.Size)
	if (fileOffsetLimit > fileInode.size) || (fileOffsetLimit < fileOffsetCurrent) {
		fileOffsetLimit = fileInode.size
	}

	readOut = &fission.ReadOut{
		Data: make([]byte, 0, fileOffsetLimit-fileOffsetCurrent),
	}

	for fileOffsetCurrent < fileOffsetLimit {
		globals.Lock()

		// First, see if we need to prune ramCache

		if uint64(len(globals.ramCacheMap)) > globals.config.RAMCacheLines {
			listElement = globals.ramCacheLRU.Front()
			ramCacheLine, ok = listElement.Value.(*ramCacheLineStruct)
			if !ok {
				globals.logger.Fatalf("listElement.Value.(*ramCacheLineStruct) returned !ok")
			}
			if ramCacheLine.content == nil {
				globals.Unlock()
				ramCacheLine.Wait()
			} else {
				if globals.config.FileCacheLines == 0 {
					ramCacheLine.content = nil
					_ = globals.ramCacheLRU.Remove(ramCacheLine.listElement)
					delete(globals.ramCacheMap, ramCacheLine.tag)
					globals.Unlock()
				} else {
					fileCacheLine, ok = globals.fileCacheMap[ramCacheLine.tag]
					if ok {
						ramCacheLine.content = nil
						_ = globals.ramCacheLRU.Remove(ramCacheLine.listElement)
						delete(globals.ramCacheMap, ramCacheLine.tag)
						globals.Unlock()
					} else {
						ramCacheLineContent = ramCacheLine.content
						ramCacheLine.content = nil
						_ = globals.ramCacheLRU.Remove(ramCacheLine.listElement)
						delete(globals.ramCacheMap, ramCacheLine.tag)
						fileCacheLine = &fileCacheLineStruct{
							tag:          ramCacheLine.tag,
							contentReady: false,
						}
						fileCacheLine.Add(1)
						fileCacheLine.listElement = globals.fileCacheLRU.PushBack(fileCacheLine)
						globals.fileCacheMap[fileCacheLine.tag] = fileCacheLine
						globals.Unlock()
						err = os.WriteFile(fmt.Sprintf("%s/%08X_%08X", globals.fileCacheDir, fileCacheLine.tag.inodeNumber, fileCacheLine.tag.lineNumber), ramCacheLineContent, 0666)
						if err != nil {
							globals.logger.Fatalf("os.WriteFile(\"%s/%08X_%08X\", ramCacheLineContent, 0666) failed: %v\n", globals.fileCacheDir, fileCacheLine.tag.inodeNumber, fileCacheLine.tag.lineNumber, err)
						}
						globals.Lock()
						fileCacheLine.contentReady = true
						globals.Unlock()
						fileCacheLine.Done()
					}
				}
			}

			// However we reached here, just retry at fileOffsetCurrent

			continue
		}

		// Next, see if we need to prune fileCache (if enabled)

		if globals.config.FileCacheLines >= 0 {
			if uint64(len(globals.fileCacheMap)) > globals.config.FileCacheLines {
				listElement = globals.fileCacheLRU.Front()
				fileCacheLine, ok = listElement.Value.(*fileCacheLineStruct)
				if !ok {
					globals.logger.Fatalf("listElement.Value.(*fileCacheLineStruct) returned !ok")
				}
				if !fileCacheLine.contentReady {
					globals.Unlock()
					fileCacheLine.Wait()
				} else {
					fileCacheLine.Add(1)
					fileCacheLine.contentReady = false
					_ = globals.fileCacheLRU.Remove(fileCacheLine.listElement)
					delete(globals.fileCacheMap, fileCacheLine.tag)
					globals.Unlock()
					err = os.Remove(fmt.Sprintf("%s/%08X_%08X", globals.fileCacheDir, fileCacheLine.tag.inodeNumber, fileCacheLine.tag.lineNumber))
					if err != nil {
						globals.logger.Fatalf("os.Remove(\"%s/%08X_%08X\") failed: %v", fileCacheLine.tag.inodeNumber, fileCacheLine.tag.lineNumber, err)
					}
					fileCacheLine.Done()
				}

				// However we reached here, just retry at fileOffsetCurrent

				continue
			}
		}

		cacheLineTag.inodeNumber = fileInode.inodeNumber
		cacheLineTag.lineNumber = fileOffsetCurrent / globals.config.CacheLineSize

		ramCacheLine, ok = globals.ramCacheMap[cacheLineTag]
		if ok {
			if ramCacheLine.content == nil {
				globals.Unlock()
				ramCacheLine.Wait()
			} else {
				globals.ramCacheLRU.MoveToBack(ramCacheLine.listElement)
				ramCacheLineContent = ramCacheLine.content
				globals.Unlock()
				ramCacheLineContentOffset = fileOffsetCurrent - (cacheLineTag.lineNumber * globals.config.CacheLineSize)
				ramCacheLineContentRemaining = uint64(len(ramCacheLineContent)) - ramCacheLineContentOffset
				if ramCacheLineContentRemaining > (fileOffsetLimit - fileOffsetCurrent) {
					ramCacheLineContentLimit = ramCacheLineContentOffset + (fileOffsetLimit - fileOffsetCurrent)
				} else {
					ramCacheLineContentLimit = uint64(len(ramCacheLineContent))
				}
				readOut.Data = append(readOut.Data, ramCacheLineContent[ramCacheLineContentOffset:ramCacheLineContentLimit]...)
				fileOffsetCurrent += (ramCacheLineContentLimit - ramCacheLineContentOffset)
			}
		} else {
			if globals.config.FileCacheLines == 0 {
				ramCacheLine = &ramCacheLineStruct{
					tag:     cacheLineTag,
					content: nil,
				}
				ramCacheLine.Add(1)
				ramCacheLine.listElement = globals.ramCacheLRU.PushBack(ramCacheLine)
				globals.ramCacheMap[ramCacheLine.tag] = ramCacheLine
				globals.Unlock()
				ramCacheLineContent, err = fileInode.fetchCacheLine(ramCacheLine.tag.lineNumber)
				if err != nil {
					globals.logger.Fatalf("fileInode.fetchCacheLine(ramCacheLine.tag.lineNumber) failed: %v", err)
				}
				globals.Lock()
				ramCacheLine.content = ramCacheLineContent
				globals.Unlock()
				ramCacheLine.Done()
			} else {
				fileCacheLine, ok = globals.fileCacheMap[cacheLineTag]
				if ok {
					if fileCacheLine.contentReady {
						globals.fileCacheLRU.MoveToBack(fileCacheLine.listElement)
						ramCacheLine = &ramCacheLineStruct{
							tag:     cacheLineTag,
							content: nil,
						}
						ramCacheLine.Add(1)
						ramCacheLine.listElement = globals.ramCacheLRU.PushBack(ramCacheLine)
						globals.ramCacheMap[ramCacheLine.tag] = ramCacheLine
						globals.Unlock()
						ramCacheLineContent, err = os.ReadFile(fmt.Sprintf("%s/%08X_%08X", globals.fileCacheDir, fileCacheLine.tag.inodeNumber, fileCacheLine.tag.lineNumber))
						if err != nil {
							globals.logger.Fatalf("os.ReadFile(\"%s/%08X_%08X\") failed: %v", globals.fileCacheDir, fileCacheLine.tag.inodeNumber, fileCacheLine.tag.lineNumber, err)
						}
						globals.Lock()
						ramCacheLine.content = ramCacheLineContent
						globals.Unlock()
						ramCacheLine.Done()
					} else {
						globals.Unlock()
						fileCacheLine.Wait()
					}
				} else {
					ramCacheLine = &ramCacheLineStruct{
						tag:     cacheLineTag,
						content: nil,
					}
					ramCacheLine.Add(1)
					ramCacheLine.listElement = globals.ramCacheLRU.PushBack(ramCacheLine)
					globals.ramCacheMap[ramCacheLine.tag] = ramCacheLine
					globals.Unlock()
					ramCacheLineContent, err = fileInode.fetchCacheLine(ramCacheLine.tag.lineNumber)
					if err != nil {
						globals.logger.Fatalf("fileInode.fetchCacheLine(ramCacheLine.tag.lineNumber) failed: %v", err)
					}
					globals.Lock()
					ramCacheLine.content = ramCacheLineContent
					globals.Unlock()
					ramCacheLine.Done()
				}
			}
		}
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
			Blocks:  globals.blocks,
			BFree:   0,
			BAvail:  0,
			Files:   uint64(len(globals.inodeTable)),
			FFree:   0,
			BSize:   0,
			NameLen: 0,
			FRSize:  attrBlkSize,
			Padding: 0,
			Spare:   [6]uint32{0, 0, 0, 0, 0, 0},
		},
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoRelease(inHeader *fission.InHeader, releaseIn *fission.ReleaseIn) (errno syscall.Errno) {
	var (
		fileInode *inodeStruct
	)

	fileInode = fetchInode(inHeader.NodeID)
	if fileInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (fileInode.mode & syscall.S_IFMT) != syscall.S_IFREG {
		errno = syscall.EINVAL
		return
	}

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
	var (
		dirInode *inodeStruct
	)

	dirInode = fetchInode(inHeader.NodeID)
	if dirInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (dirInode.mode & syscall.S_IFMT) != syscall.S_IFDIR {
		errno = syscall.EINVAL
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
		dirEntCountMax           uint64
		dirEntMinSize            uint64
		dirEntSize               uint64
		dirEntSliceSize          uint64
		dirInode                 *inodeStruct
		dirTableEntryKey         sortedmap.Key
		dirTableEntryInode       *inodeStruct
		dirTableEntryInodeNumber uint64
		dirTableEntryName        string
		dirTableEntryValue       sortedmap.Value
		dirTableIndex            int
		dirTableLen              int
		err                      error
		ok                       bool
	)

	dirInode = fetchInode(inHeader.NodeID)
	if dirInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (dirInode.mode & syscall.S_IFMT) != syscall.S_IFDIR {
		errno = syscall.EINVAL
		return
	}

	dirEntMinSize = fission.DirEntFixedPortionSize + 1 + fission.DirEntAlignment - 1
	dirEntMinSize /= fission.DirEntAlignment
	dirEntMinSize *= fission.DirEntAlignment
	dirEntCountMax = uint64(readDirIn.Size) / dirEntMinSize

	readDirOut = &fission.ReadDirOut{
		DirEnt: make([]fission.DirEnt, 0, dirEntCountMax),
	}

	if dirEntCountMax == 0 {
		errno = 0
		return
	}

	dirTableLen, err = dirInode.dirTable.Len()
	if err != nil {
		globals.logger.Fatalf("dirInode.dirTable.Len() failed: %v", err)
	}

	if readDirIn.Offset >= uint64(dirTableLen) {
		errno = 0
		return
	}

	dirTableIndex = int(readDirIn.Offset)
	dirEntSliceSize = 0

	for dirTableIndex < dirTableLen {
		dirTableEntryKey, dirTableEntryValue, ok, err = dirInode.dirTable.GetByIndex(dirTableIndex)
		if err != nil {
			globals.logger.Fatalf("dirInode.dirTable.GetByIndex(dirTableIndex) failed: %v", err)
		}
		if !ok {
			globals.logger.Fatalf("dirInode.dirTable.GetByIndex(dirTableIndex) returned !ok")
		}

		dirTableEntryName, ok = dirTableEntryKey.(string)
		if !ok {
			globals.logger.Fatalf("dirTableEntryKey.(string) returned !ok")
		}

		dirTableEntryInodeNumber, ok = dirTableEntryValue.(uint64)
		if !ok {
			globals.logger.Fatalf(" dirTableEntryValue.(uint64) returned !ok")
		}

		dirEntSize = fission.DirEntFixedPortionSize + uint64(len(dirTableEntryName)) + fission.DirEntAlignment - 1
		dirEntSize /= fission.DirEntAlignment
		dirEntSize *= fission.DirEntAlignment

		dirEntSliceSize += dirEntSize
		if dirEntSliceSize > uint64(readDirIn.Size) {
			break
		}

		dirTableEntryInode = fetchInode(dirTableEntryInodeNumber)
		if dirTableEntryInode == nil {
			globals.logger.Fatalf("fetchInode(dirTableEntryInodeNumber) returned nil")
		}

		dirTableIndex++

		readDirOut.DirEnt = append(readDirOut.DirEnt, fission.DirEnt{
			Ino:     dirTableEntryInode.inodeNumber,
			Off:     uint64(dirTableIndex),
			NameLen: uint32(len(dirTableEntryName)),
			Type:    dirTableEntryInode.mode & syscall.S_IFMT,
			Name:    []byte(dirTableEntryName),
		})
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoReleaseDir(inHeader *fission.InHeader, releaseDirIn *fission.ReleaseDirIn) (errno syscall.Errno) {
	var (
		dirInode *inodeStruct
	)

	dirInode = fetchInode(inHeader.NodeID)
	if dirInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (dirInode.mode & syscall.S_IFMT) != syscall.S_IFDIR {
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
		inode *inodeStruct
	)

	inode = fetchInode(inHeader.NodeID)
	if inode == nil {
		errno = syscall.ENOENT
		return
	}

	if (accessIn.Mask & accessWOK) == 0 {
		errno = 0
	} else {
		errno = syscall.EACCES
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
		dirEntPlusCountMax       uint64
		dirEntPlusMinSize        uint64
		dirEntPlusSize           uint64
		dirEntPlusSliceSize      uint64
		dirInode                 *inodeStruct
		dirTableEntryKey         sortedmap.Key
		dirTableEntryInode       *inodeStruct
		dirTableEntryInodeNumber uint64
		dirTableEntryName        string
		dirTableEntryValue       sortedmap.Value
		dirTableIndex            int
		dirTableLen              int
		err                      error
		ok                       bool
	)

	dirInode = fetchInode(inHeader.NodeID)
	if dirInode == nil {
		errno = syscall.ENOENT
		return
	}

	if (dirInode.mode & syscall.S_IFMT) != syscall.S_IFDIR {
		errno = syscall.EINVAL
		return
	}

	dirEntPlusMinSize = fission.DirEntPlusFixedPortionSize + 1 + fission.DirEntAlignment - 1
	dirEntPlusMinSize /= fission.DirEntAlignment
	dirEntPlusMinSize *= fission.DirEntAlignment
	dirEntPlusCountMax = uint64(readDirPlusIn.Size) / dirEntPlusMinSize

	readDirPlusOut = &fission.ReadDirPlusOut{
		DirEntPlus: make([]fission.DirEntPlus, 0, dirEntPlusCountMax),
	}

	if dirEntPlusCountMax == 0 {
		errno = 0
		return
	}

	dirTableLen, err = dirInode.dirTable.Len()
	if err != nil {
		globals.logger.Fatalf("dirInode.dirTable.Len() failed: %v", err)
	}

	if readDirPlusIn.Offset >= uint64(dirTableLen) {
		errno = 0
		return
	}

	dirTableIndex = int(readDirPlusIn.Offset)
	dirEntPlusSliceSize = 0

	for dirTableIndex < dirTableLen {
		dirTableEntryKey, dirTableEntryValue, ok, err = dirInode.dirTable.GetByIndex(dirTableIndex)
		if err != nil {
			globals.logger.Fatalf("dirInode.dirTable.GetByIndex(dirTableIndex) failed: %v", err)
		}
		if !ok {
			globals.logger.Fatalf("dirInode.dirTable.GetByIndex(dirTableIndex) returned !ok")
		}

		dirTableEntryName, ok = dirTableEntryKey.(string)
		if !ok {
			globals.logger.Fatalf("dirTableEntryKey.(string) returned !ok")
		}

		dirTableEntryInodeNumber, ok = dirTableEntryValue.(uint64)
		if !ok {
			globals.logger.Fatalf(" dirTableEntryValue.(uint64) returned !ok")
		}

		dirEntPlusSize = fission.DirEntPlusFixedPortionSize + uint64(len(dirTableEntryName)) + fission.DirEntAlignment - 1
		dirEntPlusSize /= fission.DirEntAlignment
		dirEntPlusSize *= fission.DirEntAlignment

		dirEntPlusSliceSize += dirEntPlusSize
		if dirEntPlusSliceSize > uint64(readDirPlusIn.Size) {
			break
		}

		dirTableEntryInode = fetchInode(dirTableEntryInodeNumber)
		if dirTableEntryInode == nil {
			globals.logger.Fatalf("fetchInode(dirTableEntryInodeNumber) returned nil")
		}

		dirTableIndex++

		readDirPlusOut.DirEntPlus = append(readDirPlusOut.DirEntPlus, fission.DirEntPlus{
			EntryOut: fission.EntryOut{
				NodeID:         dirTableEntryInode.inodeNumber,
				Generation:     entryGeneration,
				EntryValidSec:  entryValidSec,
				AttrValidSec:   attrValidSec,
				EntryValidNSec: entryValidNSec,
				AttrValidNSec:  attrValidNSec,
			},
			DirEnt: fission.DirEnt{
				Ino:     dirTableEntryInode.inodeNumber,
				Off:     uint64(dirTableIndex),
				NameLen: uint32(len(dirTableEntryName)),
				Type:    dirTableEntryInode.mode & syscall.S_IFMT,
				Name:    []byte(dirTableEntryName),
			},
		})

		populateAttr(&readDirPlusOut.DirEntPlus[len(readDirPlusOut.DirEntPlus)-1].EntryOut.Attr, dirTableEntryInode)
	}

	errno = 0
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
