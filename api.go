// Copyright (c) 2015-2023, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package fission

import (
	"log"
	"syscall"
)

// Volume represents a file system instance. A Volume is provisioned by calling
// NewVolume(). After recording the returned interface from NewVolume(), a single
// call to DoMount() kicks off the mounting process and the caller should expect
// to see the various callbacks listed in the Callbacks interface. This will
// continue a single call to DoUnmount() is made after which the Volume instance
// may be safely discarded.
type Volume interface {
	// DoMount is invoked on a Volume interface to perform the FUSE mount and
	// begin receiving the various callbacks listed in the Callbacks interface.
	//
	DoMount() (err error)

	// DoUnmount is invoked on a Volume interface to perform the FUSE unmount.
	// Upon return, no callbacks will be made and the Volume instance may be
	// safely discarded.
	//
	DoUnmount() (err error)
}

// Callbacks is the interface declaring the various callbacks that will be issued
// in response to a Volume instance while it is mounted. Note that some callbacks
// are expected to return both an error as well as a struct pointer. In the case of an
// error, the struct pointer should be <nil> as it will not be written to /dev/fuse.
// Finally, one callback is special: DoInit(). Provisioning a Volume instance involved
// providing the InitOut.MaxWrite to allow configuring the buffer pool used by the
// /dev/fuse read loop (including, of course, the reception of the InitIn up-call).
// The implementation of DoInit, therefore, should not expect returning an InitOut
// with a different MaxWrite to be honored.
type Callbacks interface {
	DoLookup(inHeader *InHeader, lookupIn *LookupIn) (lookupOut *LookupOut, errno syscall.Errno)
	DoForget(inHeader *InHeader, forgetIn *ForgetIn)
	DoGetAttr(inHeader *InHeader, getAttrIn *GetAttrIn) (getAttrOut *GetAttrOut, errno syscall.Errno)
	DoSetAttr(inHeader *InHeader, setAttrIn *SetAttrIn) (setAttrOut *SetAttrOut, errno syscall.Errno)
	DoReadLink(inHeader *InHeader) (readLinkOut *ReadLinkOut, errno syscall.Errno)
	DoSymLink(inHeader *InHeader, symLinkIn *SymLinkIn) (symLinkOut *SymLinkOut, errno syscall.Errno)
	DoMkNod(inHeader *InHeader, mkNodIn *MkNodIn) (mkNodOut *MkNodOut, errno syscall.Errno)
	DoMkDir(inHeader *InHeader, mkDirIn *MkDirIn) (mkDirOut *MkDirOut, errno syscall.Errno)
	DoUnlink(inHeader *InHeader, unlinkIn *UnlinkIn) (errno syscall.Errno)
	DoRmDir(inHeader *InHeader, rmDirIn *RmDirIn) (errno syscall.Errno)
	DoRename(inHeader *InHeader, renameIn *RenameIn) (errno syscall.Errno)
	DoLink(inHeader *InHeader, linkIn *LinkIn) (linkOut *LinkOut, errno syscall.Errno)
	DoOpen(inHeader *InHeader, openIn *OpenIn) (openOut *OpenOut, errno syscall.Errno)
	DoRead(inHeader *InHeader, readIn *ReadIn) (readOut *ReadOut, errno syscall.Errno)
	DoWrite(inHeader *InHeader, writeIn *WriteIn) (writeOut *WriteOut, errno syscall.Errno)
	DoStatFS(inHeader *InHeader) (statFSOut *StatFSOut, errno syscall.Errno)
	DoRelease(inHeader *InHeader, releaseIn *ReleaseIn) (errno syscall.Errno)
	DoFSync(inHeader *InHeader, fSyncIn *FSyncIn) (errno syscall.Errno)
	DoSetXAttr(inHeader *InHeader, setXAttrIn *SetXAttrIn) (errno syscall.Errno)
	DoGetXAttr(inHeader *InHeader, getXAttrIn *GetXAttrIn) (getXAttrOut *GetXAttrOut, errno syscall.Errno)
	DoListXAttr(inHeader *InHeader, listXAttrIn *ListXAttrIn) (listXAttrOut *ListXAttrOut, errno syscall.Errno)
	DoRemoveXAttr(inHeader *InHeader, removeXAttrIn *RemoveXAttrIn) (errno syscall.Errno)
	DoFlush(inHeader *InHeader, flushIn *FlushIn) (errno syscall.Errno)
	DoInit(inHeader *InHeader, initIn *InitIn) (initOut *InitOut, errno syscall.Errno)
	DoOpenDir(inHeader *InHeader, openDirIn *OpenDirIn) (openDirOut *OpenDirOut, errno syscall.Errno)
	DoReadDir(inHeader *InHeader, readDirIn *ReadDirIn) (readDirOut *ReadDirOut, errno syscall.Errno)
	DoReleaseDir(inHeader *InHeader, releaseDirIn *ReleaseDirIn) (errno syscall.Errno)
	DoFSyncDir(inHeader *InHeader, fSyncDirIn *FSyncDirIn) (errno syscall.Errno)
	DoGetLK(inHeader *InHeader, getLKIn *GetLKIn) (getLKOut *GetLKOut, errno syscall.Errno)
	DoSetLK(inHeader *InHeader, setLKIn *SetLKIn) (errno syscall.Errno)
	DoSetLKW(inHeader *InHeader, setLKWIn *SetLKWIn) (errno syscall.Errno)
	DoAccess(inHeader *InHeader, accessIn *AccessIn) (errno syscall.Errno)
	DoCreate(inHeader *InHeader, createIn *CreateIn) (createOut *CreateOut, errno syscall.Errno)
	DoInterrupt(inHeader *InHeader, interruptIn *InterruptIn)
	DoBMap(inHeader *InHeader, bMapIn *BMapIn) (bMapOut *BMapOut, errno syscall.Errno)
	DoDestroy(inHeader *InHeader) (errno syscall.Errno)
	DoPoll(inHeader *InHeader, pollIn *PollIn) (pollOut *PollOut, errno syscall.Errno)
	DoBatchForget(inHeader *InHeader, batchForgetIn *BatchForgetIn)
	DoFAllocate(inHeader *InHeader, fAllocateIn *FAllocateIn) (errno syscall.Errno)
	DoReadDirPlus(inHeader *InHeader, readDirPlusIn *ReadDirPlusIn) (readDirPlusOut *ReadDirPlusOut, errno syscall.Errno)
	DoRename2(inHeader *InHeader, rename2In *Rename2In) (errno syscall.Errno)
	DoLSeek(inHeader *InHeader, lSeekIn *LSeekIn) (lSeekOut *LSeekOut, errno syscall.Errno)
}

