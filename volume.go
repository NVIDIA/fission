// Copyright (c) 2015-2025, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package fission

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type volumeStruct struct {
	volumeName         string
	mountpointDirPath  string
	fuseSubtype        string
	maxRead            uint32
	maxWrite           uint32
	defaultPermissions bool
	allowOther         bool
	callbacks          Callbacks
	logger             *log.Logger
	errChan            chan error
	devFuseFDReadSize  uint32 // InHeaderSize + WriteInSize + InitOut.MaxWrite
	devFuseFDReadPool  sync.Pool
	devFuseFD          int
	devFuseFile        *os.File
	devFuseFDReaderWG  sync.WaitGroup
	callbacksWG        sync.WaitGroup
	fuseMajor          uint32
	fuseMinor          uint32
	doInitWG           sync.WaitGroup
}

const (
	recvmsgFlags   = int(0)
	recvmsgOOBSize = 32
	recvmsgPSize   = 4

	devLinuxFusePath = "/dev/fuse"

	devFuseFDReadSizeMin = 2 * 4096
)

func newVolume(volumeName, mountpointDirPath, fuseSubtype string, maxRead, maxWrite uint32, defaultPermissions, allowOther bool, callbacks Callbacks, logger *log.Logger, errChan chan error) (volume *volumeStruct) {
	var (
		devFuseFDReadSize uint32
	)

	// Note: The following assumes maxWrite is either zero (indicating a ReadOnly Volume)
	//       or sufficiently large to make the WriteIn case the largest message possible
	//
	devFuseFDReadSize = InHeaderSize + WriteInFixedPortionSize + maxWrite
	if devFuseFDReadSize < devFuseFDReadSizeMin {
		devFuseFDReadSize = devFuseFDReadSizeMin
	}

	volume = &volumeStruct{
		volumeName:         volumeName,
		mountpointDirPath:  mountpointDirPath,
		fuseSubtype:        fuseSubtype,
		maxRead:            maxRead,
		maxWrite:           maxWrite,
		defaultPermissions: defaultPermissions,
		allowOther:         allowOther,
		callbacks:          callbacks,
		logger:             logger,
		errChan:            errChan,
		devFuseFDReadSize:  devFuseFDReadSize,
		fuseMajor:          0,
		fuseMinor:          0,
	}

	volume.devFuseFDReadPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, volume.devFuseFDReadSize) // len == cap
			return &buf
		},
	}

	return
}

