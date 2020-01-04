package fission

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const (
	devOsxFusePrefix      = "/dev/osxfuse" // Suffix is a non-negative decimal number starting with 0, 1, ...
	osxFuseLoadPath       = "/Library/Filesystems/osxfuse.fs/Contents/Resources/load_osxfuse"
	osxFuseMountPath      = "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse"
	osxFuseMountCallByEnv = "MOUNT_OSXFUSE_CALL_BY_LIB=" // No value should be appended
	osxFuseMountCommFDEnv = "_FUSE_COMMFD=3"             // References first (only) element of osxFuseMountCmd.ExtraFiles
	osxFuseDaemonPathEnv  = "MOUNT_OSXFUSE_DAEMON_PATH=" // Append program name (os.Args[0])
)

func (volume *volumeStruct) DoMount() (err error) {
	var (
		devOsxFuseIndex               uint64
		devOsxFusePath                string
		osxFuseLoadCmd                *exec.Cmd
		osxFuseLoadCmdCombinedOutput  []byte
		osxFuseMountCmd               *exec.Cmd
		osxFuseMountCmdCombinedOutput []byte
		errAsPathError                *os.PathError
		errAsPathErrorOK              bool
	)

	// Ensure OSXFuse is installed

	_, err = os.Stat(osxFuseLoadPath)
	if nil != err {
		volume.logger.Printf("DoMount() unable to find osxFuseLoadPath (\"%s\"): %v", osxFuseLoadPath, err)
		return
	}
	_, err = os.Stat(osxFuseMountPath)
	if nil != err {
		volume.logger.Printf("DoMount() unable to find osxFuseMountPath (\"%s\"): %v", osxFuseMountPath, err)
		return
	}

	// Try to open one of the devOsxFusePrefix* files (only there when OSXFuse has been loaded)

	devOsxFuseIndex = 0

	for devOsxFuseIndex = 0; ; devOsxFuseIndex++ {
		devOsxFusePath = devOsxFusePrefix + strconv.FormatUint(devOsxFuseIndex, 10)

		volume.devFuseFD, err = syscall.Open(devOsxFusePath, syscall.O_RDWR|syscall.O_CLOEXEC, 0)

		// Handle special case where OSXFuse is not yet loaded

		if (0 == devOsxFuseIndex) && (nil != err) && os.IsNotExist(err) {
			osxFuseLoadCmd = exec.Command(osxFuseLoadPath)
			osxFuseLoadCmd.Dir = "/" // Not sure if this is necessary

			osxFuseLoadCmdCombinedOutput, err = osxFuseLoadCmd.CombinedOutput()
			if nil != err {
				volume.logger.Printf("DoMount() unable to load OSXFuse via osxFuseLoadPath (\"%s\") [%v]: %s", osxFuseLoadPath, err, string(osxFuseLoadCmdCombinedOutput[:]))
				return
			}

			// Now reattempt to open devOsxFusePrefix + "0" before falling into the common logic

			volume.devFuseFD, err = syscall.Open(devOsxFusePath, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		}

		if nil == err {
			// Got one - proceed to mount phase
			break
		}

		if os.IsNotExist(err) {
			volume.logger.Printf("DoMount() unable to find available FUSE device file among devOsxFusePrefix* (%s*)", devOsxFusePrefix)
			return
		}

		errAsPathError, errAsPathErrorOK = err.(*os.PathError)

		if errAsPathErrorOK {
			if syscall.EBUSY == errAsPathError.Err {
				// This one is busy - proceed to the next one
				continue
			}

			volume.logger.Printf("DoMount() open of \"%s\" got unexpeced errAsPathError: %+v", devOsxFusePath, errAsPathError)
			return
		}

		volume.logger.Printf("DoMount() open of \"%s\" got unexpected error: %v", devOsxFusePath, err)
		return
	}

	// Start serving volume.devFuseFD

	volume.devFuseFDReaderWG.Add(1)
	go volume.devFuseFDReader()

	// Finally, launch the Mount Helper (fusermount equivalent)

	osxFuseMountCmd = &exec.Cmd{
		Path: osxFuseMountPath,
		Args: []string{
			"-o iosize=" + strconv.FormatUint(uint64(volume.initOutMaxWrite), 10),
			volume.mountpointDirPath,
		},
		Env:          append(os.Environ(), osxFuseMountCallByEnv, osxFuseMountCommFDEnv, osxFuseDaemonPathEnv+os.Args[0]),
		Dir:          "",
		Stdin:        nil,
		Stdout:       nil, // This will be redirected to osxFuseMountCmdCombinedOutput below
		Stderr:       nil, // This will be redirected to osxFuseMountCmdCombinedOutput below
		ExtraFiles:   []*os.File{os.NewFile(uintptr(volume.devFuseFD), devOsxFusePath)},
		SysProcAttr:  nil,
		Process:      nil,
		ProcessState: nil,
	}

	osxFuseMountCmdCombinedOutput, err = osxFuseMountCmd.CombinedOutput()
	if nil != err {
		volume.logger.Printf("DoMount() unable to mount %s (%v): %s", volume.volumeName, err, string(osxFuseMountCmdCombinedOutput[:]))
		return
	}

	volume.logger.Printf("Volume %s mounted on mountpoint %s", volume.volumeName, volume.mountpointDirPath)

	return
}

func (volume *volumeStruct) DoUnmount() (err error) {
	err = syscall.Unmount(volume.mountpointDirPath, 0)
	if nil != err {
		volume.logger.Printf("DoUnmount() unable to unmount volume %s from mountpoint %s: %v", volume.volumeName, volume.mountpointDirPath, err)
		return
	}

	err = syscall.Close(volume.devFuseFD)
	if nil != err {
		volume.logger.Printf("DoUnmount() unable to close /dev/osxfuse*: %v", err)
		return
	}

	volume.devFuseFDReaderWG.Wait()

	volume.logger.Printf("Volume %s unmounted from mountpoint %s", volume.volumeName, volume.mountpointDirPath)

	err = nil
	return
}