// NewVolume is called to create a Volume instance. Various callbacks listed in the Callbacks
// interface will be made while the Volume is mounted. The type of the file system, once mounted,
// will be "fuse" and, if non-empty, followed by a "." and the fuseSubtype (if supported... as it
// is on Linux). Non-root users may want to specify allowOther as TRUE to enable other non-root
// users access to the mount point. A chan error is also supplied to enable the Volume to indicate
// that it is no longer servicing FUSE upcalls (e.g. as a result of an intentional DoUnmount() call
// or some unexpected error reading from /dev/fuse).
func NewVolume(volumeName string, mountpointDirPath string, fuseSubtype string, maxRead uint32, maxWrite uint32, defaultPermissions bool, allowOther bool, callbacks Callbacks, logger *log.Logger, errChan chan error) (volume Volume) {
	volume = newVolume(volumeName, mountpointDirPath, fuseSubtype, maxRead, maxWrite, defaultPermissions, allowOther, callbacks, logger, errChan)
	return
}

// Linux Kernel to FUSE version mapping:
//   1) note that distributions may modify this mapping)
//   2) only kernel releases (i.e. vX.YY) are displayed here...
//   3) ...starting with v3.1
//   4) ...and skipping versions that didn't change fuse.h version
//   5) kernels up thru v3.6   have fuse.h in include/linux/
//   6) kernels at v3.7 and up have fuse.h in include/uapi/linux/
//
// Kernel    FUSE
// ------    ----
//  3.1      7.17
//  3.3      7.18
//  3.5      7.19
//  3.6      7.20   [Note: here and above, fuse.h in include/linux/     ]
//  3.9      7.21   [Note: here and below, fuse.h in include/uapi/linux/]
//  3.10     7.22
//  3.15     7.23
//  4.5      7.24
//  4.7      7.25
//  4.9      7.26
//  4.18     7.27
//  4.20     7.28
//  5.1      7.29
//  5.2      7.31a  [Note: fuse_init_out had a uint16_t field named "padding"      ]
//  5.4      7.31b  [      that in this kernel version named it     "map_alignment"]
//  5.10     7.32
//  5.11     7.33
//  5.14     7.34
//  5.16     7.35
//  5.17     7.36
//  6.1      7.37
//  6.2      7.38
//  6.6      7.39

