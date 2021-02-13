// Copyright (c) 2015-2021, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/NVIDIA/fission"
	"github.com/NVIDIA/sortedmap"
)

func (fileInode *fileInodeStruct) ensureAttrInCache() {
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

	// TODO: populate cachedAttr

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
		fileInode, ok = globals.fileInodeMap[uint64(inHeader.NodeID)]
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

	fileInode, ok = globals.fileInodeMap[uint64(inHeader.NodeID)]
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
	// TODO: Implement DoRead()

	// if helloInodeIno != inHeader.NodeID {
	// 	errno = syscall.ENOENT
	// 	return
	// }

	// var (
	// 	adjustedOffset uint64
	// 	adjustedSize   uint32
	// )

	// adjustedOffset = uint64(len(helloInodeFileData))
	// if readIn.Offset < adjustedOffset {
	// 	adjustedOffset = readIn.Offset
	// }

	// adjustedSize = readIn.Size
	// if (adjustedOffset + uint64(adjustedSize)) > uint64(len(helloInodeFileData)) {
	// 	adjustedSize = uint32(len(helloInodeFileData)) - uint32(adjustedOffset)
	// }

	// readOut = &fission.ReadOut{
	// 	Data: cloneByteSlice(helloInodeFileData[adjustedOffset:(adjustedOffset + uint64(adjustedSize))]),
	// }

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
		MaxWrite:             initOutMaxWrite,
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
	// TODO: Implement DoReadDir()

	// var (
	// 	dirEntIndex          int
	// 	dirEntNameLenAligned uint32
	// 	dirEntSize           uint32
	// 	totalSize            uint32
	// )

	// if helloInodeIno == inHeader.NodeID {
	// 	errno = syscall.EINVAL
	// 	return
	// }
	// if 1 != inHeader.NodeID {
	// 	errno = syscall.ENOENT
	// 	return
	// }

	// readDirOut = &fission.ReadDirOut{
	// 	DirEnt: globals.dirEnt[readDirIn.Offset:],
	// }

	// totalSize = 0

	// for dirEntIndex = range readDirOut.DirEnt {
	// 	dirEntNameLenAligned = (uint32(len(readDirOut.DirEnt[dirEntIndex].Name)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
	// 	dirEntSize = fission.DirEntFixedPortionSize + dirEntNameLenAligned

	// 	if (totalSize + dirEntSize) > readDirIn.Size {
	// 		// Truncate readDirOut here and return

	// 		readDirOut.DirEnt = readDirOut.DirEnt[:dirEntIndex]

	// 		errno = 0
	// 		return
	// 	}
	// }

	errno = 0
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
			fileInode, ok = globals.fileInodeMap[uint64(inHeader.NodeID)]
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
	// TODO: Implement DoReadDirPlus()

	// var (
	// 	dirEntPlusIndex      int
	// 	dirEntNameLenAligned uint32
	// 	dirEntSize           uint32
	// 	totalSize            uint32
	// )

	// if helloInodeIno == inHeader.NodeID {
	// 	errno = syscall.EINVAL
	// 	return
	// }
	// if 1 != inHeader.NodeID {
	// 	errno = syscall.ENOENT
	// 	return
	// }

	// readDirPlusOut = &fission.ReadDirPlusOut{
	// 	DirEntPlus: globals.dirEntPlus[readDirPlusIn.Offset:],
	// }

	// totalSize = 0

	// for dirEntPlusIndex = range readDirPlusOut.DirEntPlus {
	// 	dirEntNameLenAligned = (uint32(len(readDirPlusOut.DirEntPlus[dirEntPlusIndex].Name)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
	// 	dirEntSize = fission.DirEntPlusFixedPortionSize + dirEntNameLenAligned

	// 	if (totalSize + dirEntSize) > readDirPlusIn.Size {
	// 		// Truncate readDirOut here and return

	// 		readDirPlusOut.DirEntPlus = readDirPlusOut.DirEntPlus[:dirEntPlusIndex]

	// 		errno = 0
	// 		return
	// 	}
	// }

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
