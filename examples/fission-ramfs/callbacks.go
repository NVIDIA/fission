// Copyright (c) 2015-2025, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"syscall"

	"github.com/NVIDIA/fission/v3"
	"github.com/NVIDIA/sortedmap"
)

func (*globalsStruct) DoLookup(inHeader *fission.InHeader, lookupIn *fission.LookupIn) (lookupOut *fission.LookupOut, errno syscall.Errno) {
	var (
		dirEntInoAsU64   uint64
		dirEntInoAsValue sortedmap.Value
		dirEntInode      *inodeStruct
		dirInode         *inodeStruct
		err              error
		granted          bool
		grantedLockSet   = makeGrantedLockSet()
		ok               bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	dirEntInoAsValue, ok, err = dirInode.dirEntryMap.GetByKey(lookupIn.Name)
	if err != nil {
		globals.logger.Printf("func DoLookup(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(lookupIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	dirEntInoAsU64 = dirEntInoAsValue.(uint64)

	dirEntInode, ok = globals.inodeMap[dirEntInoAsU64]
	if !ok {
		globals.logger.Printf("func DoLookup(NodeID==%v) failed fetching globals.inodeMap[%v]", inHeader.NodeID, dirEntInoAsU64)
		os.Exit(1)
	}

	granted = grantedLockSet.try(dirEntInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	lookupOut = &fission.LookupOut{
		EntryOut: fission.EntryOut{
			NodeID:         dirEntInode.attr.Ino,
			Generation:     0,
			EntryValidSec:  entryValidSec,
			AttrValidSec:   attrValidSec,
			EntryValidNSec: entryValidNSec,
			AttrValidNSec:  attrValidNSec,
			Attr: fission.Attr{
				Ino:       dirEntInode.attr.Ino,
				Size:      dirEntInode.attr.Size,
				ATimeSec:  dirEntInode.attr.ATimeSec,
				MTimeSec:  dirEntInode.attr.MTimeSec,
				CTimeSec:  dirEntInode.attr.CTimeSec,
				ATimeNSec: dirEntInode.attr.ATimeNSec,
				MTimeNSec: dirEntInode.attr.MTimeNSec,
				CTimeNSec: dirEntInode.attr.CTimeNSec,
				Mode:      dirEntInode.attr.Mode,
				NLink:     dirEntInode.attr.NLink,
				UID:       dirEntInode.attr.UID,
				GID:       dirEntInode.attr.GID,
				RDev:      dirEntInode.attr.RDev,
				Padding:   dirEntInode.attr.Padding,
			},
		},
	}

	fixAttrSizes(&lookupOut.EntryOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoForget(_ *fission.InHeader, _ *fission.ForgetIn) {
}

func (*globalsStruct) DoGetAttr(inHeader *fission.InHeader, _ *fission.GetAttrIn) (getAttrOut *fission.GetAttrOut, errno syscall.Errno) {
	var (
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		inode          *inodeStruct
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	getAttrOut = &fission.GetAttrOut{
		AttrValidSec:  attrValidSec,
		AttrValidNSec: attrValidNSec,
		Dummy:         0,
		Attr: fission.Attr{
			Ino:       inode.attr.Ino,
			Size:      inode.attr.Size,
			ATimeSec:  inode.attr.ATimeSec,
			MTimeSec:  inode.attr.MTimeSec,
			CTimeSec:  inode.attr.CTimeSec,
			ATimeNSec: inode.attr.ATimeNSec,
			MTimeNSec: inode.attr.MTimeNSec,
			CTimeNSec: inode.attr.CTimeNSec,
			Mode:      inode.attr.Mode,
			NLink:     inode.attr.NLink,
			UID:       inode.attr.UID,
			GID:       inode.attr.GID,
			RDev:      inode.attr.RDev,
			Padding:   inode.attr.Padding,
		},
	}

	fixAttrSizes(&getAttrOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoSetAttr(inHeader *fission.InHeader, setAttrIn *fission.SetAttrIn) (setAttrOut *fission.SetAttrOut, errno syscall.Errno) {
	var (
		granted         bool
		grantedLockSet  = makeGrantedLockSet()
		inode           *inodeStruct
		inodeAttrMode   uint32
		ok              bool
		setAttrInMode   uint32
		unixTimeNowNSec uint32
		unixTimeNowSec  uint64
	)

	if setAttrIn.Valid != (setAttrIn.Valid & (fission.SetAttrInValidMode | fission.SetAttrInValidUID | fission.SetAttrInValidGID | fission.SetAttrInValidSize | fission.SetAttrInValidATime | fission.SetAttrInValidMTime | fission.SetAttrInValidFH | fission.SetAttrInValidATimeNow | fission.SetAttrInValidMTimeNow)) {
		errno = syscall.ENOSYS
		return
	}

Restart:
	grantedLockSet.get(globals.tryLock)

	if ((setAttrIn.Valid & fission.SetAttrInValidFH) != 0) && (setAttrIn.FH != 0) {
		if !globals.alreadyLoggedIgnoring.setAttrInValidFH {
			globals.logger.Printf("func DoSetAttr(,setAttrIn.Valid==0x%08X) ignoring FH bit (0x%08X)", setAttrIn.Valid, fission.SetAttrInValidFH)
			globals.alreadyLoggedIgnoring.setAttrInValidFH = true
		}
	}

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	unixTimeNowSec, unixTimeNowNSec = unixTimeNow()

	if (setAttrIn.Valid & fission.SetAttrInValidMode) != 0 {
		inodeAttrMode = inode.attr.Mode & ^uint32(syscall.S_IRWXU|syscall.S_IRWXG|syscall.S_IRWXO)
		setAttrInMode = setAttrIn.Mode & uint32(syscall.S_IRWXU|syscall.S_IRWXG|syscall.S_IRWXO)
		inodeAttrMode |= setAttrInMode

		inode.attr.Mode = inodeAttrMode
	}

	if (setAttrIn.Valid & fission.SetAttrInValidUID) != 0 {
		inode.attr.UID = setAttrIn.UID
	}
	if (setAttrIn.Valid & fission.SetAttrInValidGID) != 0 {
		inode.attr.GID = setAttrIn.GID
	}

	if (setAttrIn.Valid & fission.SetAttrInValidSize) != 0 {
		if syscall.S_IFREG != (inode.attr.Mode & syscall.S_IFMT) {
			grantedLockSet.freeAll(false)
			errno = syscall.EINVAL
			return
		}
		if setAttrIn.Size <= inode.attr.Size {
			inode.fileData = inode.fileData[:setAttrIn.Size]
		} else {
			inode.fileData = append(inode.fileData, make([]byte, (setAttrIn.Size-inode.attr.Size))...)
		}
		inode.attr.Size = setAttrIn.Size
		inode.attr.Blocks = inode.attr.Size + uint64(attrBlkSize-1)
		inode.attr.Blocks /= uint64(attrBlkSize)
	}

	if (setAttrIn.Valid & fission.SetAttrInValidATime) != 0 {
		inode.attr.ATimeSec = setAttrIn.ATimeSec
		inode.attr.ATimeNSec = setAttrIn.ATimeNSec
	}

	if (setAttrIn.Valid & fission.SetAttrInValidMTime) != 0 {
		inode.attr.MTimeSec = setAttrIn.MTimeSec
		inode.attr.MTimeNSec = setAttrIn.MTimeNSec
	}

	if (setAttrIn.Valid & fission.SetAttrInValidATimeNow) != 0 {
		inode.attr.ATimeSec = unixTimeNowSec
		inode.attr.ATimeNSec = unixTimeNowNSec
	}

	if (setAttrIn.Valid & fission.SetAttrInValidMTimeNow) != 0 {
		inode.attr.MTimeSec = unixTimeNowSec
		inode.attr.MTimeNSec = unixTimeNowNSec
	}

	// TODO: Verify it is ok to accept but ignore fission.SetAttrInValidFH in setAttrIn.Valid

	setAttrOut = &fission.SetAttrOut{
		AttrValidSec:  attrValidSec,
		AttrValidNSec: attrValidNSec,
		Dummy:         0,
		Attr: fission.Attr{
			Ino:       inode.attr.Ino,
			Size:      inode.attr.Size,
			ATimeSec:  inode.attr.ATimeSec,
			MTimeSec:  inode.attr.MTimeSec,
			CTimeSec:  inode.attr.CTimeSec,
			ATimeNSec: inode.attr.ATimeNSec,
			MTimeNSec: inode.attr.MTimeNSec,
			CTimeNSec: inode.attr.CTimeNSec,
			Mode:      inode.attr.Mode,
			NLink:     inode.attr.NLink,
			UID:       inode.attr.UID,
			GID:       inode.attr.GID,
			RDev:      inode.attr.RDev,
			Padding:   inode.attr.Padding,
		},
	}

	fixAttrSizes(&setAttrOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoReadLink(inHeader *fission.InHeader) (readLinkOut *fission.ReadLinkOut, errno syscall.Errno) {
	var (
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
		symInode       *inodeStruct
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	symInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(symInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFLNK != (symInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	readLinkOut = &fission.ReadLinkOut{
		Data: cloneByteSlice(symInode.symlinkData),
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoSymLink(inHeader *fission.InHeader, symLinkIn *fission.SymLinkIn) (symLinkOut *fission.SymLinkOut, errno syscall.Errno) {
	var (
		dirEntInode     *inodeStruct
		dirEntInodeMode uint32
		dirInode        *inodeStruct
		err             error
		granted         bool
		grantedLockSet  = makeGrantedLockSet()
		ok              bool
		unixTimeNowNSec uint32
		unixTimeNowSec  uint64
	)

	dirEntInodeMode = uint32(syscall.S_IRWXU | syscall.S_IRWXG | syscall.S_IRWXO)
	dirEntInodeMode |= syscall.S_IFLNK

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	_, ok, err = dirInode.dirEntryMap.GetByKey(symLinkIn.Name)
	if err != nil {
		globals.logger.Printf("func DoSymLink(NodeID==%v,Name=%s,Data=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(symLinkIn.Name), string(symLinkIn.Data), err)
		os.Exit(1)
	}

	if ok {
		grantedLockSet.freeAll(false)
		errno = syscall.EEXIST
		return
	}

	globals.lastNodeID++

	unixTimeNowSec, unixTimeNowNSec = unixTimeNow()

	dirEntInode = &inodeStruct{
		tryLock: makeTryLock(),
		attr: fission.Attr{
			Ino:       globals.lastNodeID,
			ATimeSec:  unixTimeNowSec,
			MTimeSec:  unixTimeNowSec,
			CTimeSec:  unixTimeNowSec,
			ATimeNSec: unixTimeNowNSec,
			MTimeNSec: unixTimeNowNSec,
			CTimeNSec: unixTimeNowNSec,
			Mode:      dirEntInodeMode,
			NLink:     1,
			UID:       inHeader.UID,
			GID:       inHeader.GID,
			RDev:      0,
			Padding:   0,
		},
		xattrMap:    sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.xattrMapDummy),
		dirEntryMap: nil,
		fileData:    nil,
		symlinkData: symLinkIn.Data,
	}

	fixAttrSizes(&dirEntInode.attr)

	ok, err = dirInode.dirEntryMap.Put(symLinkIn.Name, dirEntInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func DoSymLink(NodeID==%v,Name=%s,Data=%s) failed on .dirEntryMap.Put(): %v", inHeader.NodeID, string(symLinkIn.Name), string(symLinkIn.Data), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoSymLink(NodeID==%v,Name=%s,Data=%s) .dirEntryMap.Put() returned !ok", inHeader.NodeID, string(symLinkIn.Name), string(symLinkIn.Data))
		os.Exit(1)
	}

	globals.inodeMap[dirEntInode.attr.Ino] = dirEntInode

	symLinkOut = &fission.SymLinkOut{
		EntryOut: fission.EntryOut{
			NodeID:         dirEntInode.attr.Ino,
			Generation:     0,
			EntryValidSec:  entryValidSec,
			AttrValidSec:   attrValidSec,
			EntryValidNSec: entryValidNSec,
			AttrValidNSec:  attrValidNSec,
			Attr: fission.Attr{
				Ino:       dirEntInode.attr.Ino,
				ATimeSec:  dirEntInode.attr.ATimeSec,
				MTimeSec:  dirEntInode.attr.MTimeSec,
				CTimeSec:  dirEntInode.attr.CTimeSec,
				ATimeNSec: dirEntInode.attr.ATimeNSec,
				MTimeNSec: dirEntInode.attr.MTimeNSec,
				CTimeNSec: dirEntInode.attr.CTimeNSec,
				Mode:      dirEntInode.attr.Mode,
				NLink:     dirEntInode.attr.NLink,
				UID:       dirEntInode.attr.UID,
				GID:       dirEntInode.attr.GID,
				RDev:      dirEntInode.attr.RDev,
				Padding:   dirEntInode.attr.Padding,
			},
		},
	}

	fixAttrSizes(&symLinkOut.EntryOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoMkNod(_ *fission.InHeader, _ *fission.MkNodIn) (mkNodOut *fission.MkNodOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoMkDir(inHeader *fission.InHeader, mkDirIn *fission.MkDirIn) (mkDirOut *fission.MkDirOut, errno syscall.Errno) {
	var (
		dirEntInode     *inodeStruct
		dirEntInodeMode uint32
		dirInode        *inodeStruct
		err             error
		granted         bool
		grantedLockSet  = makeGrantedLockSet()
		ok              bool
		unixTimeNowNSec uint32
		unixTimeNowSec  uint64
	)

	dirEntInodeMode = uint32(syscall.S_IRWXU | syscall.S_IRWXG | syscall.S_IRWXO)
	dirEntInodeMode &= mkDirIn.Mode
	dirEntInodeMode &= ^mkDirIn.UMask
	dirEntInodeMode |= syscall.S_IFDIR

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	_, ok, err = dirInode.dirEntryMap.GetByKey(mkDirIn.Name)
	if err != nil {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(mkDirIn.Name), err)
		os.Exit(1)
	}

	if ok {
		grantedLockSet.freeAll(false)
		errno = syscall.EEXIST
		return
	}

	globals.lastNodeID++

	unixTimeNowSec, unixTimeNowNSec = unixTimeNow()

	dirEntInode = &inodeStruct{
		tryLock: makeTryLock(),
		attr: fission.Attr{
			Ino:       globals.lastNodeID,
			ATimeSec:  unixTimeNowSec,
			MTimeSec:  unixTimeNowSec,
			CTimeSec:  unixTimeNowSec,
			ATimeNSec: unixTimeNowNSec,
			MTimeNSec: unixTimeNowNSec,
			CTimeNSec: unixTimeNowNSec,
			Mode:      dirEntInodeMode,
			NLink:     2,
			UID:       inHeader.UID,
			GID:       inHeader.GID,
			RDev:      0,
			Padding:   0,
		},
		xattrMap:    sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.xattrMapDummy),
		dirEntryMap: sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.dirEntryMapDummy),
		fileData:    nil,
		symlinkData: nil,
	}

	fixAttrSizes(&dirEntInode.attr)

	ok, err = dirEntInode.dirEntryMap.Put([]byte("."), dirEntInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) failed on dirEntInode.dirEntryMap.Put(\".\"): %v", inHeader.NodeID, string(mkDirIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) dirEntInode.dirEntryMap.Put(\".\") returned !ok", inHeader.NodeID, string(mkDirIn.Name))
		os.Exit(1)
	}
	ok, err = dirEntInode.dirEntryMap.Put([]byte(".."), dirInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) failed on dirEntInode.dirEntryMap.Put(\"..\"): %v", inHeader.NodeID, string(mkDirIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) dirEntInode.dirEntryMap.Put(\"..\") returned !ok", inHeader.NodeID, string(mkDirIn.Name))
		os.Exit(1)
	}
	ok, err = dirInode.dirEntryMap.Put(mkDirIn.Name, dirEntInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) failed on dirInode.dirEntryMap.Put(): %v", inHeader.NodeID, string(mkDirIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoMkDir(NodeID==%v,Name=%s) dirInode.dirEntryMap.Put() returned !ok", inHeader.NodeID, string(mkDirIn.Name))
		os.Exit(1)
	}

	dirInode.attr.NLink++

	globals.inodeMap[dirEntInode.attr.Ino] = dirEntInode

	mkDirOut = &fission.MkDirOut{
		EntryOut: fission.EntryOut{
			NodeID:         dirEntInode.attr.Ino,
			Generation:     0,
			EntryValidSec:  entryValidSec,
			AttrValidSec:   attrValidSec,
			EntryValidNSec: entryValidNSec,
			AttrValidNSec:  attrValidNSec,
			Attr: fission.Attr{
				Ino:       dirEntInode.attr.Ino,
				ATimeSec:  dirEntInode.attr.ATimeSec,
				MTimeSec:  dirEntInode.attr.MTimeSec,
				CTimeSec:  dirEntInode.attr.CTimeSec,
				ATimeNSec: dirEntInode.attr.ATimeNSec,
				MTimeNSec: dirEntInode.attr.MTimeNSec,
				CTimeNSec: dirEntInode.attr.CTimeNSec,
				Mode:      dirEntInode.attr.Mode,
				NLink:     dirEntInode.attr.NLink,
				UID:       dirEntInode.attr.UID,
				GID:       dirEntInode.attr.GID,
				RDev:      dirEntInode.attr.RDev,
				Padding:   dirEntInode.attr.Padding,
			},
		},
	}

	fixAttrSizes(&mkDirOut.EntryOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoUnlink(inHeader *fission.InHeader, unlinkIn *fission.UnlinkIn) (errno syscall.Errno) {
	var (
		dirEntInoAsU64   uint64
		dirEntInoAsValue sortedmap.Value
		dirEntInode      *inodeStruct
		dirInode         *inodeStruct
		err              error
		granted          bool
		grantedLockSet   = makeGrantedLockSet()
		ok               bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	dirEntInoAsValue, ok, err = dirInode.dirEntryMap.GetByKey(unlinkIn.Name)
	if err != nil {
		globals.logger.Printf("func DoUnlink(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(unlinkIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	dirEntInoAsU64 = dirEntInoAsValue.(uint64)

	dirEntInode, ok = globals.inodeMap[dirEntInoAsU64]
	if !ok {
		globals.logger.Printf("func DoUnlink(NodeID==%v,Name==%s) failed fetching globals.inodeMap[%v]", inHeader.NodeID, unlinkIn.Name, dirEntInoAsU64)
		os.Exit(1)
	}

	granted = grantedLockSet.try(dirEntInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR == (dirEntInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EISDIR
		return
	}

	ok, err = dirInode.dirEntryMap.DeleteByKey(unlinkIn.Name)
	if err != nil {
		globals.logger.Printf("func DoUnlink(NodeID==%v,Name=%s) failed on .dirEntryMap.DeleteByKey(): %v", inHeader.NodeID, string(unlinkIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoUnlink(NodeID==%v,Name=%s) .dirEntryMap.DeleteByKey() returned !ok", inHeader.NodeID, string(unlinkIn.Name))
		os.Exit(1)
	}

	dirEntInode.attr.NLink--

	if dirEntInode.attr.NLink == 0 {
		delete(globals.inodeMap, dirEntInode.attr.Ino)
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoRmDir(inHeader *fission.InHeader, rmDirIn *fission.RmDirIn) (errno syscall.Errno) {
	var (
		dirEntInoAsU64            uint64
		dirEntInoAsValue          sortedmap.Value
		dirEntInode               *inodeStruct
		dirEntInodeDirEntryMapLen int
		dirInode                  *inodeStruct
		err                       error
		granted                   bool
		grantedLockSet            = makeGrantedLockSet()
		ok                        bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	dirEntInoAsValue, ok, err = dirInode.dirEntryMap.GetByKey(rmDirIn.Name)
	if err != nil {
		globals.logger.Printf("func DoRmDir(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(rmDirIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	dirEntInoAsU64 = dirEntInoAsValue.(uint64)

	dirEntInode, ok = globals.inodeMap[dirEntInoAsU64]
	if !ok {
		globals.logger.Printf("func DoRmDir(NodeID==%v,Name==%s) failed fetching globals.inodeMap[%v]", inHeader.NodeID, rmDirIn.Name, dirEntInoAsU64)
		os.Exit(1)
	}

	granted = grantedLockSet.try(dirEntInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirEntInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	dirEntInodeDirEntryMapLen, err = dirEntInode.dirEntryMap.Len()
	if err != nil {
		globals.logger.Printf("func DoRmDir(NodeID==%v,Name=%s) failed on .dirEntryMap.Len(): %v", inHeader.NodeID, string(rmDirIn.Name), err)
		os.Exit(1)
	}

	if dirEntInodeDirEntryMapLen != 2 {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTEMPTY
		return
	}

	ok, err = dirInode.dirEntryMap.DeleteByKey(rmDirIn.Name)
	if err != nil {
		globals.logger.Printf("func DoRmDir(NodeID==%v,Name=%s) failed on .dirEntryMap.DeleteByKey(): %v", inHeader.NodeID, string(rmDirIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoRmDir(NodeID==%v,Name=%s) .dirEntryMap.DeleteByKey() returned !ok", inHeader.NodeID, string(rmDirIn.Name))
		os.Exit(1)
	}

	dirEntInode.attr.NLink--

	delete(globals.inodeMap, dirEntInode.attr.Ino)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoRename(inHeader *fission.InHeader, renameIn *fission.RenameIn) (errno syscall.Errno) {
	errno = commonRename("DoRename", inHeader.NodeID, renameIn.OldName, renameIn.NewDir, renameIn.NewName)
	return
}

func (*globalsStruct) DoLink(inHeader *fission.InHeader, linkIn *fission.LinkIn) (linkOut *fission.LinkOut, errno syscall.Errno) {
	var (
		dirInode       *inodeStruct
		err            error
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
		oldInode       *inodeStruct
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	_, ok, err = dirInode.dirEntryMap.GetByKey(linkIn.Name)
	if err != nil {
		globals.logger.Printf("func DoLink(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(linkIn.Name), err)
		os.Exit(1)
	}

	if ok {
		grantedLockSet.freeAll(false)
		errno = syscall.EEXIST
		return
	}

	oldInode, ok = globals.inodeMap[linkIn.OldNodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(oldInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR == (oldInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EISDIR
		return
	}

	ok, err = dirInode.dirEntryMap.Put(linkIn.Name, oldInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func DoLink(NodeID==%v,Name=%s) failed on .dirEntryMap.Put(): %v", inHeader.NodeID, string(linkIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoLink(NodeID==%v,Name=%s) .dirEntryMap.Put() returned !ok", inHeader.NodeID, string(linkIn.Name))
		os.Exit(1)
	}

	oldInode.attr.NLink++

	linkOut = &fission.LinkOut{
		EntryOut: fission.EntryOut{
			NodeID:         oldInode.attr.Ino,
			Generation:     0,
			EntryValidSec:  entryValidSec,
			AttrValidSec:   attrValidSec,
			EntryValidNSec: entryValidNSec,
			AttrValidNSec:  attrValidNSec,
			Attr: fission.Attr{
				Ino:       oldInode.attr.Ino,
				Size:      oldInode.attr.Size,
				ATimeSec:  oldInode.attr.ATimeSec,
				MTimeSec:  oldInode.attr.MTimeSec,
				CTimeSec:  oldInode.attr.CTimeSec,
				ATimeNSec: oldInode.attr.ATimeNSec,
				MTimeNSec: oldInode.attr.MTimeNSec,
				CTimeNSec: oldInode.attr.CTimeNSec,
				Mode:      oldInode.attr.Mode,
				NLink:     oldInode.attr.NLink,
				UID:       oldInode.attr.UID,
				GID:       oldInode.attr.GID,
				RDev:      oldInode.attr.RDev,
				Padding:   oldInode.attr.Padding,
			},
		},
	}

	fixAttrSizes(&linkOut.EntryOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoOpen(inHeader *fission.InHeader, openIn *fission.OpenIn) (openOut *fission.OpenOut, errno syscall.Errno) {
	var (
		fileInode      *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFREG != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	if (openIn.Flags & fission.FOpenRequestTRUNC) != 0 {
		fileInode.attr.Size = 0
		fileInode.fileData = make([]byte, 0)
	}

	globals.lastFH++

	globals.fhMap[globals.lastFH] = openIn.Flags

	openOut = &fission.OpenOut{
		FH:        globals.lastFH,
		OpenFlags: fission.FOpenResponseDirectIO,
		Padding:   0,
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoRead(inHeader *fission.InHeader, readIn *fission.ReadIn) (readOut *fission.ReadOut, errno syscall.Errno) {
	var (
		fileInode          *inodeStruct
		fOpenRequestFlags  uint32
		granted            bool
		grantedLockSet     = makeGrantedLockSet()
		ok                 bool
		readOffsetPlusSize uint64
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	fOpenRequestFlags, ok = globals.fhMap[readIn.FH]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}
	if (fOpenRequestFlags & fission.FOpenRequestWRONLY) != 0 {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFREG != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	readOffsetPlusSize = readIn.Offset + uint64(readIn.Size)

	if readIn.Offset < fileInode.attr.Size {
		if readOffsetPlusSize <= fileInode.attr.Size {
			readOut = &fission.ReadOut{
				Data: cloneByteSlice(fileInode.fileData[readIn.Offset:readOffsetPlusSize]),
			}
		} else {
			readOut = &fission.ReadOut{
				Data: cloneByteSlice(fileInode.fileData[readIn.Offset:]),
			}
		}
	} else {
		readOut = &fission.ReadOut{
			Data: make([]byte, 0),
		}
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoWrite(inHeader *fission.InHeader, writeIn *fission.WriteIn) (writeOut *fission.WriteOut, errno syscall.Errno) {
	var (
		fileInode           *inodeStruct
		fOpenRequestFlags   uint32
		granted             bool
		grantedLockSet      = makeGrantedLockSet()
		ok                  bool
		overwriteSize       uint64
		writeOffsetActual   uint64
		writeOffsetPlusSize uint64
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	fOpenRequestFlags, ok = globals.fhMap[writeIn.FH]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}
	if (fOpenRequestFlags & fission.FOpenRequestRDONLY) != 0 {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFREG != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	if (fOpenRequestFlags & fission.FOpenRequestAPPEND) == 0 {
		writeOffsetActual = writeIn.Offset
	} else {
		writeOffsetActual = fileInode.attr.Size
	}

	writeOffsetPlusSize = writeIn.Offset + uint64(writeIn.Size)

	if writeOffsetActual < fileInode.attr.Size {
		if writeOffsetPlusSize <= fileInode.attr.Size {
			_ = copy(fileInode.fileData[writeOffsetActual:writeOffsetPlusSize], writeIn.Data)
		} else {
			overwriteSize = fileInode.attr.Size - writeOffsetActual

			_ = copy(fileInode.fileData[writeOffsetActual:], writeIn.Data[:overwriteSize])
			fileInode.fileData = append(fileInode.fileData, writeIn.Data[overwriteSize:]...)

			fileInode.attr.Size = writeOffsetPlusSize
		}
	} else {
		if writeOffsetActual > fileInode.attr.Size {
			fileInode.fileData = append(fileInode.fileData, make([]byte, (writeOffsetActual-fileInode.attr.Size))...)
		}

		fileInode.fileData = append(fileInode.fileData, writeIn.Data...)

		fileInode.attr.Size = writeOffsetPlusSize
	}

	fileInode.attr.Blocks = fileInode.attr.Size + uint64(attrBlkSize-1)
	fileInode.attr.Blocks /= uint64(attrBlkSize)

	grantedLockSet.freeAll(false)

	writeOut = &fission.WriteOut{
		Size:    writeIn.Size,
		Padding: 0,
	}

	errno = 0
	return
}

func (*globalsStruct) DoStatFS(_ *fission.InHeader) (statFSOut *fission.StatFSOut, errno syscall.Errno) {
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

func (*globalsStruct) DoRelease(inHeader *fission.InHeader, releaseIn *fission.ReleaseIn) (errno syscall.Errno) {
	var (
		fileInode      *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	_, ok = globals.fhMap[releaseIn.FH]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFREG != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	delete(globals.fhMap, releaseIn.FH)

	if fileInode.attr.NLink == 0 {
		delete(globals.inodeMap, inHeader.NodeID)
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoFSync(inHeader *fission.InHeader, _ *fission.FSyncIn) (errno syscall.Errno) {
	var (
		fileInode      *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFREG != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoSetXAttr(inHeader *fission.InHeader, setXAttrIn *fission.SetXAttrIn) (errno syscall.Errno) {
	var (
		err            error
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		inode          *inodeStruct
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	ok, err = inode.xattrMap.PatchByKey(setXAttrIn.Name, setXAttrIn.Data)
	if err != nil {
		globals.logger.Printf("func DoSetXAttr(NodeID==%v, Name==%s) failed on .xattrMap.PatchByKey(): %v", inHeader.NodeID, string(setXAttrIn.Name), err)
		os.Exit(1)
	}

	if !ok {
		ok, err = inode.xattrMap.Put(setXAttrIn.Name, setXAttrIn.Data)
		if err != nil {
			globals.logger.Printf("func DoSetXAttr(NodeID==%v, Name==%s) failed on .xattrMap.Put(): %v", inHeader.NodeID, string(setXAttrIn.Name), err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func DoSetXAttr(NodeID==%v, Name==%s) .xattrMap.Put() returned !ok", inHeader.NodeID, string(setXAttrIn.Name))
			os.Exit(1)
		}
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoGetXAttr(inHeader *fission.InHeader, getXAttrIn *fission.GetXAttrIn) (getXAttrOut *fission.GetXAttrOut, errno syscall.Errno) {
	var (
		dataAsByteSlice []byte
		dataAsValue     sortedmap.Value
		err             error
		granted         bool
		grantedLockSet  = makeGrantedLockSet()
		inode           *inodeStruct
		ok              bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	dataAsValue, ok, err = inode.xattrMap.GetByKey(getXAttrIn.Name)
	if err != nil {
		globals.logger.Printf("func DoGetXAttr(NodeID==%v) failed on .xattrMap.GetByKey(): %v", inHeader.NodeID, err)
		os.Exit(1)
	}

	grantedLockSet.freeAll(false)

	if !ok {
		errno = syscall.ENODATA
		return
	}

	dataAsByteSlice = dataAsValue.([]byte)

	if getXAttrIn.Size == 0 {
		getXAttrOut = &fission.GetXAttrOut{
			Size:    uint32(len(dataAsByteSlice)),
			Padding: 0,
			Data:    make([]byte, 0),
		}
		errno = 0
		return
	}

	if uint32(len(dataAsByteSlice)) > getXAttrIn.Size {
		errno = syscall.ERANGE
		return
	}

	getXAttrOut = &fission.GetXAttrOut{
		Size:    uint32(len(dataAsByteSlice)),
		Padding: 0,
		Data:    cloneByteSlice(dataAsByteSlice),
	}

	errno = 0
	return
}

func (*globalsStruct) DoListXAttr(inHeader *fission.InHeader, listXAttrIn *fission.ListXAttrIn) (listXAttrOut *fission.ListXAttrOut, errno syscall.Errno) {
	var (
		err                  error
		granted              bool
		grantedLockSet       = makeGrantedLockSet()
		inode                *inodeStruct
		ok                   bool
		totalSize            uint32
		xattrCount           int
		xattrIndex           int
		xattrNameAsByteSlice []byte
		xattrNameAsKey       sortedmap.Key
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	listXAttrOut = &fission.ListXAttrOut{
		Size:    0,
		Padding: 0,
		Name:    make([][]byte, 0),
	}

	xattrCount, err = inode.xattrMap.Len()
	if err != nil {
		globals.logger.Printf("func DoListXAttr(NodeID==%v) failed on .dirEntryMap.Len(): %v", inHeader.NodeID, err)
		os.Exit(1)
	}

	totalSize = 0

	for xattrIndex = range xattrCount {
		xattrNameAsKey, _, ok, err = inode.xattrMap.GetByIndex(xattrIndex)
		if err != nil {
			globals.logger.Printf("func DoGetXAttr(NodeID==%v) failed on .xattrMap.GetByIndex(%d): %v", inHeader.NodeID, xattrIndex, err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func DoGetXAttr(NodeID==%v) .xattrMap.GetByIndex(%d) returned !ok", inHeader.NodeID, xattrIndex)
			os.Exit(1)
		}

		xattrNameAsByteSlice = xattrNameAsKey.([]byte)

		if listXAttrIn.Size != 0 {
			if (totalSize + uint32(len(xattrNameAsByteSlice)+1)) > listXAttrIn.Size {
				grantedLockSet.freeAll(false)
				errno = syscall.ERANGE
				return
			}
		}

		totalSize += uint32(len(xattrNameAsByteSlice) + 1)

		if listXAttrIn.Size != 0 {
			listXAttrOut.Name = append(listXAttrOut.Name, xattrNameAsByteSlice)
		}
	}

	grantedLockSet.freeAll(false)

	listXAttrOut.Size = totalSize

	errno = 0
	return
}

func (*globalsStruct) DoRemoveXAttr(inHeader *fission.InHeader, removeXAttrIn *fission.RemoveXAttrIn) (errno syscall.Errno) {
	var (
		err            error
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		inode          *inodeStruct
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	grantedLockSet.free(globals.tryLock)

	ok, err = inode.xattrMap.DeleteByKey(removeXAttrIn.Name)
	if err != nil {
		globals.logger.Printf("func DoRemoveXAttr(NodeID==%v, Name==%s) failed on .xattrMap.DeleteByKey(): %v", inHeader.NodeID, string(removeXAttrIn.Name), err)
		os.Exit(1)
	}

	grantedLockSet.freeAll(false)

	if !ok {
		errno = syscall.ENOENT
		return
	}

	errno = 0
	return
}

func (*globalsStruct) DoFlush(inHeader *fission.InHeader, _ *fission.FlushIn) (errno syscall.Errno) {
	var (
		fileInode      *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFREG != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.EINVAL
		return
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoInit(_ *fission.InHeader, initIn *fission.InitIn) (initOut *fission.InitOut, errno syscall.Errno) {
	initOut = &fission.InitOut{
		Major:                initIn.Major,
		Minor:                initIn.Minor,
		MaxReadAhead:         initIn.MaxReadAhead,
		Flags:                initOutFlagsNearlyAll,
		MaxBackground:        initOutMaxBackgound,
		CongestionThreshhold: initOutCongestionThreshhold,
		MaxWrite:             maxWrite,
		TimeGran:             0, // accept default
		MaxPages:             maxPages,
		MapAlignment:         0, // accept default
		Flags2:               0,
		MaxStackDepth:        0,
		RequestTimeout:       0,
		Unused:               [11]uint16{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}

	errno = 0
	return
}

func (*globalsStruct) DoOpenDir(inHeader *fission.InHeader, _ *fission.OpenDirIn) (openDirOut *fission.OpenDirOut, errno syscall.Errno) {
	var (
		dirInode       *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	openDirOut = &fission.OpenDirOut{
		FH:        0,
		OpenFlags: 0,
		Padding:   0,
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoReadDir(inHeader *fission.InHeader, readDirIn *fission.ReadDirIn) (readDirOut *fission.ReadDirOut, errno syscall.Errno) {
	var (
		dirEntCount           int
		dirEntIndex           int
		dirEntInoAsU64        uint64
		dirEntInoAsValue      sortedmap.Value
		dirEntInode           *inodeStruct
		dirEntNameAsByteSlice []byte
		dirEntNameAsKey       sortedmap.Key
		dirEntNameLenAligned  uint32
		dirEntSize            uint32
		dirEntType            uint32
		dirInode              *inodeStruct
		err                   error
		granted               bool
		grantedLockSet        = makeGrantedLockSet()
		totalSize             uint32
		ok                    bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	dirEntCount, err = dirInode.dirEntryMap.Len()
	if err != nil {
		globals.logger.Printf("func DoReadDir(NodeID==%v) failed on .dirEntryMap.Len(): %v", inHeader.NodeID, err)
		os.Exit(1)
	}

	if uint64(dirEntCount) < readDirIn.Offset {
		// Just return an empty ReadDirOut

		grantedLockSet.freeAll(false)

		readDirOut = &fission.ReadDirOut{
			DirEnt: make([]fission.DirEnt, 0),
		}

		errno = 0
		return
	}

	// Just compute the maximal ReadDirOut... we'll prune it later

	readDirOut = &fission.ReadDirOut{
		DirEnt: make([]fission.DirEnt, dirEntCount),
	}

	for dirEntIndex = range dirEntCount {
		dirEntNameAsKey, dirEntInoAsValue, ok, err = dirInode.dirEntryMap.GetByIndex(dirEntIndex)
		if err != nil {
			globals.logger.Printf("func DoReadDir(NodeID==%v) failed on .dirEntryMap.GetByIndex(): %v", inHeader.NodeID, err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func DoReadDir(NodeID==%v) .dirEntryMap.GetByIndex() returned !ok", inHeader.NodeID)
			os.Exit(1)
		}

		dirEntNameAsByteSlice = dirEntNameAsKey.([]byte)
		dirEntInoAsU64 = dirEntInoAsValue.(uint64)

		dirEntInode, ok = globals.inodeMap[dirEntInoAsU64]
		if !ok {
			globals.logger.Printf("func DoReadDir(NodeID==%v) failed fetching globals.inodeMap[%v]", inHeader.NodeID, dirEntInoAsU64)
			os.Exit(1)
		}

		granted = grantedLockSet.try(dirEntInode.tryLock)
		if !granted {
			grantedLockSet.freeAll(true)
			goto Restart
		}

		if (dirEntInode.attr.Mode & syscall.S_IFMT) == syscall.S_IFDIR {
			dirEntType = syscall.DT_DIR
		} else {
			dirEntType = syscall.DT_REG
		}

		readDirOut.DirEnt[dirEntIndex] = fission.DirEnt{
			Ino:     dirEntInode.attr.Ino,
			Off:     uint64(dirEntIndex) + 1,
			NameLen: uint32(len(dirEntNameAsByteSlice)), // unnecessary
			Type:    dirEntType,
			Name:    cloneByteSlice(dirEntNameAsByteSlice),
		}
	}

	grantedLockSet.freeAll(false)

	// Now prune on the left to readDirIn.Offset & on the right anything beyond readDirIn.Size

	readDirOut.DirEnt = readDirOut.DirEnt[readDirIn.Offset:]

	totalSize = 0

	for dirEntIndex = 0; dirEntIndex < len(readDirOut.DirEnt); dirEntIndex++ {
		dirEntNameLenAligned = (uint32(len(readDirOut.DirEnt[dirEntIndex].Name)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
		dirEntSize = fission.DirEntFixedPortionSize + dirEntNameLenAligned

		if (totalSize + dirEntSize) > readDirIn.Size {
			// Truncate readDirOut here and return

			readDirOut.DirEnt = readDirOut.DirEnt[:dirEntIndex]

			errno = 0
			return
		}

		totalSize += dirEntSize
	}

	errno = 0
	return
}

func (*globalsStruct) DoReleaseDir(inHeader *fission.InHeader, _ *fission.ReleaseDirIn) (errno syscall.Errno) {
	var (
		dirInode       *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	if dirInode.attr.NLink == 0 {
		delete(globals.inodeMap, inHeader.NodeID)
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoFSyncDir(inHeader *fission.InHeader, _ *fission.FSyncDirIn) (errno syscall.Errno) {
	var (
		fileInode      *inodeStruct
		granted        bool
		grantedLockSet = makeGrantedLockSet()
		ok             bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	fileInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(fileInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (fileInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoGetLK(_ *fission.InHeader, _ *fission.GetLKIn) (getLKOut *fission.GetLKOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoSetLK(_ *fission.InHeader, _ *fission.SetLKIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoSetLKW(_ *fission.InHeader, _ *fission.SetLKWIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoAccess(inHeader *fission.InHeader, accessIn *fission.AccessIn) (errno syscall.Errno) {
	var (
		executeGrantedOrNotRequested bool
		executeRequested             bool
		granted                      bool
		grantedLockSet               = makeGrantedLockSet()
		inode                        *inodeStruct
		inodeAttrGID                 uint32
		inodeAttrMode                uint32
		inodeAttrModeGroup           uint32
		inodeAttrModeOther           uint32
		inodeAttrModeOwner           uint32
		inodeAttrUID                 uint32
		isInodeGroup                 bool
		isInodeOwner                 bool
		isRoot                       bool
		ok                           bool
		readGrantedOrNotRequested    bool
		readRequested                bool
		writeGrantedOrNotRequested   bool
		writeRequested               bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	inode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(inode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	inodeAttrUID = inode.attr.UID
	inodeAttrGID = inode.attr.GID

	inodeAttrMode = inode.attr.Mode

	inodeAttrModeOwner = inodeAttrMode >> accessOwnerShift
	inodeAttrModeGroup = inodeAttrMode >> accessGroupShift
	inodeAttrModeOther = inodeAttrMode >> accessOtherShift

	grantedLockSet.freeAll(false)

	isRoot = (inHeader.UID == uint32(0))

	isInodeOwner = (inHeader.UID == inodeAttrUID)
	isInodeGroup = (inHeader.GID == inodeAttrGID)

	readRequested = ((accessIn.Mask & accessROK) != 0)
	writeRequested = ((accessIn.Mask & accessWOK) != 0)
	executeRequested = ((accessIn.Mask & accessXOK) != 0)

	if readRequested {
		if isRoot {
			readGrantedOrNotRequested = true
		} else {
			readGrantedOrNotRequested = false
			if isInodeOwner && ((inodeAttrModeOwner & accessROK) != 0) {
				readGrantedOrNotRequested = true
			}
			if isInodeGroup && ((inodeAttrModeGroup & accessROK) != 0) {
				readGrantedOrNotRequested = true
			}
			if (inodeAttrModeOther & accessROK) != 0 {
				readGrantedOrNotRequested = true
			}
		}
	} else {
		readGrantedOrNotRequested = true
	}

	if writeRequested {
		if isRoot {
			writeGrantedOrNotRequested = true
		} else {
			writeGrantedOrNotRequested = false
			if isInodeOwner && ((inodeAttrModeOwner & accessWOK) != 0) {
				writeGrantedOrNotRequested = true
			}
			if isInodeGroup && ((inodeAttrModeGroup & accessWOK) != 0) {
				writeGrantedOrNotRequested = true
			}
			if (inodeAttrModeOther & accessWOK) != 0 {
				writeGrantedOrNotRequested = true
			}
		}
	} else {
		writeGrantedOrNotRequested = true
	}

	if executeRequested {
		if isRoot {
			executeGrantedOrNotRequested = false
			if (inodeAttrModeOwner & accessXOK) != 0 {
				executeGrantedOrNotRequested = true
			}
			if (inodeAttrModeGroup & accessXOK) != 0 {
				executeGrantedOrNotRequested = true
			}
			if (inodeAttrModeOther & accessXOK) != 0 {
				executeGrantedOrNotRequested = true
			}
		} else {
			executeGrantedOrNotRequested = false
			if isInodeOwner && ((inodeAttrModeOwner & accessXOK) != 0) {
				executeGrantedOrNotRequested = true
			}
			if isInodeGroup && ((inodeAttrModeGroup & accessXOK) != 0) {
				executeGrantedOrNotRequested = true
			}
			if (inodeAttrModeOther & accessXOK) != 0 {
				executeGrantedOrNotRequested = true
			}
		}
	} else {
		executeGrantedOrNotRequested = true
	}

	if readGrantedOrNotRequested && writeGrantedOrNotRequested && executeGrantedOrNotRequested {
		errno = 0
	} else {
		errno = syscall.EACCES
	}

	return
}

func (*globalsStruct) DoCreate(inHeader *fission.InHeader, createIn *fission.CreateIn) (createOut *fission.CreateOut, errno syscall.Errno) {
	var (
		dirInode        *inodeStruct
		err             error
		fileInode       *inodeStruct
		fileInodeMode   uint32
		granted         bool
		grantedLockSet  = makeGrantedLockSet()
		ok              bool
		unixTimeNowNSec uint32
		unixTimeNowSec  uint64
	)

	fileInodeMode = uint32(syscall.S_IRWXU | syscall.S_IRWXG | syscall.S_IRWXO)
	fileInodeMode &= createIn.Mode
	fileInodeMode &= ^createIn.UMask
	fileInodeMode |= syscall.S_IFREG

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	_, ok, err = dirInode.dirEntryMap.GetByKey(createIn.Name)
	if err != nil {
		globals.logger.Printf("func DoCreate(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(createIn.Name), err)
		os.Exit(1)
	}

	if ok {
		grantedLockSet.freeAll(false)
		errno = syscall.EEXIST
		return
	}

	globals.lastNodeID++

	unixTimeNowSec, unixTimeNowNSec = unixTimeNow()

	fileInode = &inodeStruct{
		tryLock: makeTryLock(),
		attr: fission.Attr{
			Ino:       globals.lastNodeID,
			Size:      0,
			ATimeSec:  unixTimeNowSec,
			MTimeSec:  unixTimeNowSec,
			CTimeSec:  unixTimeNowSec,
			ATimeNSec: unixTimeNowNSec,
			MTimeNSec: unixTimeNowNSec,
			CTimeNSec: unixTimeNowNSec,
			Mode:      fileInodeMode,
			NLink:     1,
			UID:       inHeader.UID,
			GID:       inHeader.GID,
			RDev:      0,
			Padding:   0,
		},
		xattrMap:    sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.xattrMapDummy),
		dirEntryMap: nil,
		fileData:    make([]byte, 0),
		symlinkData: nil,
	}

	fixAttrSizes(&fileInode.attr)

	ok, err = dirInode.dirEntryMap.Put(createIn.Name, fileInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func DoCreate(NodeID==%v,Name=%s) failed on .dirEntryMap.Put(): %v", inHeader.NodeID, string(createIn.Name), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoCreate(NodeID==%v,Name=%s) .dirEntryMap.Put() returned !ok", inHeader.NodeID, string(createIn.Name))
		os.Exit(1)
	}

	globals.inodeMap[fileInode.attr.Ino] = fileInode

	globals.lastFH++

	globals.fhMap[globals.lastFH] = createIn.Flags

	createOut = &fission.CreateOut{
		EntryOut: fission.EntryOut{
			NodeID:         fileInode.attr.Ino,
			Generation:     0,
			EntryValidSec:  entryValidSec,
			AttrValidSec:   attrValidSec,
			EntryValidNSec: entryValidNSec,
			AttrValidNSec:  attrValidNSec,
			Attr: fission.Attr{
				Ino:       fileInode.attr.Ino,
				Size:      fileInode.attr.Size,
				ATimeSec:  fileInode.attr.ATimeSec,
				MTimeSec:  fileInode.attr.MTimeSec,
				CTimeSec:  fileInode.attr.CTimeSec,
				ATimeNSec: fileInode.attr.ATimeNSec,
				MTimeNSec: fileInode.attr.MTimeNSec,
				CTimeNSec: fileInode.attr.CTimeNSec,
				Mode:      fileInode.attr.Mode,
				NLink:     fileInode.attr.NLink,
				UID:       fileInode.attr.UID,
				GID:       fileInode.attr.GID,
				RDev:      fileInode.attr.RDev,
				Padding:   fileInode.attr.Padding,
			},
		},
		FH:        globals.lastFH,
		OpenFlags: fission.FOpenResponseDirectIO,
		Padding:   0,
	}

	fixAttrSizes(&createOut.EntryOut.Attr)

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (*globalsStruct) DoInterrupt(_ *fission.InHeader, _ *fission.InterruptIn) {
}

func (*globalsStruct) DoBMap(_ *fission.InHeader, _ *fission.BMapIn) (bMapOut *fission.BMapOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoDestroy(_ *fission.InHeader) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoPoll(_ *fission.InHeader, _ *fission.PollIn) (pollOut *fission.PollOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoBatchForget(_ *fission.InHeader, _ *fission.BatchForgetIn) {
}

func (*globalsStruct) DoFAllocate(_ *fission.InHeader, _ *fission.FAllocateIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoReadDirPlus(inHeader *fission.InHeader, readDirPlusIn *fission.ReadDirPlusIn) (readDirPlusOut *fission.ReadDirPlusOut, errno syscall.Errno) {
	var (
		dirEntInoAsU64        uint64
		dirEntInoAsValue      sortedmap.Value
		dirEntInode           *inodeStruct
		dirEntNameAsByteSlice []byte
		dirEntNameAsKey       sortedmap.Key
		dirEntNameLenAligned  uint32
		dirEntSize            uint32
		dirEntPlusCount       int
		dirEntPlusIndex       int
		dirEntType            uint32
		dirInode              *inodeStruct
		err                   error
		granted               bool
		grantedLockSet        = makeGrantedLockSet()
		totalSize             uint32
		ok                    bool
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	dirInode, ok = globals.inodeMap[inHeader.NodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(dirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (dirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	dirEntPlusCount, err = dirInode.dirEntryMap.Len()
	if err != nil {
		globals.logger.Printf("func DoReadDirPlus(NodeID==%v) failed on .dirEntryMap.Len(): %v", inHeader.NodeID, err)
		os.Exit(1)
	}

	if uint64(dirEntPlusCount) < readDirPlusIn.Offset {
		// Just return an empty ReadDirPlusOut

		grantedLockSet.freeAll(false)

		readDirPlusOut = &fission.ReadDirPlusOut{
			DirEntPlus: make([]fission.DirEntPlus, 0),
		}

		errno = 0
		return
	}

	// Just compute the maximal ReadDirPlusOut... we'll prune it later

	readDirPlusOut = &fission.ReadDirPlusOut{
		DirEntPlus: make([]fission.DirEntPlus, dirEntPlusCount),
	}

	for dirEntPlusIndex = range dirEntPlusCount {
		dirEntNameAsKey, dirEntInoAsValue, ok, err = dirInode.dirEntryMap.GetByIndex(dirEntPlusIndex)
		if err != nil {
			globals.logger.Printf("func DoReadDirPlus(NodeID==%v) failed on .dirEntryMap.GetByIndex(): %v", inHeader.NodeID, err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func DoReadDirPlus(NodeID==%v) .dirEntryMap.GetByIndex() returned !ok", inHeader.NodeID)
			os.Exit(1)
		}

		dirEntNameAsByteSlice = dirEntNameAsKey.([]byte)
		dirEntInoAsU64 = dirEntInoAsValue.(uint64)

		dirEntInode, ok = globals.inodeMap[dirEntInoAsU64]
		if !ok {
			globals.logger.Printf("func DoReadDirPlus(NodeID==%v) failed fetching globals.inodeMap[%v]", inHeader.NodeID, dirEntInoAsU64)
			os.Exit(1)
		}

		granted = grantedLockSet.try(dirEntInode.tryLock)
		if !granted {
			grantedLockSet.freeAll(true)
			goto Restart
		}

		if (dirEntInode.attr.Mode & syscall.S_IFMT) == syscall.S_IFDIR {
			dirEntType = syscall.DT_DIR
		} else {
			dirEntType = syscall.DT_REG
		}

		readDirPlusOut.DirEntPlus[dirEntPlusIndex] = fission.DirEntPlus{
			EntryOut: fission.EntryOut{
				NodeID:         dirEntInode.attr.Ino,
				Generation:     0,
				EntryValidSec:  entryValidSec,
				AttrValidSec:   attrValidSec,
				EntryValidNSec: entryValidNSec,
				AttrValidNSec:  attrValidNSec,
				Attr: fission.Attr{
					Ino:       dirEntInode.attr.Ino,
					Size:      dirEntInode.attr.Size,
					ATimeSec:  dirEntInode.attr.ATimeSec,
					MTimeSec:  dirEntInode.attr.MTimeSec,
					CTimeSec:  dirEntInode.attr.CTimeSec,
					ATimeNSec: dirEntInode.attr.ATimeNSec,
					MTimeNSec: dirEntInode.attr.MTimeNSec,
					CTimeNSec: dirEntInode.attr.CTimeNSec,
					Mode:      dirEntInode.attr.Mode,
					NLink:     dirEntInode.attr.NLink,
					UID:       dirEntInode.attr.UID,
					GID:       dirEntInode.attr.GID,
					RDev:      dirEntInode.attr.RDev,
					Padding:   dirEntInode.attr.Padding,
				},
			},
			DirEnt: fission.DirEnt{
				Ino:     dirEntInode.attr.Ino,
				Off:     uint64(dirEntPlusIndex) + 1,
				NameLen: uint32(len(dirEntNameAsByteSlice)), // unnecessary
				Type:    dirEntType,
				Name:    cloneByteSlice(dirEntNameAsByteSlice),
			},
		}

		fixAttrSizes(&readDirPlusOut.DirEntPlus[dirEntPlusIndex].EntryOut.Attr)
	}

	grantedLockSet.freeAll(false)

	// Now prune on the left to readDirPlusIn.Offset & on the right anything beyond readDirPlusIn.Size

	readDirPlusOut.DirEntPlus = readDirPlusOut.DirEntPlus[readDirPlusIn.Offset:]

	totalSize = 0

	for dirEntPlusIndex = 0; dirEntPlusIndex < len(readDirPlusOut.DirEntPlus); dirEntPlusIndex++ {
		dirEntNameLenAligned = (uint32(len(readDirPlusOut.DirEntPlus[dirEntPlusIndex].Name)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
		dirEntSize = fission.DirEntPlusFixedPortionSize + dirEntNameLenAligned

		if (totalSize + dirEntSize) > readDirPlusIn.Size {
			// Truncate readDirPlusOut here and return

			readDirPlusOut.DirEntPlus = readDirPlusOut.DirEntPlus[:dirEntPlusIndex]

			errno = 0
			return
		}

		totalSize += dirEntSize
	}

	errno = 0
	return
}

func (*globalsStruct) DoRename2(inHeader *fission.InHeader, rename2In *fission.Rename2In) (errno syscall.Errno) {
	errno = commonRename("DoRename2", inHeader.NodeID, rename2In.OldName, rename2In.NewDir, rename2In.NewName)
	return
}

func (*globalsStruct) DoLSeek(_ *fission.InHeader, _ *fission.LSeekIn) (lSeekOut *fission.LSeekOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoStatX(_ *fission.InHeader, _ *fission.StatXIn) (lSeekOut *fission.StatXOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func fixAttrSizes(attr *fission.Attr) {
	if syscall.S_IFREG == (attr.Mode & syscall.S_IFMT) {
		attr.Blocks = attr.Size + (uint64(attrBlkSize) - 1)
		attr.Blocks /= uint64(attrBlkSize)
		attr.BlkSize = attrBlkSize
	} else {
		attr.Size = 0
		attr.Blocks = 0
		attr.BlkSize = 0
	}
}

func commonRename(callerName string, oldDirNodeID uint64, oldName []byte, newDirNodeID uint64, newName []byte) (errno syscall.Errno) {
	var (
		err                         error
		granted                     bool
		grantedLockSet              = makeGrantedLockSet()
		movedInode                  *inodeStruct
		movedInodeNodeIDAsU64       uint64
		movedInodeNodeIDAsValue     sortedmap.Value
		newDirInode                 *inodeStruct
		ok                          bool
		oldDirInode                 *inodeStruct
		replacedInode               *inodeStruct
		replacedInodeDirEntryMapLen int
		replacedInodeNodeIDAsU64    uint64
		replacedInodeNodeIDAsValue  sortedmap.Value
	)

Restart:
	grantedLockSet.get(globals.tryLock)

	oldDirInode, ok = globals.inodeMap[oldDirNodeID]
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	granted = grantedLockSet.try(oldDirInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	if syscall.S_IFDIR != (oldDirInode.attr.Mode & syscall.S_IFMT) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOTDIR
		return
	}

	if oldDirNodeID == newDirNodeID {
		newDirInode = oldDirInode
	} else {
		newDirInode, ok = globals.inodeMap[newDirNodeID]
		if !ok {
			grantedLockSet.freeAll(false)
			errno = syscall.ENOENT
			return
		}

		granted = grantedLockSet.try(newDirInode.tryLock)
		if !granted {
			grantedLockSet.freeAll(true)
			goto Restart
		}

		if syscall.S_IFDIR != (newDirInode.attr.Mode & syscall.S_IFMT) {
			grantedLockSet.freeAll(false)
			errno = syscall.ENOTDIR
			return
		}
	}

	movedInodeNodeIDAsValue, ok, err = oldDirInode.dirEntryMap.GetByKey(oldName)
	if err != nil {
		globals.logger.Printf("func %s(,OldName=%s) failed on .dirEntryMap.GetByKey(): %v", callerName, string(oldName), err)
		os.Exit(1)
	}
	if !ok {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOENT
		return
	}

	movedInodeNodeIDAsU64 = movedInodeNodeIDAsValue.(uint64)

	movedInode, ok = globals.inodeMap[movedInodeNodeIDAsU64]
	if !ok {
		globals.logger.Printf("func %s(,OldName=%s) globals.inodeMap[movedInodeNodeIDAsU64] returned !ok", callerName, string(oldName))
		os.Exit(1)
	}

	granted = grantedLockSet.try(movedInode.tryLock)
	if !granted {
		grantedLockSet.freeAll(true)
		goto Restart
	}

	replacedInodeNodeIDAsValue, ok, err = newDirInode.dirEntryMap.GetByKey(newName)
	if err != nil {
		globals.logger.Printf("func %s(,NewName=%s) failed on .dirEntryMap.GetByKey(): %v", callerName, string(newName), err)
		os.Exit(1)
	}

	if ok {
		replacedInodeNodeIDAsU64 = replacedInodeNodeIDAsValue.(uint64)

		replacedInode, ok = globals.inodeMap[replacedInodeNodeIDAsU64]
		if !ok {
			globals.logger.Printf("func %s(,NewName=%s) globals.inodeMap[replacedInodeNodeIDAsU64] returned !ok", callerName, string(newName))
			os.Exit(1)
		}

		granted = grantedLockSet.try(movedInode.tryLock)
		if !granted {
			grantedLockSet.freeAll(true)
			goto Restart
		}
	} else {
		replacedInode = nil
	}

	if syscall.S_IFDIR == (movedInode.attr.Mode & syscall.S_IFMT) {
		if replacedInode != nil {
			if syscall.S_IFDIR != (movedInode.attr.Mode & syscall.S_IFMT) {
				grantedLockSet.freeAll(false)
				errno = syscall.ENOTDIR
				return
			}

			replacedInodeDirEntryMapLen, err = replacedInode.dirEntryMap.Len()
			if err != nil {
				globals.logger.Printf("func %s(,NewName=%s) failed on .dirEntryMap.Len(): %v", callerName, string(newName), err)
				os.Exit(1)
			}

			if replacedInodeDirEntryMapLen != 2 {
				grantedLockSet.freeAll(false)
				errno = syscall.EEXIST
				return
			}

			ok, err = newDirInode.dirEntryMap.DeleteByKey(newName)
			if err != nil {
				globals.logger.Printf("func %s(,[Dir]NewName=%s) failed on .dirEntryMap.DeleteByKey(): %v", callerName, string(newName), err)
				os.Exit(1)
			}
			if !ok {
				globals.logger.Printf("func %s(,[Dir]NewName=%s) .dirEntryMap.DeleteByKey() returned !ok", callerName, string(newName))
				os.Exit(1)
			}

			newDirInode.attr.NLink--

			delete(globals.inodeMap, replacedInode.attr.Ino)
		}

		oldDirInode.attr.NLink--
		newDirInode.attr.NLink++

		ok, err = movedInode.dirEntryMap.PatchByKey([]byte(".."), newDirInode.attr.Ino)
		if err != nil {
			globals.logger.Printf("func %s() failed on .dirEntryMap.PatchByKey(): %v", callerName, err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func %s() .dirEntryMap.PatchByKey() returned !ok", callerName)
			os.Exit(1)
		}
	} else if replacedInode != nil {
		if syscall.S_IFDIR == (movedInode.attr.Mode & syscall.S_IFMT) {
			grantedLockSet.freeAll(false)
			errno = syscall.EISDIR
			return
		}

		ok, err = newDirInode.dirEntryMap.DeleteByKey(newName)
		if err != nil {
			globals.logger.Printf("func %s(,[Non-Dir]NewName=%s) failed on .dirEntryMap.DeleteByKey(): %v", callerName, string(newName), err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func %s(,[Non-Dir]NewName=%s) .dirEntryMap.DeleteByKey() returned !ok", callerName, string(newName))
			os.Exit(1)
		}

		replacedInode.attr.NLink--

		if replacedInode.attr.NLink == 0 {
			delete(globals.inodeMap, replacedInode.attr.Ino)
		}
	}

	ok, err = oldDirInode.dirEntryMap.DeleteByKey(oldName)
	if err != nil {
		globals.logger.Printf("func %s(,OldName=%s) failed on .dirEntryMap.DeleteByKey(): %v", callerName, string(oldName), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func %s() .dirEntryMap.DeleteByKey(,OldName=%s) returned !ok", callerName, string(oldName))
		os.Exit(1)
	}

	ok, err = newDirInode.dirEntryMap.Put(newName, movedInode.attr.Ino)
	if err != nil {
		globals.logger.Printf("func %s(,OldName=%s) failed on .dirEntryMap.Put(): %v", callerName, string(newName), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func %s(,NewName=%s) .dirEntryMap.Put() returned !ok", callerName, string(newName))
		os.Exit(1)
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}
