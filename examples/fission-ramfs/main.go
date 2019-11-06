package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/swiftstack/sortedmap"

	"github.com/swiftstack/fission"
)

const (
	mountFlags = uintptr(0)

	initOutFlagsMask            = fission.InitFlagsAsyncRead | fission.InitFlagsBigWrites | fission.InitFlagsDontMask | fission.InitFlagsAutoInvalData | fission.InitFlagsDoReadDirPlus
	initOutMaxBackgound         = uint16(100)
	initOutCongestionThreshhold = uint16(0)
	initOutMaxWrite             = uint32(128 * 1024) // 128KiB... the max write size in Linux FUSE at this time
)

type inodeStruct struct {
	nodeID      uint64             // key in globals.inodeMap
	attr        fission.Attr       // (attr.Mode&syscall.S_IFMT) must be one of syscall.{S_IFDIR|S_IFREG|S_IFLNK}
	xattrMap    sortedmap.LLRBTree // key is xattr Name ([]byte); value is xattr Data ([]byte)
	dirEntryMap sortedmap.LLRBTree // [S_IFDIR only] key is basename of dirEntry ([]byte); value is nodeID (uint64)
	fileData    []byte             // [S_IFREG only] zero-filled up to attr.Size contents of file
	symlinkData []byte             // [S_IFLNK only] target path of symlink
}

type xattrMapDummyStruct struct{}
type dirEntryMapDummyStruct struct{}

type globalsStruct struct {
	sync.Mutex
	programPath      string
	mountPoint       string
	programName      string
	volumeName       string
	logger           *log.Logger
	errChan          chan error
	xattrMapDummy    *xattrMapDummyStruct
	dirEntryMapDummy *dirEntryMapDummyStruct
	inodeMap         map[uint64]*inodeStruct
	volume           fission.Volume
}

var globals globalsStruct

func main() {
	var (
		err        error
		ok         bool
		rootInode  *inodeStruct
		signalChan chan os.Signal
	)

	if 2 != len(os.Args) {
		fmt.Printf("Usage: %s <mount_point>\n", os.Args[0])
		os.Exit(0)
	}

	globals.programPath = os.Args[0]
	globals.mountPoint = os.Args[1]

	globals.programName = path.Base(globals.programPath)
	globals.volumeName = path.Base(globals.mountPoint)

	globals.logger = log.New(os.Stdout, "", log.Ldate|log.Ltime) // |log.Lmicroseconds|log.Lshortfile

	globals.errChan = make(chan error, 1)

	globals.xattrMapDummy = nil
	globals.dirEntryMapDummy = nil

	rootInode = &inodeStruct{
		nodeID:      1,
		attr:        fission.Attr{},
		xattrMap:    sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.xattrMapDummy),
		dirEntryMap: sortedmap.NewLLRBTree(sortedmap.CompareByteSlice, globals.dirEntryMapDummy),
		fileData:    nil,
		symlinkData: nil,
	}

	ok, err = rootInode.dirEntryMap.Put([]byte("."), uint64(1))
	if nil != err {
		globals.logger.Printf("rootInode.dirEntryMap.Put([]byte(\".\"), uint64(1)) failed: %v", globals.volumeName, err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("rootInode.dirEntryMap.Put([]byte(\".\"), uint64(1)) returned !ok", globals.volumeName)
		os.Exit(1)
	}
	ok, err = rootInode.dirEntryMap.Put([]byte(".."), uint64(1))
	if nil != err {
		globals.logger.Printf("rootInode.dirEntryMap.Put([]byte(\".\"), uint64(1)) failed: %v", globals.volumeName, err)
		os.Exit(1)
	}
	if !ok {
		globals.logger.Printf("rootInode.dirEntryMap.Put([]byte(\".\"), uint64(1)) returned !ok", globals.volumeName)
		os.Exit(1)
	}

	globals.inodeMap = make(map[uint64]*inodeStruct)

	globals.inodeMap[1] = rootInode

	globals.volume = fission.NewVolume(globals.volumeName, globals.mountPoint, mountFlags, initOutMaxWrite, &globals, globals.logger, globals.errChan)

	err = globals.volume.DoMount()
	if nil != err {
		globals.logger.Printf("fission.DoMount() of %s failed: %v", globals.volumeName, err)
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
		globals.logger.Printf("fission.DoUnmount() of %s failed: %v", globals.volumeName, err)
		os.Exit(2)
	}
}