const AttrSize = 88

type Attr struct {
	Ino       uint64
	Size      uint64
	Blocks    uint64
	ATimeSec  uint64
	MTimeSec  uint64
	CTimeSec  uint64
	ATimeNSec uint32
	MTimeNSec uint32
	CTimeNSec uint32
	Mode      uint32
	NLink     uint32
	UID       uint32
	GID       uint32
	RDev      uint32
	BlkSize   uint32
	Padding   uint32
}

const KStatFSSize = 80

type KStatFS struct {
	Blocks  uint64
	BFree   uint64
	BAvail  uint64
	Files   uint64
	FFree   uint64
	BSize   uint32
	NameLen uint32
	FRSize  uint32
	Padding uint32
	Spare   [6]uint32
}

const FileLockSize = 24

type FileLock struct {
	Start uint64
	End   uint64
	Type  uint32
	PID   uint32
}

const (
	SetAttrInValidMode        = uint32(1) << 0
	SetAttrInValidUID         = uint32(1) << 1
	SetAttrInValidGID         = uint32(1) << 2
	SetAttrInValidSize        = uint32(1) << 3
	SetAttrInValidATime       = uint32(1) << 4
	SetAttrInValidMTime       = uint32(1) << 5
	SetAttrInValidFH          = uint32(1) << 6
	SetAttrInValidATimeNow    = uint32(1) << 7
	SetAttrInValidMTimeNow    = uint32(1) << 8
	SetAttrInValidLockOwner   = uint32(1) << 9
	SetAttrInValidCTime       = uint32(1) << 10 // not supported
	SetAttrInValidKillSuidGID = uint32(1) << 11 // not supported
)

const (
	SetXAttrInCreate  = uint32(1)
	SetXAttrInReplace = uint32(2)
)

const (
	FOpenRequestRDONLY = uint32(syscall.O_RDONLY)
	FOpenRequestWRONLY = uint32(syscall.O_WRONLY)
	FOpenRequestRDWR   = uint32(syscall.O_RDWR)
	FOpenRequestAPPEND = uint32(syscall.O_APPEND)
	FOpenRequestCREAT  = uint32(syscall.O_CREAT)
	FOpenRequestEXCL   = uint32(syscall.O_EXCL)
	FOpenRequestSYNC   = uint32(syscall.O_SYNC)
	FOpenRequestTRUNC  = uint32(syscall.O_TRUNC)
)

const (
	FOpenResponseDirectIO    = uint32(1) << 0
	FOpenResponseKeepCache   = uint32(1) << 1
	FOpenResponseNonSeekable = uint32(1) << 2
	FOpenResponseCacheDir    = uint32(1) << 3
	FOpenResponseStream      = uint32(1) << 4
)

