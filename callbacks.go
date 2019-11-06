package fission

import (
	"bytes"
	"syscall"
	"unsafe"
)

func (volume *volumeStruct) doLookup(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		lookupIn   *LookupIn
		lookupOut  *LookupOut
		outPayload []byte
	)

	lookupIn = &LookupIn{
		Name: devFuseFDReadBufPayload,
	}

	lookupOut, errno = volume.callbacks.DoLookup(inHeader, lookupIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, LookupOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = lookupOut.NodeID
	*(*uint64)(unsafe.Pointer(&outPayload[8])) = lookupOut.Generation
	*(*uint64)(unsafe.Pointer(&outPayload[16])) = lookupOut.EntryValidSec
	*(*uint64)(unsafe.Pointer(&outPayload[24])) = lookupOut.AttrValidSec
	*(*uint32)(unsafe.Pointer(&outPayload[32])) = lookupOut.EntryValidNSec
	*(*uint32)(unsafe.Pointer(&outPayload[36])) = lookupOut.AttrValidNSec

	*(*uint64)(unsafe.Pointer(&outPayload[40])) = lookupOut.Ino
	*(*uint64)(unsafe.Pointer(&outPayload[48])) = lookupOut.Size
	*(*uint64)(unsafe.Pointer(&outPayload[56])) = lookupOut.Blocks
	*(*uint64)(unsafe.Pointer(&outPayload[64])) = lookupOut.ATimeSec
	*(*uint64)(unsafe.Pointer(&outPayload[72])) = lookupOut.MTimeSec
	*(*uint64)(unsafe.Pointer(&outPayload[80])) = lookupOut.CTimeSec
	*(*uint32)(unsafe.Pointer(&outPayload[88])) = lookupOut.ATimeNSec
	*(*uint32)(unsafe.Pointer(&outPayload[92])) = lookupOut.MTimeNSec
	*(*uint32)(unsafe.Pointer(&outPayload[96])) = lookupOut.CTimeNSec
	*(*uint32)(unsafe.Pointer(&outPayload[100])) = lookupOut.Mode
	*(*uint32)(unsafe.Pointer(&outPayload[104])) = lookupOut.NLink
	*(*uint32)(unsafe.Pointer(&outPayload[108])) = lookupOut.UID
	*(*uint32)(unsafe.Pointer(&outPayload[112])) = lookupOut.GID
	*(*uint32)(unsafe.Pointer(&outPayload[116])) = lookupOut.RDev
	*(*uint32)(unsafe.Pointer(&outPayload[120])) = lookupOut.BlkSize
	*(*uint32)(unsafe.Pointer(&outPayload[124])) = lookupOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doForget(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		forgetIn *ForgetIn
	)

	if len(devFuseFDReadBufPayload) != ForgetInSize {
		volume.logger.Printf("Call to doForget() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		return
	}

	forgetIn = &ForgetIn{
		NLookup: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
	}

	volume.callbacks.DoForget(inHeader, forgetIn)
}

func (volume *volumeStruct) doGetAttr(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		getAttrIn  *GetAttrIn
		getAttrOut *GetAttrOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != GetAttrInSize {
		volume.logger.Printf("Call to doGetAttr() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	getAttrIn = &GetAttrIn{
		Flags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Dummy: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
		FH:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
	}

	getAttrOut, errno = volume.callbacks.DoGetAttr(inHeader, getAttrIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, GetAttrOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = getAttrOut.AttrValidSec
	*(*uint32)(unsafe.Pointer(&outPayload[8])) = getAttrOut.AttrValidNSec
	*(*uint32)(unsafe.Pointer(&outPayload[12])) = getAttrOut.Dummy

	*(*uint64)(unsafe.Pointer(&outPayload[16])) = getAttrOut.Ino
	*(*uint64)(unsafe.Pointer(&outPayload[24])) = getAttrOut.Size
	*(*uint64)(unsafe.Pointer(&outPayload[32])) = getAttrOut.Blocks
	*(*uint64)(unsafe.Pointer(&outPayload[40])) = getAttrOut.ATimeSec
	*(*uint64)(unsafe.Pointer(&outPayload[48])) = getAttrOut.MTimeSec
	*(*uint64)(unsafe.Pointer(&outPayload[56])) = getAttrOut.CTimeSec
	*(*uint32)(unsafe.Pointer(&outPayload[64])) = getAttrOut.ATimeNSec
	*(*uint32)(unsafe.Pointer(&outPayload[68])) = getAttrOut.MTimeNSec
	*(*uint32)(unsafe.Pointer(&outPayload[72])) = getAttrOut.CTimeNSec
	*(*uint32)(unsafe.Pointer(&outPayload[76])) = getAttrOut.Mode
	*(*uint32)(unsafe.Pointer(&outPayload[80])) = getAttrOut.NLink
	*(*uint32)(unsafe.Pointer(&outPayload[84])) = getAttrOut.UID
	*(*uint32)(unsafe.Pointer(&outPayload[88])) = getAttrOut.GID
	*(*uint32)(unsafe.Pointer(&outPayload[92])) = getAttrOut.RDev
	*(*uint32)(unsafe.Pointer(&outPayload[96])) = getAttrOut.BlkSize
	*(*uint32)(unsafe.Pointer(&outPayload[100])) = getAttrOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doSetAttr(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno     syscall.Errno
		setAttrIn *SetAttrIn
	)

	if len(devFuseFDReadBufPayload) != SetAttrInSize {
		volume.logger.Printf("Call to doSetAttr() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	setAttrIn = &SetAttrIn{
		Valid:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Padding:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
		FH:        *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Size:      *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		LockOwner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
		ATimeSec:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
		MTimeSec:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[40])),
		Unused2:   *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[48])),
		ATimeNSec: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[56])),
		MTimeNSec: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[60])),
		Unused3:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[64])),
		Mode:      *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[68])),
		Unused4:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[72])),
		UID:       *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[76])),
		GID:       *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[80])),
		Unused5:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[84])),
	}

	errno = volume.callbacks.DoSetAttr(inHeader, setAttrIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doReadLink(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno       syscall.Errno
		outPayload  []byte
		readLinkOut *ReadLinkOut
	)

	if len(devFuseFDReadBufPayload) != 0 {
		volume.logger.Printf("Call to doReadLink() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	readLinkOut, errno = volume.callbacks.DoReadLink(inHeader)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = readLinkOut.Data

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doSymLink(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno         syscall.Errno
		nameDataSplit [][]byte
		symLinkIn     *SymLinkIn
	)

	nameDataSplit = bytes.SplitN(devFuseFDReadBufPayload, []byte{0}, 1)
	if len(nameDataSplit) != 2 {
		volume.logger.Printf("Call to doSymLink() with bad devFuseFDReadBufPayload")
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	symLinkIn = &SymLinkIn{
		Name: nameDataSplit[0],
		Data: nameDataSplit[1],
	}

	errno = volume.callbacks.DoSymLink(inHeader, symLinkIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doMkNod(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno   syscall.Errno
		mkNodIn *MkNodIn
	)

	mkNodIn = &MkNodIn{
		Name: devFuseFDReadBufPayload,
	}

	errno = volume.callbacks.DoMkNod(inHeader, mkNodIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doMkDir(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno   syscall.Errno
		mkDirIn *MkDirIn
	)

	mkDirIn = &MkDirIn{
		Name: devFuseFDReadBufPayload,
	}

	errno = volume.callbacks.DoMkDir(inHeader, mkDirIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doUnlink(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno    syscall.Errno
		unlinkIn *UnlinkIn
	)

	unlinkIn = &UnlinkIn{
		Name: devFuseFDReadBufPayload,
	}

	errno = volume.callbacks.DoUnlink(inHeader, unlinkIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doRmDir(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno   syscall.Errno
		rmDirIn *RmDirIn
	)

	rmDirIn = &RmDirIn{
		Name: devFuseFDReadBufPayload,
	}

	errno = volume.callbacks.DoRmDir(inHeader, rmDirIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doRename(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno               syscall.Errno
		oldNameNewNameSplit [][]byte
		renameIn            *RenameIn
	)

	if len(devFuseFDReadBufPayload) < RenameInFixedPortionSize {
		volume.logger.Printf("Call to doRename() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	oldNameNewNameSplit = bytes.SplitN(devFuseFDReadBufPayload[RenameInFixedPortionSize:], []byte{0}, 1)
	if len(oldNameNewNameSplit) != 2 {
		volume.logger.Printf("Call to doRename() with bad devFuseFDReadBufPayload")
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	renameIn = &RenameIn{
		NewDir:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		OldName: oldNameNewNameSplit[0],
		NewName: oldNameNewNameSplit[1],
	}

	errno = volume.callbacks.DoRename(inHeader, renameIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doLink(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno  syscall.Errno
		linkIn *LinkIn
	)

	if len(devFuseFDReadBufPayload) < LinkInFixedPortionSize {
		volume.logger.Printf("Call to doLink() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	linkIn = &LinkIn{
		OldNodeID: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Name:      devFuseFDReadBufPayload[LinkInFixedPortionSize:],
	}

	errno = volume.callbacks.DoLink(inHeader, linkIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doOpen(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		openIn     *OpenIn
		openOut    *OpenOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != OpenInSize {
		volume.logger.Printf("Call to doOpen() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	openIn = &OpenIn{
		Flags:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Unused: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
	}

	openOut, errno = volume.callbacks.DoOpen(inHeader, openIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, OpenOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = openOut.FH
	*(*uint32)(unsafe.Pointer(&outPayload[8])) = openOut.OpenFlags
	*(*uint32)(unsafe.Pointer(&outPayload[12])) = openOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doRead(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		outPayload []byte
		readIn     *ReadIn
		readOut    *ReadOut
	)

	if len(devFuseFDReadBufPayload) != ReadInSize {
		volume.logger.Printf("Call to doRead() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	readIn = &ReadIn{
		FH:        *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Offset:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Size:      *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		ReadFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[20])),
		LockOwner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
		Flags:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
		Padding:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
	}

	readOut, errno = volume.callbacks.DoRead(inHeader, readIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = readOut.Data

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doWrite(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		outPayload []byte
		writeIn    *WriteIn
		writeOut   *WriteOut
	)

	if len(devFuseFDReadBufPayload) < WriteInFixedPortionSize {
		volume.logger.Printf("Call to doWrite() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	writeIn = &WriteIn{
		FH:         *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Offset:     *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Size:       *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		WriteFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[20])),
		LockOwner:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
		Flags:      *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
		Padding:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
		Data:       devFuseFDReadBufPayload[WriteInFixedPortionSize:],
	}

	if len(writeIn.Data) != int(writeIn.Size) {
		volume.logger.Printf("Call to doWrite() with bad Size == %v expected %v", writeIn.Size, len(writeIn.Data))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	writeOut, errno = volume.callbacks.DoWrite(inHeader, writeIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, WriteOutSize)

	*(*uint32)(unsafe.Pointer(&outPayload[0])) = writeOut.Size
	*(*uint32)(unsafe.Pointer(&outPayload[4])) = writeOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doStatFS(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		outPayload []byte
		statFSOut  *StatFSOut
	)

	if len(devFuseFDReadBufPayload) != 0 {
		volume.logger.Printf("Call to doStatFS() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
	}

	statFSOut, errno = volume.callbacks.DoStatFS(inHeader)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, StatFSOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = statFSOut.Blocks
	*(*uint64)(unsafe.Pointer(&outPayload[8])) = statFSOut.BFree
	*(*uint64)(unsafe.Pointer(&outPayload[16])) = statFSOut.BAvail
	*(*uint64)(unsafe.Pointer(&outPayload[24])) = statFSOut.Files
	*(*uint64)(unsafe.Pointer(&outPayload[32])) = statFSOut.FFree
	*(*uint32)(unsafe.Pointer(&outPayload[40])) = statFSOut.BSize
	*(*uint32)(unsafe.Pointer(&outPayload[44])) = statFSOut.NameLen
	*(*uint32)(unsafe.Pointer(&outPayload[48])) = statFSOut.FRSize
	*(*uint32)(unsafe.Pointer(&outPayload[52])) = statFSOut.Padding
	*(*uint32)(unsafe.Pointer(&outPayload[56])) = statFSOut.Spare[0]
	*(*uint32)(unsafe.Pointer(&outPayload[60])) = statFSOut.Spare[1]
	*(*uint32)(unsafe.Pointer(&outPayload[64])) = statFSOut.Spare[2]
	*(*uint32)(unsafe.Pointer(&outPayload[68])) = statFSOut.Spare[3]
	*(*uint32)(unsafe.Pointer(&outPayload[72])) = statFSOut.Spare[4]
	*(*uint32)(unsafe.Pointer(&outPayload[76])) = statFSOut.Spare[5]

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doRelease(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno     syscall.Errno
		releaseIn *ReleaseIn
	)

	if len(devFuseFDReadBufPayload) != ReleaseInSize {
		volume.logger.Printf("Call to doRelease() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
	}

	releaseIn = &ReleaseIn{
		FH:           *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Flags:        *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		ReleaseFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
		LockOwner:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
	}

	errno = volume.callbacks.DoRelease(inHeader, releaseIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doFSync(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno   syscall.Errno
		fSyncIn *FSyncIn
	)

	if len(devFuseFDReadBufPayload) != FSyncInSize {
		volume.logger.Printf("Call to doFSync() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
	}

	fSyncIn = &FSyncIn{
		FH:         *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		FsyncFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Padding:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
	}

	errno = volume.callbacks.DoFSync(inHeader, fSyncIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doSetXAttr(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno          syscall.Errno
		nameDataSplit  [][]byte
		setXAttrIn     *SetXAttrIn
		setXAttrInSize int
	)

	if len(devFuseFDReadBufPayload) < SetXAttrInFixedPortionSize {
		volume.logger.Printf("Call to doSetXAttr() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	nameDataSplit = bytes.SplitN(devFuseFDReadBufPayload[SetXAttrInFixedPortionSize:], []byte{0}, 1)
	if len(nameDataSplit) != 2 {
		volume.logger.Printf("Call to doSetXAttr() with bad devFuseFDReadBufPayload")
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	setXAttrIn = &SetXAttrIn{
		Size:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Flags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
		Name:  nameDataSplit[0],
		Data:  nameDataSplit[1],
	}

	setXAttrInSize = SetXAttrInFixedPortionSize + len(setXAttrIn.Name) + 1 + len(setXAttrIn.Data)

	if len(devFuseFDReadBufPayload) != setXAttrInSize {
		volume.logger.Printf("Call to doSetXAttr() with bad Size == %v expected %v", setXAttrIn.Size, setXAttrInSize)
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	errno = volume.callbacks.DoSetXAttr(inHeader, setXAttrIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doGetXAttr(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno       syscall.Errno
		getXAttrIn  *GetXAttrIn
		getXAttrOut *GetXAttrOut
		outPayload  []byte
	)

	if len(devFuseFDReadBufPayload) < GetXAttrInFixedPortionSize {
		volume.logger.Printf("Call to doGetXAttr() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	getXAttrIn = &GetXAttrIn{
		Size:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
		Name:    devFuseFDReadBufPayload[GetXAttrInFixedPortionSize:],
	}

	getXAttrOut, errno = volume.callbacks.DoGetXAttr(inHeader, getXAttrIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, GetXAttrOutFixedPortionSize+len(getXAttrOut.Data))

	*(*uint32)(unsafe.Pointer(&outPayload[0])) = uint32(len(getXAttrOut.Data))
	*(*uint32)(unsafe.Pointer(&outPayload[4])) = getXAttrOut.Padding

	copy(outPayload[GetXAttrOutFixedPortionSize:], getXAttrOut.Data)

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doListXAttr(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno            syscall.Errno
		listXAttrIn      *ListXAttrIn
		listXAttrOut     *ListXAttrOut
		nameElement      []byte
		nameElementIndex int
		nameTotalLen     uint32
		outPayload       []byte
		outPayloadOffset uint32
	)

	if len(devFuseFDReadBufPayload) != ListXAttrInSize {
		volume.logger.Printf("Call to doListXAttr() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	listXAttrIn = &ListXAttrIn{
		Size:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
	}

	listXAttrOut, errno = volume.callbacks.DoListXAttr(inHeader, listXAttrIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	if 0 == len(listXAttrOut.Name) {
		nameTotalLen = 0
	} else {
		nameTotalLen = uint32(len(listXAttrOut.Name)) - 1

		for _, nameElement = range listXAttrOut.Name {
			nameTotalLen += uint32(len(nameElement))
		}
	}

	outPayload = make([]byte, ListXAttrOutFixedPortionSize+nameTotalLen)

	*(*uint32)(unsafe.Pointer(&outPayload[0])) = nameTotalLen
	*(*uint32)(unsafe.Pointer(&outPayload[4])) = listXAttrOut.Padding

	if 0 != len(listXAttrOut.Name) {
		outPayloadOffset = ListXAttrOutFixedPortionSize

		for nameElementIndex, nameElement = range listXAttrOut.Name {
			if 0 != nameElementIndex {
				outPayload[outPayloadOffset] = 0
				outPayloadOffset++
			}

			copy(outPayload[outPayloadOffset:], nameElement)
			outPayloadOffset += uint32(len(nameElement))
		}
	}

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doRemoveXAttr(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno         syscall.Errno
		removeXAttrIn *RemoveXAttrIn
	)

	removeXAttrIn = &RemoveXAttrIn{
		Name: devFuseFDReadBufPayload[GetXAttrInFixedPortionSize:],
	}

	errno = volume.callbacks.DoRemoveXAttr(inHeader, removeXAttrIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doFlush(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno   syscall.Errno
		flushIn *FlushIn
	)

	if len(devFuseFDReadBufPayload) != FlushInSize {
		volume.logger.Printf("Call to doFlush() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	flushIn = &FlushIn{
		FH:        *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Unused:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Padding:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
		LockOwner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
	}

	errno = volume.callbacks.DoFlush(inHeader, flushIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doInit(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		initIn     *InitIn
		initOut    *InitOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != InitInSize {
		volume.logger.Printf("Call to doInit() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	initIn = &InitIn{
		Major:        *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Minor:        *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
		MaxReadAhead: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Flags:        *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
	}

	initOut, errno = volume.callbacks.DoInit(inHeader, initIn)
	if 0 != errno {
		volume.logger.Printf("Call to doInit() returning bad errno == %v", errno)
		volume.devFuseFDWriter(inHeader, errno)
		return
	}
	if initOut.MaxWrite != volume.initOutMaxWrite {
		volume.logger.Printf("Call to doInit() attempted to modify MaxWrite... ignoring it")
	}

	outPayload = make([]byte, InitOutSize)

	*(*uint32)(unsafe.Pointer(&outPayload[0])) = initOut.Major
	*(*uint32)(unsafe.Pointer(&outPayload[4])) = initOut.Minor
	*(*uint32)(unsafe.Pointer(&outPayload[8])) = initOut.MaxReadAhead
	*(*uint32)(unsafe.Pointer(&outPayload[12])) = initOut.Flags
	*(*uint16)(unsafe.Pointer(&outPayload[16])) = initOut.MaxBackground
	*(*uint16)(unsafe.Pointer(&outPayload[18])) = initOut.CongestionThreshhold
	*(*uint32)(unsafe.Pointer(&outPayload[20])) = volume.initOutMaxWrite

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doOpenDir(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		openDirIn  *OpenDirIn
		openDirOut *OpenDirOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != OpenDirInSize {
		volume.logger.Printf("Call to doOpenDir() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	openDirIn = &OpenDirIn{
		Flags:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Unused: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
	}

	openDirOut, errno = volume.callbacks.DoOpenDir(inHeader, openDirIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, OpenDirOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = openDirOut.FH
	*(*uint32)(unsafe.Pointer(&outPayload[8])) = openDirOut.OpenFlags
	*(*uint32)(unsafe.Pointer(&outPayload[12])) = openDirOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doReadDir(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		dirEnt            *DirEnt
		dirEntIndex       int
		dirEntPayloadSize uint32
		errno             syscall.Errno
		nameLenAligned    uint32
		outPayload        []byte
		outPayloadOffset  uint32
		paddingIndex      uint32
		readDirIn         *ReadDirIn
		readDirOut        *ReadDirOut
	)

	if len(devFuseFDReadBufPayload) != ReadDirInSize {
		volume.logger.Printf("Call to doReadDir() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	readDirIn = &ReadDirIn{
		FH:        *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Offset:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Size:      *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		ReadFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[20])),
		LockOwner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
		Flags:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
		Padding:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
	}

	readDirOut, errno = volume.callbacks.DoReadDir(inHeader, readDirIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, readDirIn.Size)

	outPayloadOffset = 0

	for dirEntIndex = 0; dirEntIndex < len(readDirOut.DirEnt); dirEntIndex++ {
		dirEnt = &readDirOut.DirEnt[dirEntIndex]

		nameLenAligned = (uint32(len(dirEnt.Name)) + (DirEntAlignment - 1)) & ^uint32(DirEntAlignment-1)
		dirEntPayloadSize = DirEntFixedPortionSize + nameLenAligned

		if (outPayloadOffset + dirEntPayloadSize) > readDirIn.Size {
			volume.logger.Printf("Return from DoReadDir() had excessive DirEnt's")

			outPayload = outPayload[:outPayloadOffset]

			volume.devFuseFDWriter(inHeader, 0, outPayload)

			return
		}

		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+0])) = dirEnt.Ino
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+8])) = dirEnt.Off
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+16])) = uint32(len(dirEnt.Name))
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+20])) = dirEnt.Type

		outPayloadOffset += DirEntFixedPortionSize

		copy(outPayload[outPayloadOffset:], dirEnt.Name)

		outPayloadOffset += uint32(len(dirEnt.Name))

		for paddingIndex = uint32(len(dirEnt.Name)); paddingIndex < nameLenAligned; paddingIndex++ {
			outPayload[outPayloadOffset] = 0
			outPayloadOffset++
		}
	}

	outPayload = outPayload[:outPayloadOffset]

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doReleaseDir(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno        syscall.Errno
		releaseDirIn *ReleaseDirIn
	)

	if len(devFuseFDReadBufPayload) != ReleaseDirInSize {
		volume.logger.Printf("Call to doReleaseDir() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	releaseDirIn = &ReleaseDirIn{
		FH:           *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Flags:        *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		ReleaseFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
		LockOwner:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
	}

	errno = volume.callbacks.DoReleaseDir(inHeader, releaseDirIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doFSyncDir(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		fSyncDirIn *FSyncDirIn
	)

	if len(devFuseFDReadBufPayload) != FSyncDirInSize {
		volume.logger.Printf("Call to doFSyncDir() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	fSyncDirIn = &FSyncDirIn{
		FH:         *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		FsyncFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Padding:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
	}

	errno = volume.callbacks.DoFSyncDir(inHeader, fSyncDirIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doGetLK(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		getLKIn    *GetLKIn
		getLKOut   *GetLKOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != GetLKInSize {
		volume.logger.Printf("Call to doGetLK() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	getLKIn = &GetLKIn{
		FH:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Owner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		FileLock: FileLock{
			Start: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
			End:   *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
			Type:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
			PID:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
		},
		LKFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[40])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[44])),
	}

	getLKOut, errno = volume.callbacks.DoGetLK(inHeader, getLKIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, GetLKOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = getLKOut.Start
	*(*uint64)(unsafe.Pointer(&outPayload[8])) = getLKOut.End
	*(*uint32)(unsafe.Pointer(&outPayload[16])) = getLKOut.Type
	*(*uint32)(unsafe.Pointer(&outPayload[20])) = getLKOut.PID

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doSetLK(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno   syscall.Errno
		setLKIn *SetLKIn
	)

	if len(devFuseFDReadBufPayload) != SetLKInSize {
		volume.logger.Printf("Call to doSetLK() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	setLKIn = &SetLKIn{
		FH:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Owner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		FileLock: FileLock{
			Start: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
			End:   *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
			Type:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
			PID:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
		},
		LKFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[40])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[44])),
	}

	errno = volume.callbacks.DoSetLK(inHeader, setLKIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doSetLKW(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno    syscall.Errno
		setLKWIn *SetLKWIn
	)

	if len(devFuseFDReadBufPayload) != SetLKWInSize {
		volume.logger.Printf("Call to doSetLKW() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	setLKWIn = &SetLKWIn{
		FH:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Owner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		FileLock: FileLock{
			Start: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
			End:   *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
			Type:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
			PID:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
		},
		LKFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[40])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[44])),
	}

	errno = volume.callbacks.DoSetLKW(inHeader, setLKWIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doAccess(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno    syscall.Errno
		accessIn *AccessIn
	)

	if len(devFuseFDReadBufPayload) != AccessInSize {
		volume.logger.Printf("Call to doAccess() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	accessIn = &AccessIn{
		Mask:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
	}

	errno = volume.callbacks.DoAccess(inHeader, accessIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doCreate(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		createIn   *CreateIn
		createOut  *CreateOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) < CreateInFixedPortionSize {
		volume.logger.Printf("Call to doCreate() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	createIn = &CreateIn{
		Flags:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Mode:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
		UMask:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
		Name:    devFuseFDReadBufPayload[CreateInFixedPortionSize:],
	}

	createOut, errno = volume.callbacks.DoCreate(inHeader, createIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, CreateOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = createOut.FH
	*(*uint32)(unsafe.Pointer(&outPayload[8])) = createOut.OpenFlags
	*(*uint32)(unsafe.Pointer(&outPayload[12])) = createOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doInterrupt(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno       syscall.Errno
		interruptIn *InterruptIn
	)

	if len(devFuseFDReadBufPayload) != InterruptInSize {
		volume.logger.Printf("Call to doInterrupt() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	interruptIn = &InterruptIn{
		Unique: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
	}

	errno = volume.callbacks.DoInterrupt(inHeader, interruptIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doBMap(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		bMapIn     *BMapIn
		bMapOut    *BMapOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != BMapInSize {
		volume.logger.Printf("Call to doBMap() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	bMapIn = &BMapIn{
		Block:     *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		BlockSize: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Padding:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
	}

	bMapOut, errno = volume.callbacks.DoBMap(inHeader, bMapIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, BMapOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = bMapOut.Block

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doDestroy(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno syscall.Errno
	)

	if len(devFuseFDReadBufPayload) != 0 {
		volume.logger.Printf("Call to doDestroy() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	errno = volume.callbacks.DoDestroy(inHeader)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doPoll(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		outPayload []byte
		pollIn     *PollIn
		pollOut    *PollOut
	)

	if len(devFuseFDReadBufPayload) != PollInSize {
		volume.logger.Printf("Call to doPoll() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	pollIn = &PollIn{
		FH:     *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		KH:     *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Flags:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		Events: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[20])),
	}

	pollOut, errno = volume.callbacks.DoPoll(inHeader, pollIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, PollOutSize)

	*(*uint32)(unsafe.Pointer(&outPayload[0])) = pollOut.REvents
	*(*uint32)(unsafe.Pointer(&outPayload[4])) = pollOut.Padding

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doBatchForget(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		batchForgetIn            *BatchForgetIn
		batchForgetInForgetIndex uint32
		batchForgetInSize        int
		batchForgetInOffset      uint32
		errno                    syscall.Errno
	)

	if len(devFuseFDReadBufPayload) < BatchForgetInFixedPortionSize {
		volume.logger.Printf("Call to doBatchForget() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	batchForgetIn = &BatchForgetIn{
		Count: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Dummy: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[4])),
	}

	batchForgetInSize = BatchForgetInFixedPortionSize + int(batchForgetIn.Count*ForgetOneSize)

	if len(devFuseFDReadBufPayload) != batchForgetInSize {
		volume.logger.Printf("Call to doBatchForget() with bad len(devFuseFDReadBufPayload) == %v expected %v", len(devFuseFDReadBufPayload), batchForgetInSize)
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	batchForgetIn.Forget = make([]ForgetOne, batchForgetIn.Count)

	batchForgetInOffset = BatchForgetInFixedPortionSize

	for batchForgetInForgetIndex = 0; batchForgetInForgetIndex < batchForgetIn.Count; batchForgetInForgetIndex++ {
		batchForgetIn.Forget[batchForgetInForgetIndex] = ForgetOne{
			NodeID:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[batchForgetInOffset+0])),
			NLookup: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[batchForgetInOffset+8])),
		}
	}

	errno = volume.callbacks.DoBatchForget(inHeader, batchForgetIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doFAllocate(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno       syscall.Errno
		fAllocateIn *FAllocateIn
	)

	if len(devFuseFDReadBufPayload) != FAllocateInSize {
		volume.logger.Printf("Call to doFAllocate() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	fAllocateIn = &FAllocateIn{
		FH:      *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Offset:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Length:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		Mode:    *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[28])),
	}

	errno = volume.callbacks.DoFAllocate(inHeader, fAllocateIn)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doReadDirPlus(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		dirEntPlus            *DirEntPlus
		dirEntPlusIndex       int
		dirEntPlusPayloadSize uint32
		errno                 syscall.Errno
		nameLenAligned        uint32
		outPayload            []byte
		outPayloadOffset      uint32
		paddingIndex          uint32
		readDirPlusIn         *ReadDirPlusIn
		readDirPlusOut        *ReadDirPlusOut
	)

	if len(devFuseFDReadBufPayload) != ReadDirPlusInSize {
		volume.logger.Printf("Call to doReadDirPlus() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	readDirPlusIn = &ReadDirPlusIn{
		FH:        *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Offset:    *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Size:      *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		ReadFlags: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[20])),
		LockOwner: *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[24])),
		Flags:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[32])),
		Padding:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[36])),
	}

	readDirPlusOut, errno = volume.callbacks.DoReadDirPlus(inHeader, readDirPlusIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, readDirPlusIn.Size)

	outPayloadOffset = 0

	for dirEntPlusIndex = 0; dirEntPlusIndex < len(readDirPlusOut.DirEntPlus); dirEntPlusIndex++ {
		dirEntPlus = &readDirPlusOut.DirEntPlus[dirEntPlusIndex]

		nameLenAligned = (uint32(len(dirEntPlus.Name)) + (DirEntAlignment - 1)) & ^uint32(DirEntAlignment-1)
		dirEntPlusPayloadSize = DirEntPlusFixedPortionSize + nameLenAligned

		if (outPayloadOffset + dirEntPlusPayloadSize) > readDirPlusIn.Size {
			volume.logger.Printf("Return from DoReadDirPlus() had excessive DirEntPlus's")

			outPayload = outPayload[:outPayloadOffset]

			volume.devFuseFDWriter(inHeader, 0, outPayload)

			return
		}

		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+0])) = dirEntPlus.NodeID
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+8])) = dirEntPlus.Generation
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+16])) = dirEntPlus.EntryValidSec
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+24])) = dirEntPlus.AttrValidSec
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+32])) = dirEntPlus.EntryValidNSec
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+36])) = dirEntPlus.AttrValidNSec

		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+40])) = dirEntPlus.Ino
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+48])) = dirEntPlus.Size
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+56])) = dirEntPlus.Blocks
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+64])) = dirEntPlus.ATimeSec
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+72])) = dirEntPlus.MTimeSec
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+80])) = dirEntPlus.CTimeSec
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+88])) = dirEntPlus.ATimeNSec
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+92])) = dirEntPlus.MTimeNSec
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+96])) = dirEntPlus.CTimeNSec
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+100])) = dirEntPlus.Mode
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+104])) = dirEntPlus.NLink
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+108])) = dirEntPlus.UID
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+112])) = dirEntPlus.GID
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+116])) = dirEntPlus.RDev
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+120])) = dirEntPlus.BlkSize
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+124])) = dirEntPlus.Padding

		outPayloadOffset += EntryOutSize

		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+0])) = dirEntPlus.Ino
		*(*uint64)(unsafe.Pointer(&outPayload[outPayloadOffset+8])) = dirEntPlus.Off
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+16])) = uint32(len(dirEntPlus.Name))
		*(*uint32)(unsafe.Pointer(&outPayload[outPayloadOffset+20])) = dirEntPlus.Type

		outPayloadOffset += DirEntFixedPortionSize

		copy(outPayload[outPayloadOffset:], dirEntPlus.Name)

		outPayloadOffset += uint32(len(dirEntPlus.Name))

		for paddingIndex = uint32(len(dirEntPlus.Name)); paddingIndex < nameLenAligned; paddingIndex++ {
			outPayload[outPayloadOffset] = 0
			outPayloadOffset++
		}
	}

	outPayload = outPayload[:outPayloadOffset]

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}