func (dummy *xattrMapDummyStruct) DumpKey(key sortedmap.Key) (keyAsString string, err error) {
	globals.logger.Printf("func (dummy *xattrMapDummyStruct) DumpKey() not implemented")
	os.Exit(1)
	return // Unreachable
}
func (dummy *xattrMapDummyStruct) DumpValue(value sortedmap.Value) (valueAsString string, err error) {
	globals.logger.Printf("func (dummy *xattrMapDummyStruct) DumpValue() not implemented")
	os.Exit(1)
	return // Unreachable
}
func (dummy *dirEntryMapDummyStruct) DumpKey(key sortedmap.Key) (keyAsString string, err error) {
	globals.logger.Printf("func (dummy *dirEntryMapDummyStruct) DumpKey() not implemented")
	os.Exit(1)
	return // Unreachable
}
func (dummy *dirEntryMapDummyStruct) DumpValue(value sortedmap.Value) (valueAsString string, err error) {
	globals.logger.Printf("func (dummy *dirEntryMapDummyStruct) DumpValue() not implemented")
	os.Exit(1)
	return // Unreachable
}
func (dummy *globalsStruct) DoLookup(inHeader *fission.InHeader, lookupIn *fission.LookupIn) (lookupOut *fission.LookupOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoForget(inHeader *fission.InHeader, forgetIn *fission.ForgetIn) {
	return
}

func (dummy *globalsStruct) DoGetAttr(inHeader *fission.InHeader, getAttrIn *fission.GetAttrIn) (getAttrOut *fission.GetAttrOut, errno syscall.Errno) {
	getAttrOut = &fission.GetAttrOut{
		AttrValidSec:  0,
		AttrValidNSec: 0,
		Dummy:         0,
		Attr: fission.Attr{
			Ino:       inHeader.NodeID,
			Size:      0,
			Blocks:    0,
			ATimeSec:  0,
			MTimeSec:  0,
			CTimeSec:  0,
			ATimeNSec: 0,
			MTimeNSec: 0,
			CTimeNSec: 0,
			Mode:      uint32(syscall.S_IFDIR | syscall.S_IRWXU | syscall.S_IRWXG | syscall.S_IRWXO),
			NLink:     0,
			UID:       0,
			GID:       0,
			RDev:      0,
			BlkSize:   0,
			Padding:   0,
		},
	}

	errno = 0

	return
}

func (dummy *globalsStruct) DoSetAttr(inHeader *fission.InHeader, setAttrIn *fission.SetAttrIn) (errno syscall.Errno) {
	return syscall.ENOSYS
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
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoGetXAttr(inHeader *fission.InHeader, getXAttrIn *fission.GetXAttrIn) (getXAttrOut *fission.GetXAttrOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoListXAttr(inHeader *fission.InHeader, listXAttrIn *fission.ListXAttrIn) (listXAttrOut *fission.ListXAttrOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoRemoveXAttr(inHeader *fission.InHeader, removeXAttrIn *fission.RemoveXAttrIn) (errno syscall.Errno) {
	return syscall.ENOSYS
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
	openDirOut = &fission.OpenDirOut{
		FH:        inHeader.NodeID,
		OpenFlags: 0,
		Padding:   0,
	}

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
	return nil, syscall.ENOSYS
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
	return nil, syscall.ENOSYS
}
func (dummy *globalsStruct) DoRename2(inHeader *fission.InHeader, rename2In *fission.Rename2In) (errno syscall.Errno) {
	return syscall.ENOSYS
}
func (dummy *globalsStruct) DoLSeek(inHeader *fission.InHeader, lSeekIn *fission.LSeekIn) (lSeekOut *fission.LSeekOut, errno syscall.Errno) {
	return nil, syscall.ENOSYS
}