const (
	InitFlagsAsyncRead         = uint32(1) << 0
	InitFlagsPosixLocks        = uint32(1) << 1
	InitFlagsFileOps           = uint32(1) << 2
	InitFlagsAtomicOTrunc      = uint32(1) << 3
	InitFlagsExportSupport     = uint32(1) << 4
	InitFlagsBigWrites         = uint32(1) << 5
	InitFlagsDontMask          = uint32(1) << 6
	InitFlagsSpliceWrite       = uint32(1) << 7
	InitFlagsSpliceMove        = uint32(1) << 8
	InitFlagsSpliceRead        = uint32(1) << 9
	InitFlagsFLockLocks        = uint32(1) << 10
	InitFlagsHasIoctlDir       = uint32(1) << 11
	InitFlagsAutoInvalData     = uint32(1) << 12
	InitFlagsDoReadDirPlus     = uint32(1) << 13
	InitFlagsReaddirplusAuto   = uint32(1) << 14
	InitFlagsAsyncDio          = uint32(1) << 15
	InitFlagsWritebackCache    = uint32(1) << 16
	InitFlagsNoOpenSupport     = uint32(1) << 17
	InitFlagsParallelDirops    = uint32(1) << 18
	InitFlagsHandleKillpriv    = uint32(1) << 19
	InitFlagsPosixACL          = uint32(1) << 20
	InitFlagsAbortError        = uint32(1) << 21
	InitFlagsMaxPages          = uint32(1) << 22
	InitFlagsCacheSymlinks     = uint32(1) << 23
	InitFlagsNoOpendirSupport  = uint32(1) << 24
	InitFlagsExplicitInvalData = uint32(1) << 25
	InitFlagsMapAlignment      = uint32(1) << 26
	InitFlagsSubMounts         = uint32(1) << 27
	InitFlagsHandleKillprivV2  = uint32(1) << 28
	InitFlagsSetXattrExt       = uint32(1) << 29
	InitFalgsInitExt           = uint32(1) << 30
	InitFlagsInitReserved      = uint32(1) << 31

	InitFlags2SecurityCtx     = uint32(1) << 0
	InitFlags2HasInodeDAX     = uint32(1) << 1
	InitFlags2CreateSuppGroup = uint32(1) << 2
)

const (
	ReleaseFlush = uint32(1) << iota
	ReleaseFLockUnlock
)

const (
	GetAttrFH uint32 = uint32(1) << iota
)

const (
	LKFLock uint32 = uint32(1) << iota
)

const (
	WriteCache = uint32(1) << iota
	WriteLockOwner
	WriteKillPriv
)

const (
	ReadLockOwner = uint32(1) << (iota + 1)
)

const (
	IoctlCompat = uint32(1) << iota
	IoctlUnrestricted
	IoctlRetry
	Ioctl32Bit
	IoctlDir
	IoctlCompatX32
)

const IoctlIovecSize = 16

type IoctlIovec struct {
	Base uint64
	Len  uint64
}

const IoctlMaxIOV = 256

const (
	PollScheduleNotify = uint32(1) << iota
)

const (
	FSyncFDataSync = uint32(1) << iota
)

const (
	NofifyPoll = iota + 1
	NotifyInvalInode
	NotifyInvalEntry
	NotifyStore
	NotifyRetrieve
	NotifyDelete
	NotifyCodeMax
)

const EntryOutSize = 40 + AttrSize

type EntryOut struct {
	NodeID         uint64
	Generation     uint64
	EntryValidSec  uint64
	AttrValidSec   uint64
	EntryValidNSec uint32
	AttrValidNSec  uint32
	Attr
}

const ForgetOneSize = 16

type ForgetOne struct {
	NodeID  uint64
	NLookup uint64
}

const DirEntAlignment = 8 // applies to both DirEnt and DirEntPlus

const DirEntFixedPortionSize = 24 // + len(Name) and rounded up to DirEntAlignment boundary

type DirEnt struct {
	Ino     uint64
	Off     uint64 // position of next DirEnt
	NameLen uint32 // automatically computed ( == len(Name) )
	Type    uint32
	Name    []byte
}

const DirEntPlusFixedPortionSize = EntryOutSize + DirEntFixedPortionSize // + len(DirEnt.Name) and rounded up to DirEntAlignment boundary

