package main

import (
	"os"
	"syscall"

	"github.com/swiftstack/sortedmap"

	"github.com/swiftstack/fission"
)

func (dummy *globalsStruct) DoLookup(inHeader *fission.InHeader, lookupIn *fission.LookupIn) (lookupOut *fission.LookupOut, errno syscall.Errno) {
	var (
		dirEntInoAsValue sortedmap.Value
		dirEntInoAsU64   uint64
		dirEntInode      *inodeStruct
		dirInode         *inodeStruct
		err              error
		granted          bool
		grantedLockSet   *grantedLockSetStruct = makeGrantedLockSet()
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
	if nil != err {
		globals.logger.Printf("func DoLookup(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(lookupIn.Name[:]), err)
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
			EntryValidSec:  0,
			AttrValidSec:   0,
			EntryValidNSec: 0,
			AttrValidNSec:  0,
			Attr: fission.Attr{
				Ino:       dirEntInode.attr.Ino,
				Size:      dirEntInode.attr.Size,
				Blocks:    dirEntInode.attr.Blocks,
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
				BlkSize:   dirEntInode.attr.BlkSize,
				Padding:   dirEntInode.attr.Padding,
			},
		},
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (dummy *globalsStruct) DoForget(inHeader *fission.InHeader, forgetIn *fission.ForgetIn) {
	return
}

func (dummy *globalsStruct) DoGetAttr(inHeader *fission.InHeader, getAttrIn *fission.GetAttrIn) (getAttrOut *fission.GetAttrOut, errno syscall.Errno) {
	var (
		granted        bool
		grantedLockSet *grantedLockSetStruct = makeGrantedLockSet()
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
		AttrValidSec:  0,
		AttrValidNSec: 0,
		Dummy:         0,
		Attr: fission.Attr{
			Ino:       inode.attr.Ino,
			Size:      inode.attr.Size,
			Blocks:    inode.attr.Blocks,
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
			BlkSize:   inode.attr.BlkSize,
			Padding:   inode.attr.Padding,
		},
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (dummy *globalsStruct) DoSetAttr(inHeader *fission.InHeader, setAttrIn *fission.SetAttrIn) (setAttrOut *fission.SetAttrOut, errno syscall.Errno) {
	var (
		granted         bool
		grantedLockSet  *grantedLockSetStruct = makeGrantedLockSet()
		inode           *inodeStruct
		inodeAttrMode   uint32
		ok              bool
		setAttrInMode   uint32
		unixTimeNowNSec uint32
		unixTimeNowSec  uint64
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

	unixTimeNowSec, unixTimeNowNSec = unixTimeNow()

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidMode) {
		inodeAttrMode = inode.attr.Mode & ^uint32(syscall.S_IRWXU|syscall.S_IRWXG|syscall.S_IRWXO)
		setAttrInMode = setAttrIn.Mode & uint32(syscall.S_IRWXU|syscall.S_IRWXG|syscall.S_IRWXO)
		inodeAttrMode |= setAttrInMode

		inode.attr.Mode = inodeAttrMode
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidUID) {
		inode.attr.UID = setAttrIn.UID
	}
	if 0 != (setAttrIn.Valid & fission.SetAttrInValidGID) {
		inode.attr.GID = setAttrIn.GID
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidSize) {
		if 0 != (inode.attr.Mode & syscall.S_IFREG) {
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
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidATime) {
		inode.attr.ATimeSec = setAttrIn.ATimeSec
		inode.attr.ATimeNSec = setAttrIn.ATimeNSec
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidMTime) {
		inode.attr.MTimeSec = setAttrIn.MTimeSec
		inode.attr.MTimeNSec = setAttrIn.MTimeNSec
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidFH) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOSYS
		return
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidATimeNow) {
		inode.attr.ATimeSec = unixTimeNowSec
		inode.attr.ATimeNSec = unixTimeNowNSec
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidMTimeNow) {
		inode.attr.MTimeSec = unixTimeNowSec
		inode.attr.MTimeNSec = unixTimeNowNSec
	}

	if 0 != (setAttrIn.Valid & fission.SetAttrInValidLockOwner) {
		grantedLockSet.freeAll(false)
		errno = syscall.ENOSYS
	}

	setAttrOut = &fission.SetAttrOut{
		AttrValidSec:  0,
		AttrValidNSec: 0,
		Dummy:         0,
		Attr: fission.Attr{
			Ino:       inode.attr.Ino,
			Size:      inode.attr.Size,
			Blocks:    inode.attr.Blocks,
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
			BlkSize:   inode.attr.BlkSize,
			Padding:   inode.attr.Padding,
		},
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (dummy *globalsStruct) DoReadLink(inHeader *fission.InHeader) (readLinkOut *fission.ReadLinkOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoSymLink(inHeader *fission.InHeader, symLinkIn *fission.SymLinkIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoMkNod(inHeader *fission.InHeader, mkNodIn *fission.MkNodIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoMkDir(inHeader *fission.InHeader, mkDirIn *fission.MkDirIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoUnlink(inHeader *fission.InHeader, unlinkIn *fission.UnlinkIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoRmDir(inHeader *fission.InHeader, rmDirIn *fission.RmDirIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoRename(inHeader *fission.InHeader, renameIn *fission.RenameIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoLink(inHeader *fission.InHeader, linkIn *fission.LinkIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoOpen(inHeader *fission.InHeader, openIn *fission.OpenIn) (openOut *fission.OpenOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoRead(inHeader *fission.InHeader, readIn *fission.ReadIn) (readOut *fission.ReadOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoWrite(inHeader *fission.InHeader, writeIn *fission.WriteIn) (writeOut *fission.WriteOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoStatFS(inHeader *fission.InHeader) (statFSOut *fission.StatFSOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoRelease(inHeader *fission.InHeader, releaseIn *fission.ReleaseIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoFSync(inHeader *fission.InHeader, fSyncIn *fission.FSyncIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}

func (dummy *globalsStruct) DoSetXAttr(inHeader *fission.InHeader, setXAttrIn *fission.SetXAttrIn) (errno syscall.Errno) {
	var (
		err            error
		granted        bool
		grantedLockSet *grantedLockSetStruct = makeGrantedLockSet()
		inode          *inodeStruct
		ok             bool
		setXAttrInName []byte
		setXAttrInData []byte
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

	setXAttrInName = cloneByteSlice(setXAttrIn.Name, true)
	setXAttrInData = cloneByteSlice(setXAttrIn.Data, false)

	ok, err = inode.xattrMap.PatchByKey(setXAttrInName, setXAttrInData)
	if nil != err {
		globals.logger.Printf("func DoSetXAttr(NodeID==%v, Name==%s) failed on .xattrMap.PatchByKey(): %v", inHeader.NodeID, string(setXAttrIn.Name[:]), err)
		os.Exit(1)
	}

	if !ok {
		ok, err = inode.xattrMap.Put(setXAttrInName, setXAttrInData)
		if nil != err {
			globals.logger.Printf("func DoSetXAttr(NodeID==%v, Name==%s) failed on .xattrMap.Put(): %v", inHeader.NodeID, string(setXAttrIn.Name[:]), err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func DoSetXAttr(NodeID==%v, Name==%s) .xattrMap.Put() returned !ok", inHeader.NodeID, string(setXAttrIn.Name[:]))
			os.Exit(1)
		}
	}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (dummy *globalsStruct) DoGetXAttr(inHeader *fission.InHeader, getXAttrIn *fission.GetXAttrIn) (getXAttrOut *fission.GetXAttrOut, errno syscall.Errno) {
	var (
		dataAsByteSlice []byte
		dataAsValue     sortedmap.Value
		err             error
		granted         bool
		grantedLockSet  *grantedLockSetStruct = makeGrantedLockSet()
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
	if nil != err {
		globals.logger.Printf("func DoGetXAttr(NodeID==%v) failed on .xattrMap.GetByKey(): %v", inHeader.NodeID, err)
		os.Exit(1)
	}

	grantedLockSet.freeAll(false)

	if !ok {
		errno = syscall.ENOENT
		return
	}

	dataAsByteSlice = dataAsValue.([]byte)

	if uint32(len(dataAsByteSlice)) > getXAttrIn.Size {
		errno = syscall.E2BIG
		return
	}

	getXAttrOut = &fission.GetXAttrOut{
		Size:    uint32(len(dataAsByteSlice)),
		Padding: 0,
		Data:    cloneByteSlice(dataAsByteSlice, false),
	}

	errno = 0
	return
}

func (dummy *globalsStruct) DoListXAttr(inHeader *fission.InHeader, listXAttrIn *fission.ListXAttrIn) (listXAttrOut *fission.ListXAttrOut, errno syscall.Errno) {
	var (
		err                  error
		granted              bool
		grantedLockSet       *grantedLockSetStruct = makeGrantedLockSet()
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
		Size:    0, // unnecessary
		Padding: 0,
		Name:    make([][]byte, 0),
	}

	xattrCount, err = inode.xattrMap.Len()
	if nil != err {
		globals.logger.Printf("func DoListXAttr(NodeID==%v) failed on .dirEntryMap.Len(): %v", inHeader.NodeID, err)
		os.Exit(1)
	}

	totalSize = 0

	for xattrIndex = 0; xattrIndex < xattrCount; xattrIndex++ {
		xattrNameAsKey, _, ok, err = inode.xattrMap.GetByIndex(xattrIndex)
		if nil != err {
			globals.logger.Printf("func DoGetXAttr(NodeID==%v) failed on .xattrMap.GetByIndex(%d): %v", inHeader.NodeID, xattrIndex, err)
			os.Exit(1)
		}
		if !ok {
			globals.logger.Printf("func DoGetXAttr(NodeID==%v) .xattrMap.GetByIndex(%d) returned !ok", inHeader.NodeID, xattrIndex)
			os.Exit(1)
		}

		xattrNameAsByteSlice = xattrNameAsKey.([]byte)

		if 0 == xattrIndex {
			if uint32(len(xattrNameAsByteSlice)+1) > listXAttrIn.Size {
				grantedLockSet.freeAll(false)
				errno = syscall.ENOSPC
				return
			}

			totalSize += uint32(len(xattrNameAsByteSlice))
		} else {
			if (totalSize + uint32(len(xattrNameAsByteSlice)+1)) > listXAttrIn.Size {
				grantedLockSet.freeAll(false)
				errno = syscall.ENOSPC
				return
			}

			totalSize += uint32(len(xattrNameAsByteSlice) + 1)
		}

		listXAttrOut.Name = append(listXAttrOut.Name, xattrNameAsByteSlice)
	}

	grantedLockSet.freeAll(false)

	listXAttrOut.Size = totalSize // unnecessary

	errno = 0
	return
}

func (dummy *globalsStruct) DoRemoveXAttr(inHeader *fission.InHeader, removeXAttrIn *fission.RemoveXAttrIn) (errno syscall.Errno) {
	var (
		err            error
		granted        bool
		grantedLockSet *grantedLockSetStruct = makeGrantedLockSet()
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
	if nil != err {
		globals.logger.Printf("func DoRemoveXAttr(NodeID==%v, Name==%s) failed on .xattrMap.DeleteByKey(): %v", inHeader.NodeID, string(removeXAttrIn.Name[:]), err)
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

func (dummy *globalsStruct) DoFlush(inHeader *fission.InHeader, flushIn *fission.FlushIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}

func (dummy *globalsStruct) DoInit(inHeader *fission.InHeader, initIn *fission.InitIn) (initOut *fission.InitOut, errno syscall.Errno) {
	initOut = &fission.InitOut{
		Major:                initIn.Major,
		Minor:                initIn.Minor,
		MaxReadAhead:         initIn.MaxReadAhead,
		Flags:                initIn.Flags & initOutFlagsMask,
		MaxBackground:        initOutMaxBackgound,
		CongestionThreshhold: initOutCongestionThreshhold,
		MaxWrite:             initOutMaxWrite,
	}

	errno = 0

	return
}

func (dummy *globalsStruct) DoOpenDir(inHeader *fission.InHeader, openDirIn *fission.OpenDirIn) (openDirOut *fission.OpenDirOut, errno syscall.Errno) {
	var (
		dirInode       *inodeStruct
		granted        bool
		grantedLockSet *grantedLockSetStruct = makeGrantedLockSet()
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

	globals.lastFH++

	openDirOut = &fission.OpenDirOut{
		FH:        globals.lastFH,
		OpenFlags: 0,
		Padding:   0,
	}

	globals.fhMap[openDirOut.FH] = dirInode.nodeID
	dirInode.fhSet[openDirOut.FH] = struct{}{}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (dummy *globalsStruct) DoReadDir(inHeader *fission.InHeader, readDirIn *fission.ReadDirIn) (readDirOut *fission.ReadDirOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}

func (dummy *globalsStruct) DoReleaseDir(inHeader *fission.InHeader, releaseDirIn *fission.ReleaseDirIn) (errno syscall.Errno) {
	errno = 0

	return
}

func (dummy *globalsStruct) DoFSyncDir(inHeader *fission.InHeader, fSyncDirIn *fission.FSyncDirIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoGetLK(inHeader *fission.InHeader, getLKIn *fission.GetLKIn) (getLKOut *fission.GetLKOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoSetLK(inHeader *fission.InHeader, setLKIn *fission.SetLKIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoSetLKW(inHeader *fission.InHeader, setLKWIn *fission.SetLKWIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoAccess(inHeader *fission.InHeader, accessIn *fission.AccessIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}

func (dummy *globalsStruct) DoCreate(inHeader *fission.InHeader, createIn *fission.CreateIn) (createOut *fission.CreateOut, errno syscall.Errno) {
	var (
		createInName    []byte
		dirEntInode     *inodeStruct
		dirEntInodeMode uint32
		dirInode        *inodeStruct
		err             error
		granted         bool
		grantedLockSet  *grantedLockSetStruct = makeGrantedLockSet()
		ok              bool
		unixTimeNowNSec uint32
		unixTimeNowSec  uint64
	)

	createInName = cloneByteSlice(createIn.Name, false)

	dirEntInodeMode = uint32(syscall.S_IRWXU | syscall.S_IRWXG | syscall.S_IRWXO)
	dirEntInodeMode &= createIn.Mode
	dirEntInodeMode &= ^createIn.UMask
	dirEntInodeMode |= syscall.S_IFREG

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

	_, ok, err = dirInode.dirEntryMap.GetByKey(createInName)
	if nil != err {
		globals.logger.Printf("func DoCreate(NodeID==%v,Name=%s) failed on .dirEntryMap.GetByKey(): %v", inHeader.NodeID, string(createInName[:]), err)
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
		nodeID:  globals.lastNodeID,
		attr: fission.Attr{
			Ino:       globals.lastNodeID,
			Size:      0,
			Blocks:    0,
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
			BlkSize:   attrBlkSize,
			Padding:   0,
		},
		xattrMap:    sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.xattrMapDummy),
		dirEntryMap: nil,
		fileData:    make([]byte, 0),
		symlinkData: nil,
		fhSet:       make(map[uint64]struct{}),
	}

	ok, err = dirInode.dirEntryMap.Put(createInName, dirEntInode.nodeID)
	if nil != err {
		globals.logger.Printf("func DoCreate(NodeID==%v,Name=%s) failed on .dirEntryMap.Put(): %v", inHeader.NodeID, string(createInName[:]), err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("func DoCreate(NodeID==%v,Name=%s) .dirEntryMap.Put() returned !ok", inHeader.NodeID, string(createInName[:]))
		os.Exit(1)
	}

	globals.inodeMap[dirEntInode.nodeID] = dirEntInode

	globals.lastFH++

	createOut = &fission.CreateOut{
		FH:        globals.lastFH,
		OpenFlags: 0,
		Padding:   0,
		EntryOut: fission.EntryOut{
			NodeID:         dirEntInode.attr.Ino,
			Generation:     0,
			EntryValidSec:  0,
			AttrValidSec:   0,
			EntryValidNSec: 0,
			AttrValidNSec:  0,
			Attr: fission.Attr{
				Ino:       dirEntInode.attr.Ino,
				Size:      dirEntInode.attr.Size,
				Blocks:    dirEntInode.attr.Blocks,
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
				BlkSize:   dirEntInode.attr.BlkSize,
				Padding:   dirEntInode.attr.Padding,
			},
		},
	}

	globals.fhMap[createOut.FH] = dirEntInode.nodeID
	dirEntInode.fhSet[createOut.FH] = struct{}{}

	grantedLockSet.freeAll(false)

	errno = 0
	return
}

func (dummy *globalsStruct) DoInterrupt(inHeader *fission.InHeader, interruptIn *fission.InterruptIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoBMap(inHeader *fission.InHeader, bMapIn *fission.BMapIn) (bMapOut *fission.BMapOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoDestroy(inHeader *fission.InHeader) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoPoll(inHeader *fission.InHeader, pollIn *fission.PollIn) (pollOut *fission.PollOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoBatchForget(inHeader *fission.InHeader, batchForgetIn *fission.BatchForgetIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoFAllocate(inHeader *fission.InHeader, fAllocateIn *fission.FAllocateIn) (errno syscall.Errno) {
	return syscall.ENOSYS
}

func (dummy *globalsStruct) DoReadDirPlus(inHeader *fission.InHeader, readDirPlusIn *fission.ReadDirPlusIn) (readDirPlusOut *fission.ReadDirPlusOut, errno syscall.Errno) {
	var (
		dirEntInoAsValue      sortedmap.Value
		dirEntInoAsU64        uint64
		dirEntNameAsByteSlice []byte
		dirEntNameAsKey       sortedmap.Key
		dirEntNameLenAligned  uint32
		dirEntSize            uint32
		dirEntInode           *inodeStruct
		dirEntPlusCount       int
		dirEntPlusIndex       int
		dirInode              *inodeStruct
		err                   error
		granted               bool
		grantedLockSet        *grantedLockSetStruct = makeGrantedLockSet()
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
	if nil != err {
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

	for dirEntPlusIndex = 0; dirEntPlusIndex < dirEntPlusCount; dirEntPlusIndex++ {
		dirEntNameAsKey, dirEntInoAsValue, ok, err = dirInode.dirEntryMap.GetByIndex(dirEntPlusIndex)
		if nil != err {
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

		readDirPlusOut.DirEntPlus[dirEntPlusIndex] = fission.DirEntPlus{
			EntryOut: fission.EntryOut{
				NodeID:         dirEntInode.attr.Ino,
				Generation:     0,
				EntryValidSec:  0,
				AttrValidSec:   0,
				EntryValidNSec: 0,
				AttrValidNSec:  0,
				Attr: fission.Attr{
					Ino:       dirEntInode.attr.Ino,
					Size:      dirEntInode.attr.Size,
					Blocks:    dirEntInode.attr.Blocks,
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
					BlkSize:   dirEntInode.attr.BlkSize,
					Padding:   dirEntInode.attr.Padding,
				},
			},
			DirEnt: fission.DirEnt{
				Ino:     dirEntInode.nodeID,
				Off:     uint64(dirEntPlusIndex) + 1,
				NameLen: uint32(len(dirEntNameAsByteSlice)), // unnecessary
				Type:    dirEntInode.attr.Mode & syscall.S_IFMT,
				Name:    cloneByteSlice(dirEntNameAsByteSlice, false),
			},
		}
	}

	grantedLockSet.freeAll(false)

	// Now prune on the left to readDirPlusIn.Offset & on the right anything beyond readDirPlusIn.Size

	readDirPlusOut.DirEntPlus = readDirPlusOut.DirEntPlus[readDirPlusIn.Offset:]

	dirEntPlusIndex = 0
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

func (dummy *globalsStruct) DoRename2(inHeader *fission.InHeader, rename2In *fission.Rename2In) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoLSeek(inHeader *fission.InHeader, lSeekIn *fission.LSeekIn) (lSeekOut *fission.LSeekOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