func (volume *volumeStruct) doRename2(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno               syscall.Errno
		oldNameNewNameSplit [][]byte
		rename2In           *Rename2In
	)

	if len(devFuseFDReadBufPayload) < Rename2InFixedPortionSize {
		volume.logger.Printf("Call to doRename2() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	oldNameNewNameSplit = bytes.SplitN(devFuseFDReadBufPayload[Rename2InFixedPortionSize:], []byte{0}, 1)
	if len(oldNameNewNameSplit) != 2 {
		volume.logger.Printf("Call to doRename2() with bad devFuseFDReadBufPayload")
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	rename2In = &Rename2In{
		NewDir:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Flags:   *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[12])),
		OldName: oldNameNewNameSplit[0],
		NewName: oldNameNewNameSplit[1],
	}

	errno = volume.callbacks.DoRename2(inHeader, rename2In)

	volume.devFuseFDWriter(inHeader, errno)
}

func (volume *volumeStruct) doLSeek(inHeader *InHeader, devFuseFDReadBufPayload []byte) {
	var (
		errno      syscall.Errno
		lSeekIn    *LSeekIn
		lSeekOut   *LSeekOut
		outPayload []byte
	)

	if len(devFuseFDReadBufPayload) != LSeekInSize {
		volume.logger.Printf("Call to doLSeek() with bad len(devFuseFDReadBufPayload) == %v", len(devFuseFDReadBufPayload))
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
		return
	}

	lSeekIn = &LSeekIn{
		FH:      *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[0])),
		Offset:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBufPayload[8])),
		Whence:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[16])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBufPayload[20])),
	}

	lSeekOut, errno = volume.callbacks.DoLSeek(inHeader, lSeekIn)
	if 0 != errno {
		volume.devFuseFDWriter(inHeader, errno)
		return
	}

	outPayload = make([]byte, LSeekOutSize)

	*(*uint64)(unsafe.Pointer(&outPayload[0])) = lSeekOut.Offset

	volume.devFuseFDWriter(inHeader, 0, outPayload)
}