type DirEntPlus struct {
	EntryOut
	DirEnt
}

const (
	OpCodeLookup   = uint32(1)
	OpCodeForget   = uint32(2) // no reply
	OpCodeGetAttr  = uint32(3)
	OpCodeSetAttr  = uint32(4)
	OpCodeReadLink = uint32(5)
	OpCodeSymLink  = uint32(6)

	OpCodeMkNod   = uint32(8)
	OpCodeMkDir   = uint32(9)
	OpCodeUnlink  = uint32(10)
	OpCodeRmDir   = uint32(11)
	OpCodeRename  = uint32(12)
	OpCodeLink    = uint32(13)
	OpCodeOpen    = uint32(14)
	OpCodeRead    = uint32(15)
	OpCodeWrite   = uint32(16)
	OpCodeStatFS  = uint32(17)
	OpCodeRelease = uint32(18)

	OpCodeFSync         = uint32(20)
	OpCodeSetXAttr      = uint32(21)
	OpCodeGetXAttr      = uint32(22)
	OpCodeListXAttr     = uint32(23)
	OpCodeRemoveXAttr   = uint32(24)
	OpCodeFlush         = uint32(25)
	OpCodeInit          = uint32(26)
	OpCodeOpenDir       = uint32(27)
	OpCodeReadDir       = uint32(28)
	OpCodeReleaseDir    = uint32(29)
	OpCodeFSyncDir      = uint32(30)
	OpCodeGetLK         = uint32(31)
	OpCodeSetLK         = uint32(32)
	OpCodeSetLKW        = uint32(33)
	OpCodeAccess        = uint32(34)
	OpCodeCreate        = uint32(35)
	OpCodeInterrupt     = uint32(36) // no reply
	OpCodeBMap          = uint32(37)
	OpCodeDestroy       = uint32(38)
	OpCodeIoCtl         = uint32(39) // unsupported
	OpCodePoll          = uint32(40)
	OpCodeNotifyReply   = uint32(41) // unsupported
	OpCodeBatchForget   = uint32(42) // no reply
	OpCodeFAllocate     = uint32(43)
	OpCodeReadDirPlus   = uint32(44)
	OpCodeRename2       = uint32(45)
	OpCodeLSeek         = uint32(46)
	OpCodeCopyFileRange = uint32(47) // unsupported
	OpCodeSetupMapping  = uint32(48) // unsupported
	OpCodeRemoveMapping = uint32(49) // unsupported
	OpCodeSyncFS        = uint32(50) // unsupported
	OpCodeTmpFile       = uint32(51) // unsupported

	OpCodeCuseInit = uint32(4096) // unsupported
)

const InHeaderSize = 40

type InHeader struct {
	Len     uint32 // includes InHeaderSize and any payload
	OpCode  uint32 // one of const OpCode*
	Unique  uint64
	NodeID  uint64
	UID     uint32
	GID     uint32
	PID     uint32
	Padding uint32
}

const OutHeaderSize = 16

type OutHeader struct {
	Len    uint32 // automatically computed; includes OutHeaderSize and any payload
	Error  int32
	Unique uint64
}

type LookupIn struct {
	Name []byte
}

const LookupOutSize = EntryOutSize

type LookupOut struct {
	EntryOut
}

const ForgetInSize = 8

type ForgetIn struct {
	NLookup uint64
}

const GetAttrInSize = 16

type GetAttrIn struct {
	Flags uint32 // mask of const GetAttrInFlags* bits
	Dummy uint32
	FH    uint64
}

const GetAttrOutSize = 16 + AttrSize

type GetAttrOut struct {
	AttrValidSec  uint64
	AttrValidNSec uint32
	Dummy         uint32
	Attr
}

const SetAttrInSize = 88

type SetAttrIn struct {
	Valid     uint32 // mask of const SetAttrInValid* bits
	Padding   uint32
	FH        uint64
	Size      uint64
	LockOwner uint64
	ATimeSec  uint64
	MTimeSec  uint64
	Unused2   uint64
	ATimeNSec uint32
	MTimeNSec uint32
	Unused3   uint32
	Mode      uint32
	Unused4   uint32
	UID       uint32
	GID       uint32
	Unused5   uint32
}