func (volume *volumeStruct) DoMount() (err error) {
	var (
		childOpenFDs             []int
		fsnameOption             string
		fuseSubtypeOption        string
		fusermountChildWriteFile *os.File
		fusermountLineCount      uint32
		fusermountParentReadFile *os.File
		fusermountProgramPath    string
		fusermountSocketPair     [2]int
		gid                      int
		gidOption                string
		maxReadOption            string
		mountCmd                 *exec.Cmd
		mountCmdStderrPipe       io.ReadCloser
		mountCmdStdoutPipe       io.ReadCloser
		mountOptions             string
		recvmsgOOB               [recvmsgOOBSize]byte
		recvmsgOOBN              int
		recvmsgP                 [recvmsgPSize]byte
		rootMode                 uint32
		rootModeOption           string
		scanPipeWaitGroup        sync.WaitGroup
		socketControlMessages    []syscall.SocketControlMessage
		syscallRecvmsgAborted    bool
		uid                      int
		uidOption                string
		unmountCmd               *exec.Cmd
	)

	fusermountProgramPath, err = exec.LookPath("fusermount")
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to find program `fusermount`: %v", volume.volumeName, err)
		return
	}

	unmountCmd = exec.Command(fusermountProgramPath, "-u", volume.mountpointDirPath)
	_, _ = unmountCmd.CombinedOutput()

	fusermountSocketPair, err = syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to create socketpairFDs: %v", volume.volumeName, err)
		return
	}

	fusermountChildWriteFile = os.NewFile(uintptr(fusermountSocketPair[0]), "fusermountChildWriteFile")
	fusermountParentReadFile = os.NewFile(uintptr(fusermountSocketPair[1]), "fusermountParentReadFile")

	syscallRecvmsgAborted = false

	defer func() {
		var (
			localErr error
		)

		if !syscallRecvmsgAborted {
			localErr = fusermountChildWriteFile.Close()
			if localErr != nil {
				volume.logger.Printf("Volume %s DoMount() unable to close fusermountChildWriteFile: %v", volume.volumeName, err)
			}
		}

		localErr = fusermountParentReadFile.Close()
		if localErr != nil {
			volume.logger.Printf("Volume %s DoMount() unable to close fusermountParentReadFile: %v", volume.volumeName, err)
		}
	}()

	rootMode = syscall.S_IFDIR
	rootModeOption = fmt.Sprintf("rootmode=%o", rootMode)

	uid = syscall.Geteuid()
	gid = syscall.Getegid()

	uidOption = fmt.Sprintf("user_id=%d", uid)
	gidOption = fmt.Sprintf("group_id=%d", gid)
	fsnameOption = "fsname=" + volume.volumeName
	maxReadOption = fmt.Sprintf("max_read=%d", volume.maxRead)

	mountOptions = rootModeOption +
		"," + uidOption +
		"," + gidOption +
		"," + fsnameOption +
		"," + maxReadOption

	if volume.maxWrite == 0 {
		mountOptions += ",ro"
	} else {
		mountOptions += ",rw"
	}

	if volume.defaultPermissions {
		mountOptions += ",default_permissions"
	}
	if volume.allowOther {
		mountOptions += ",allow_other"
	}

	if volume.fuseSubtype != "" {
		fuseSubtypeOption = "subtype=" + volume.fuseSubtype
		mountOptions += "," + fuseSubtypeOption
	}

	mountCmd = &exec.Cmd{
		Path: fusermountProgramPath,
		Args: []string{
			fusermountProgramPath,
			"-o", mountOptions,
			"--", volume.mountpointDirPath,
		},
		Env:          append(os.Environ(), "_FUSE_COMMFD=3"),
		Dir:          "",
		Stdin:        nil,
		Stdout:       nil, // This will be redirected to mountCmdStdoutPipe below
		Stderr:       nil, // This will be redirected to mountCmdStderrPipe below
		ExtraFiles:   []*os.File{fusermountChildWriteFile},
		SysProcAttr:  nil,
		Process:      nil,
		ProcessState: nil,
	}

	scanPipeWaitGroup.Add(2)

	mountCmdStdoutPipe, err = mountCmd.StdoutPipe()
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to create mountCmd.StdoutPipe: %v", volume.volumeName, err)
		return
	}

	go volume.scanPipe("mountCmdStdoutPipe", mountCmdStdoutPipe, nil, &scanPipeWaitGroup)

	mountCmdStderrPipe, err = mountCmd.StderrPipe()
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to create mountCmd.StderrPipe: %v", volume.volumeName, err)
		return
	}

	go volume.scanPipe("mountCmdStderrPipe", mountCmdStderrPipe, &fusermountLineCount, &scanPipeWaitGroup)

	volume.doInitWG.Add(1)

	err = mountCmd.Start()
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to mountCmd.Start(): %v", volume.volumeName, err)
		return
	}

	go volume.awaitScanPipe(&scanPipeWaitGroup, &fusermountLineCount, &syscallRecvmsgAborted, fusermountChildWriteFile)

	_, recvmsgOOBN, _, _, err = syscall.Recvmsg(
		int(fusermountParentReadFile.Fd()),
		recvmsgP[:],
		recvmsgOOB[:],
		recvmsgFlags)
	if syscallRecvmsgAborted {
		err = fmt.Errorf("Volume %s DoMount() failed", volume.volumeName)
		return
	}
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() got error from fusermount: %v", volume.volumeName, err)
		return
	}

	socketControlMessages, err = syscall.ParseSocketControlMessage(recvmsgOOB[:recvmsgOOBN])
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to syscall.ParseSocketControlMessage(): %v", volume.volumeName, err)
		return
	}
	if len(socketControlMessages) != 1 {
		volume.logger.Printf("Volume %s DoMount() got unexpected len(socketControlMessages): %v", volume.volumeName, len(socketControlMessages))
		return
	}

	childOpenFDs, err = syscall.ParseUnixRights(&socketControlMessages[0])
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() unable to syscall.ParseUnixRights(): %v", volume.volumeName, err)
		return
	}
	if len(childOpenFDs) != 1 {
		volume.logger.Printf("Volume %s DoMount() got unexpected len(childOpenFDs): %v", volume.volumeName, len(childOpenFDs))
		return
	}

	volume.devFuseFD = childOpenFDs[0]
	volume.devFuseFile = os.NewFile(uintptr(volume.devFuseFD), devLinuxFusePath)

	volume.devFuseFDReaderWG.Add(1)
	go volume.devFuseFDReader()

	err = mountCmd.Wait()
	if err != nil {
		volume.logger.Printf("Volume %s DoMount() got error from fusermount: %v", volume.volumeName, err)
		return
	}

	volume.doInitWG.Wait()

	volume.logger.Printf("Volume %s mounted on mountpoint %s", volume.volumeName, volume.mountpointDirPath)

	err = nil
	return
}