const SetAttrOutSize = 16 + AttrSize

type SetAttrOut struct {
	AttrValidSec  uint64
	AttrValidNSec uint32
	Dummy         uint32
	Attr
}

type ReadLinkOut struct {
	Data []byte
}

type SymLinkIn struct {
	Name []byte
	Data []byte // byte(0) separated from Name
}

const SymLinkOutSize = EntryOutSize

type SymLinkOut struct {
	EntryOut
}

const MkNodInFixedPortionSize = 16 // + len(Name)

type MkNodIn struct {
	Mode    uint32
	RDev    uint32
	UMask   uint32
	Padding uint32
	Name    []byte
}

const MkNodOutSize = EntryOutSize

type MkNodOut struct {
	EntryOut
}

const MkDirInFixedPortionSize = 8 // + len(Name)

type MkDirIn struct {
	Mode  uint32
	UMask uint32
	Name  []byte
}

const MkDirOutSize = EntryOutSize

type MkDirOut struct {
	EntryOut
}

type UnlinkIn struct {
	Name []byte
}

type RmDirIn struct {
	Name []byte
}

const RenameInFixedPortionSize = 8 // + len(OldName) + 1 + len(NewName)

type RenameIn struct {
	NewDir  uint64
	OldName []byte
	NewName []byte // byte(0) separated from OldName
}

const LinkInFixedPortionSize = 8 // + len(Name)

type LinkIn struct {
	OldNodeID uint64
	Name      []byte
}

const LinkOutSize = EntryOutSize

type LinkOut struct {
	EntryOut
}

const OpenInSize = 8

type OpenIn struct {
	Flags  uint32 // mask of const FOpenRequest* bits
	Unused uint32
}

const OpenOutSize = 16

type OpenOut struct {
	FH        uint64
	OpenFlags uint32 // mask of const FOpenResponse* bits
	Padding   uint32
}

const ReadInSize = 40

type ReadIn struct {
	FH        uint64
	Offset    uint64
	Size      uint32
	ReadFlags uint32
	LockOwner uint64
	Flags     uint32
	Padding   uint32
}

type ReadOut struct {
	Data []byte
}

const WriteInFixedPortionSize = 40 // + len(Data) a.k.a. Size

type WriteIn struct {
	FH         uint64
	Offset     uint64
	Size       uint32
	WriteFlags uint32
	LockOwner  uint64
	Flags      uint32
	Padding    uint32
	Data       []byte
}

const WriteOutSize = 8

type WriteOut struct {
	Size    uint32
	Padding uint32
}

const StatFSOutSize = KStatFSSize

type StatFSOut struct {
	KStatFS
}

const ReleaseInSize = 24

type ReleaseIn struct {
	FH           uint64
	Flags        uint32
	ReleaseFlags uint32
	LockOwner    uint64
}

const FSyncInSize = 16

type FSyncIn struct {
	FH         uint64
	FsyncFlags uint32
	Padding    uint32
}

const SetXAttrInFixedPortionSize = 16 // + len(Name) + 1 + len(Data)

type SetXAttrIn struct {
	Size          uint32 // == len(Name) + 1 + len(Data)
	Flags         uint32 // if not 0, either of SetXAttrIn*
	SetXAttrFlags uint32
	Padding       uint32
	Name          []byte
	Data          []byte // byte(0) separated from Name
}

const GetXAttrInFixedPortionSize = 8 // + len(Name)

type GetXAttrIn struct {
	Size    uint32 // == max len(GetXAttrOut.Data)
	Padding uint32
	Name    []byte
}

const GetXAttrOutSizeOnlySize = 8

type GetXAttrOut struct {
	Size    uint32 // only returned if GetXAttrIn.Size == 0
	Padding uint32 // only returned if GetXAttrIn.Size == 0
	Data    []byte // only returned if GetXAttrIn.Size != 0
}