func (volume *volumeStruct) DoUnmount() (err error) {
	var (
		fusermountProgramPath    string
		unmountCmd               *exec.Cmd
		unmountCmdCombinedOutput []byte
	)

	fusermountProgramPath, err = exec.LookPath("fusermount")
	if err != nil {
		volume.logger.Printf("DoUnmount() unable to find program `fusermount`: %v", err)
		return
	}

	unmountCmd = exec.Command(fusermountProgramPath, "-u", "-q", "-z", "--", volume.mountpointDirPath)
	unmountCmdCombinedOutput, err = unmountCmd.CombinedOutput()
	if err != nil {
		volume.logger.Printf("DoUnmount() unable to unmount %s (%v): %s", volume.volumeName, err, string(unmountCmdCombinedOutput))
		return
	}

	err = syscall.Close(volume.devFuseFD)
	if err != nil {
		volume.logger.Printf("DoUnmount() unable to close /dev/fuse: %v", err)
		return
	}

	volume.devFuseFDReaderWG.Wait()

	volume.logger.Printf("Volume %s unmounted from mountpoint %s", volume.volumeName, volume.mountpointDirPath)

	err = nil
	return
}

func (volume *volumeStruct) scanPipe(name string, pipe io.ReadCloser, lineCount *uint32, wg *sync.WaitGroup) {
	var (
		pipeScanner *bufio.Scanner = bufio.NewScanner(pipe)
	)

	for pipeScanner.Scan() {
		if lineCount != nil {
			atomic.AddUint32(lineCount, 1)
		}
		volume.logger.Printf("Volume %s DoMount() %s: %s", volume.volumeName, name, pipeScanner.Text())
	}

	wg.Done()
}

func (volume *volumeStruct) awaitScanPipe(wg *sync.WaitGroup, lineCount *uint32, syscallRecvmsgAborted *bool, fusermountChildWriteFile *os.File) {
	var (
		err error
	)

	wg.Wait()

	if *lineCount != 0 {
		*syscallRecvmsgAborted = true

		volume.logger.Printf("Volume %s DoMount() got error(s) from fusermount - exiting", volume.volumeName)

		err = syscall.Close(int(fusermountChildWriteFile.Fd()))
		if err != nil {
			volume.logger.Printf("DoMount() unable to close fusermountChildWriteFile: %v", err)
		}
	}
}

func (volume *volumeStruct) devFuseFDReadPoolGet() (devFuseFDReadBufPtr *[]byte) {
	devFuseFDReadBufPtr = volume.devFuseFDReadPool.Get().(*[]byte)
	return
}

func (volume *volumeStruct) devFuseFDReadPoolPut(devFuseFDReadBufPtr *[]byte) {
	*devFuseFDReadBufPtr = (*devFuseFDReadBufPtr)[:cap(*devFuseFDReadBufPtr)] // len == cap
	volume.devFuseFDReadPool.Put(devFuseFDReadBufPtr)
}