const ListXAttrInSize = 8

type ListXAttrIn struct {
	Size    uint32 // == max len(ListXAttrOut.Name) with a '\0' between each Name element
	Padding uint32
}

const ListXAttrOutSizeOnlySize = 8

type ListXAttrOut struct {
	Size    uint32   // only returned if ListXAttrIn.Size == 0... SUM(each Name + trailing '\0')
	Padding uint32   // only returned if ListXAttrIn.Size == 0
	Name    [][]byte // only returned if ListXAttrIn.Size != 0... each with trailing '\0'
}

type RemoveXAttrIn struct {
	Name []byte
}

const FlushInSize = 24

type FlushIn struct {
	FH        uint64
	Unused    uint32
	Padding   uint32
	LockOwner uint64
}

const InitInMinSize = 8

type InitInMin struct {
	Major uint32
	Minor uint32
}

const InitInUpThru735Size = 16

type InitInUpThru735 struct {
	Major        uint32
	Minor        uint32
	MaxReadAhead uint32
	Flags        uint32 // mask of const InitFlags* bits
}

const InitInFrom736OnSize = 64

type InitInFrom736On struct {
	Major        uint32
	Minor        uint32
	MaxReadAhead uint32
	Flags        uint32 // mask of const InitFlags* bits
	Flags2       uint32 // mask of const InitFlags2* bits
	Unused       [11]uint32
}

const InitInSize = 64

type InitIn struct {
	Major        uint32
	Minor        uint32
	MaxReadAhead uint32
	Flags        uint32 // mask of const InitFlags* bits
	Flags2       uint32 // mask of const InitFlags2* bits
	Unused       [11]uint32
}

const InitOut717Size = 24

type InitOut717 struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
}

const InitOut718Thru720Size = 72

type InitOut718Thru720 struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
	DevMajor             uint32
	DevMinor             uint32
	Spare                [10]uint32
}

const InitOut721Thru722Size = 24

type InitOut721Thru722 struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
}

const InitOut723Thru727Size = 64

type InitOut723Thru727 struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
	TimeGran             uint32
	Spare                [9]uint32
}

const InitOut728Thru731aSize = 64

type InitOut728Thru731a struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
	TimeGran             uint32
	MaxPages             uint16
	Padding              uint16
	Spare                [8]uint32
}

const InitOut731bThru735Size = 64

type InitOut731bThru735 struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
	TimeGran             uint32
	MaxPages             uint16
	MapAlignment         uint16
	Spare                [8]uint32
}

const InitOut736AndBeyondSize = 64

type InitOut736AndBeyond struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
	TimeGran             uint32
	MaxPages             uint16
	MapAlignment         uint16
	Flags2               uint32
	Unused               [7]uint32
}

const InitOutSize = 64

type InitOut struct {
	Major                uint32
	Minor                uint32
	MaxReadAhead         uint32
	Flags                uint32 // mask of const InitFlags* bits
	MaxBackground        uint16
	CongestionThreshhold uint16
	MaxWrite             uint32
	TimeGran             uint32
	MaxPages             uint16
	MapAlignment         uint16
	Flags2               uint32
	Unused               [7]uint32
}

const OpenDirInSize = 8

type OpenDirIn struct {
	Flags  uint32
	Unused uint32
}

const OpenDirOutSize = 16

type OpenDirOut struct {
	FH        uint64
	OpenFlags uint32 // mask of const FOpen* bits
	Padding   uint32
}

const ReadDirInSize = 40

type ReadDirIn struct {
	FH        uint64
	Offset    uint64
	Size      uint32
	ReadFlags uint32
	LockOwner uint64
	Flags     uint32
	Padding   uint32
}

type ReadDirOut struct {
	DirEnt []DirEnt // aligned on DirEntAlignment boundaries
}

const ReleaseDirInSize = 24

type ReleaseDirIn struct {
	FH           uint64
	Flags        uint32
	ReleaseFlags uint32
	LockOwner    uint64
}