func (volume *volumeStruct) devFuseFDReader() {
	var (
		bytesRead           int
		devFuseFDReadBuf    []byte
		devFuseFDReadBufPtr *[]byte
		err                 error
	)

	for {
		devFuseFDReadBufPtr = volume.devFuseFDReadPoolGet()
		devFuseFDReadBuf = *devFuseFDReadBufPtr

	RetrySyscallRead:
		bytesRead, err = syscall.Read(volume.devFuseFD, devFuseFDReadBuf)
		if err != nil {
			// First check for EINTR

			if err.Error() == "interrupted system call" {
				goto RetrySyscallRead
			}

			// Now that we are not retrying syscall.Read(), discard devFuseFDReadBuf

			*devFuseFDReadBufPtr = devFuseFDReadBuf
			volume.devFuseFDReadPoolPut(devFuseFDReadBufPtr)

			if err.Error() == "operation not permitted" {
				// Special case... simply retry the Read
				continue
			}

			// Time to exit...but first await outstanding Callbacks

			volume.callbacksWG.Wait()
			volume.devFuseFDReaderWG.Done()

			// Signal errChan that we are exiting (passing <nil> if due to close of volume.devFuseFD)

			switch err.Error() {
			case "no such device":
				volume.errChan <- nil
			case "operation not supported by device":
				volume.errChan <- nil
			default:
				volume.logger.Printf("Exiting due to /dev/fuse Read err: %v", err)
				volume.errChan <- err
			}

			return
		}

		devFuseFDReadBuf = devFuseFDReadBuf[:bytesRead]
		*devFuseFDReadBufPtr = devFuseFDReadBuf

		// Dispatch goroutine to process devFuseFDReadBuf

		volume.callbacksWG.Add(1)
		go volume.processDevFuseFDReadBuf(devFuseFDReadBufPtr)
	}
}