const FSyncDirInSize = 16

type FSyncDirIn struct {
	FH         uint64
	FsyncFlags uint32
	Padding    uint32
}

const GetLKInSize = 16 + FileLockSize + 8

type GetLKIn struct {
	FH    uint64
	Owner uint64
	FileLock
	LKFlags uint32
	Padding uint32
}

const GetLKOutSize = FileLockSize

type GetLKOut struct {
	FileLock
}

const SetLKInSize = 16 + FileLockSize + 8

type SetLKIn struct {
	FH    uint64
	Owner uint64
	FileLock
	LKFlags uint32
	Padding uint32
}

const SetLKWInSize = 16 + FileLockSize + 8

type SetLKWIn struct {
	FH    uint64
	Owner uint64
	FileLock
	LKFlags uint32
	Padding uint32
}

const AccessInSize = 8

type AccessIn struct {
	Mask    uint32
	Padding uint32
}

const CreateInFixedPortionSize = 16 // + len(Name)

type CreateIn struct {
	Flags   uint32 // mask of const FOpenRequest* bits
	Mode    uint32
	UMask   uint32
	Padding uint32
	Name    []byte
}

const CreateOutSize = EntryOutSize + 16

type CreateOut struct {
	EntryOut
	FH        uint64
	OpenFlags uint32 // mask of const FOpenResponse* bits
	Padding   uint32
}

const InterruptInSize = 8

type InterruptIn struct {
	Unique uint64
}

const BMapInSize = 16

type BMapIn struct {
	Block     uint64
	BlockSize uint32
	Padding   uint32
}

const BMapOutSize = 8

type BMapOut struct {
	Block uint64
}

const IoCtlInFixedPortionSize = 32 // + len(InBuf) a.k.a. InSize

type IoCtlIn struct { // unsupported
	FH      uint64
	Flags   uint32
	Cmd     uint32
	Arg     uint64 // == a uintptr to InBuf
	InSize  uint32
	OutSize uint32
	InBuf   []byte // == nil if InSize == 0
}

const IoCtlOutFixedPortionSize = 16 // + (InIovs * IoctlIovecSize) + (OutIovs * IoctlIovecSize)

type IoCtlOut struct { // unsupported
	Result  uint32
	Flags   uint32
	InIovs  uint32
	OutIovs uint32
	InIov   []IoctlIovec // len(IoIov) == InIovs
	OutIov  []IoctlIovec // len(OutIov) == OutIovs
}

const PollInSize = 24

type PollIn struct {
	FH     uint64
	KH     uint64
	Flags  uint32
	Events uint32
}

const PollOutSize = 8

type PollOut struct {
	REvents uint32
	Padding uint32
}

const BatchForgetInFixedPortionSize = 8 // + (Count * ForgetOneSize)

type BatchForgetIn struct {
	Count  uint32
	Dummy  uint32
	Forget []ForgetOne // len(Forget) == Count
}

const FAllocateInSize = 32

type FAllocateIn struct {
	FH      uint64
	Offset  uint64
	Length  uint64
	Mode    uint32
	Padding uint32
}

const ReadDirPlusInSize = 40

type ReadDirPlusIn struct {
	FH        uint64
	Offset    uint64
	Size      uint32
	ReadFlags uint32
	LockOwner uint64
	Flags     uint32
	Padding   uint32
}

type ReadDirPlusOut struct {
	DirEntPlus []DirEntPlus // aligned on DirEntAlignment boundaries
}

const Rename2InFixedPortionSize = 16 // + len(OldName) + 1 + len(NewName)

type Rename2In struct {
	NewDir  uint64
	Flags   uint32
	Padding uint32
	OldName []byte
	NewName []byte // byte(0) separated from OldName
}

const LSeekInSize = 24

type LSeekIn struct {
	FH      uint64
	Offset  uint64
	Whence  uint32
	Padding uint32
}

const LSeekOutSize = 8

type LSeekOut struct {
	Offset uint64
}