func (volume *volumeStruct) processDevFuseFDReadBuf(devFuseFDReadBufPtr *[]byte) {
	var (
		devFuseFDReadBuf = *devFuseFDReadBufPtr
		inHeader         *InHeader
	)

	if len(devFuseFDReadBuf) < InHeaderSize {
		// All we can do is just drop it
		volume.logger.Printf("Read malformed message from /dev/fuse")
		*devFuseFDReadBufPtr = devFuseFDReadBuf
		volume.devFuseFDReadPoolPut(devFuseFDReadBufPtr)
		volume.callbacksWG.Done()
		return
	}

	inHeader = &InHeader{
		Len:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBuf[0])),
		OpCode:  *(*uint32)(unsafe.Pointer(&devFuseFDReadBuf[4])),
		Unique:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBuf[8])),
		NodeID:  *(*uint64)(unsafe.Pointer(&devFuseFDReadBuf[16])),
		UID:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBuf[24])),
		GID:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBuf[28])),
		PID:     *(*uint32)(unsafe.Pointer(&devFuseFDReadBuf[32])),
		Padding: *(*uint32)(unsafe.Pointer(&devFuseFDReadBuf[36])),
	}

	switch inHeader.OpCode {
	case OpCodeLookup:
		volume.doLookup(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeForget:
		volume.doForget(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeGetAttr:
		volume.doGetAttr(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeSetAttr:
		volume.doSetAttr(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeReadLink:
		volume.doReadLink(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeSymLink:
		volume.doSymLink(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeMkNod:
		volume.doMkNod(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeMkDir:
		volume.doMkDir(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeUnlink:
		volume.doUnlink(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeRmDir:
		volume.doRmDir(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeRename:
		volume.doRename(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeLink:
		volume.doLink(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeOpen:
		volume.doOpen(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeRead:
		volume.doRead(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeWrite:
		volume.doWrite(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeStatFS:
		volume.doStatFS(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeRelease:
		volume.doRelease(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeFSync:
		volume.doFSync(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeSetXAttr:
		volume.doSetXAttr(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeGetXAttr:
		volume.doGetXAttr(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeListXAttr:
		volume.doListXAttr(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeRemoveXAttr:
		volume.doRemoveXAttr(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeFlush:
		volume.doFlush(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeInit:
		volume.doInit(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeOpenDir:
		volume.doOpenDir(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeReadDir:
		volume.doReadDir(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeReleaseDir:
		volume.doReleaseDir(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeFSyncDir:
		volume.doFSyncDir(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeGetLK:
		volume.doGetLK(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeSetLK:
		volume.doSetLK(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeSetLKW:
		volume.doSetLKW(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeAccess:
		volume.doAccess(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeCreate:
		volume.doCreate(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeInterrupt:
		volume.doInterrupt(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeBMap:
		volume.doBMap(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeDestroy:
		volume.doDestroy(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeIoCtl:
		volume.devFuseFDWriter(inHeader, syscall.EINVAL)
	case OpCodePoll:
		volume.doPoll(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeBatchForget:
		volume.doBatchForget(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeFAllocate:
		volume.doFAllocate(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeReadDirPlus:
		volume.doReadDirPlus(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeRename2:
		volume.doRename2(inHeader, devFuseFDReadBuf[InHeaderSize:])
	case OpCodeLSeek:
		volume.doLSeek(inHeader, devFuseFDReadBuf[InHeaderSize:])
	default:
		volume.devFuseFDWriter(inHeader, syscall.ENOSYS)
	}

	*devFuseFDReadBufPtr = devFuseFDReadBuf
	volume.devFuseFDReadPoolPut(devFuseFDReadBufPtr)
	volume.callbacksWG.Done()
}

func (volume *volumeStruct) devFuseFDWriter(inHeader *InHeader, errno syscall.Errno, bufs ...[]byte) {
	var (
		buf          []byte
		bytesWritten uintptr
		iovec        []syscall.Iovec
		iovecSpan    uintptr
		outHeader    []byte
	)

	// First, log any syscall.ENOSYS responses

	if syscall.ENOSYS == errno {
		volume.logger.Printf("Read unsupported/unrecognized message OpCode == %v", inHeader.OpCode)
	}

	// Construct outHeader w/out knowing iovecSpan and put it in iovec[0]

	outHeader = make([]byte, OutHeaderSize)

	iovecSpan = uintptr(OutHeaderSize)

	*(*uint32)(unsafe.Pointer(&outHeader[0])) = uint32(iovecSpan) // Updated later
	*(*int32)(unsafe.Pointer(&outHeader[4])) = -int32(errno)
	*(*uint64)(unsafe.Pointer(&outHeader[8])) = inHeader.Unique

	iovec = make([]syscall.Iovec, 1, len(bufs)+1)

	iovec[0] = syscall.Iovec{Base: &outHeader[0], Len: uint64(OutHeaderSize)}

	// Construct iovec elements for supplied bufs (if any)

	for _, buf = range bufs {
		if len(buf) != 0 {
			iovec = append(iovec, syscall.Iovec{Base: &buf[0], Len: uint64(len(buf))})
			iovecSpan += uintptr(len(buf))
		}
	}

	// Now go back and update outHeader

	*(*uint32)(unsafe.Pointer(&outHeader[0])) = uint32(iovecSpan)

	// Finally, send iovec to /dev/fuse

RetrySyscallWriteV:
	bytesWritten, _, errno = syscall.Syscall(
		syscall.SYS_WRITEV,
		uintptr(volume.devFuseFD),
		uintptr(unsafe.Pointer(&iovec[0])),
		uintptr(len(iovec)))
	if errno == 0 {
		if bytesWritten != iovecSpan {
			volume.logger.Printf("Write to /dev/fuse returned bad bytesWritten: %v", bytesWritten)
		}
	} else {
		if syscall.EINTR == errno {
			goto RetrySyscallWriteV
		}
		volume.logger.Printf("Write to /dev/fuse returned bad errno: %v", errno)
	}
}

func cloneByteSlice(inBuf []byte, andTrimTrailingNullByte bool) (outBuf []byte) {
	outBuf = make([]byte, len(inBuf))
	if len(inBuf) != 0 {
		_ = copy(outBuf, inBuf)
		if andTrimTrailingNullByte && (outBuf[len(outBuf)-1] == 0) {
			outBuf = outBuf[:len(outBuf)-1]
		}
	}
	return
}
