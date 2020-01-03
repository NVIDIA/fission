package fission

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const (
	devOsxFusePrefix     = "/dev/osxfuse" // suffix is a non-negative decimal number starting with 0, 1, ...
	osxFuseLoadPath      = "/Library/Filesystems/osxfuse.fs/Contents/Resources/load_osxfuse"
	osxFuseMountPath     = "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse"
	osxFuseDaemonPathEnv = "MOUNT_OSXFUSE_DAEMON_PATH"
)

func (volume *volumeStruct) DoMount() (err error) {
	var (
		devOsxFuseIndex              uint64
		devOsxFusePath               string
		osxFuseLoadCmd               *exec.Cmd
		osxFuseLoadCmdCombinedOutput []byte
		errAsPathError               *os.PathError
		errAsPathErrorOK             bool
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
			fmt.Println("UNDO: Hey... time to load OSXFuse :-)")
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

	err = fmt.Errorf("TODO: Continue DoMount() for devOsxFusePath: %s", devOsxFusePath)
	return
}

func (volume *volumeStruct) DoUnmount() (err error) {
	err = fmt.Errorf("TODO: DoUnmount()")
	return
}

/* - from ./mount_linux.go
import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const (
	recvmsgFlags   int = 0
	recvmsgOOBSize     = 32
	recvmsgPSize       = 4
)

func (volume *volumeStruct) DoMount() (err error) {
	var (
		allowOtherOption         string
		fusermountChildWriteFile *os.File
		fusermountParentReadFile *os.File
		fusermountProgramPath    string
		fusermountSocketPair     [2]int
		gid                      int
		gidMountOption           string
		mountCmd                 *exec.Cmd
		mountCmdCombinedOutput   bytes.Buffer
		mountOptions             string
		childOpenFDs             []int
		recvmsgOOB               [recvmsgOOBSize]byte
		recvmsgOOBN              int
		recvmsgP                 [recvmsgPSize]byte
		rootMode                 uint32
		rootModeMountOption      string
		socketControlMessages    []syscall.SocketControlMessage
		uid                      int
		uidMountOption           string
		unmountCmd               *exec.Cmd
	)

	fusermountProgramPath, err = exec.LookPath("fusermount")
	if nil != err {
		volume.logger.Printf("DoMount() unable to find program `fusermount`: %v", err)
		return
	}

	unmountCmd = exec.Command(fusermountProgramPath, "-u", volume.mountpointDirPath)
	_, _ = unmountCmd.CombinedOutput()

	fusermountSocketPair, err = syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
	if nil != err {
		volume.logger.Printf("DoMount() unable to create socketpairFDs: %v", err)
		return
	}

	fusermountChildWriteFile = os.NewFile(uintptr(fusermountSocketPair[0]), "fusermountChildWriteFile")
	fusermountParentReadFile = os.NewFile(uintptr(fusermountSocketPair[1]), "fusermountParentReadFile")

	defer func() {
		err = fusermountChildWriteFile.Close()
		if nil != err {
			volume.logger.Printf("DoMount() unable to close fusermountChildWriteFile: %v", err)
		}
		err = fusermountParentReadFile.Close()
		if nil != err {
			volume.logger.Printf("DoMount() unable to close fusermountParentReadFile: %v", err)
		}
	}()

	rootMode = syscall.S_IFDIR
	rootModeMountOption = fmt.Sprintf("rootmode=%o", rootMode)

	uid = syscall.Geteuid()
	gid = syscall.Getegid()

	uidMountOption = fmt.Sprintf("user_id=%d", uid)
	gidMountOption = fmt.Sprintf("group_id=%d", gid)
	allowOtherOption = "allow_other"

	mountOptions = rootModeMountOption +
		"," + uidMountOption +
		"," + gidMountOption +
		"," + allowOtherOption

	mountCmd = &exec.Cmd{
		Path: fusermountProgramPath,
		Args: []string{
			"-o " + mountOptions,
			volume.mountpointDirPath,
		},
		Env:          append(os.Environ(), "_FUSE_COMMFD=3"),
		Dir:          "",
		Stdin:        nil,
		Stdout:       &mountCmdCombinedOutput,
		Stderr:       &mountCmdCombinedOutput,
		ExtraFiles:   []*os.File{fusermountChildWriteFile},
		SysProcAttr:  nil,
		Process:      nil,
		ProcessState: nil,
	}

	err = mountCmd.Start()
	if nil != err {
		volume.logger.Printf("DoMount() unable to mountCmd.Start(): %v", err)
		return
	}

	_, recvmsgOOBN, _, _, err = syscall.Recvmsg(
		int(fusermountParentReadFile.Fd()),
		recvmsgP[:],
		recvmsgOOB[:],
		recvmsgFlags)
	if nil != err {
		volume.logger.Printf("Volume %s DoMount() got error from fusermount: %v", volume.volumeName, err)
		return
	}

	socketControlMessages, err = syscall.ParseSocketControlMessage(recvmsgOOB[:recvmsgOOBN])
	if nil != err {
		volume.logger.Printf("Volume %s DoMount() unable to syscall.ParseSocketControlMessage(): %v", volume.volumeName, err)
		return
	}
	if 1 != len(socketControlMessages) {
		volume.logger.Printf("Volume %s DoMount() got unexpected len(socketControlMessages): %v", volume.volumeName, len(socketControlMessages))
		return
	}

	childOpenFDs, err = syscall.ParseUnixRights(&socketControlMessages[0])
	if nil != err {
		volume.logger.Printf("Volume %s DoMount() unable to syscall.ParseUnixRights(): %v", volume.volumeName, err)
		return
	}
	if 1 != len(childOpenFDs) {
		volume.logger.Printf("Volume %s DoMount() got unexpected len(childOpenFDs): %v", volume.volumeName, len(childOpenFDs))
		return
	}

	volume.devFuseFD = childOpenFDs[0]

	volume.devFuseFDReaderWG.Add(1)
	go volume.devFuseFDReader()

	err = mountCmd.Wait()
	if nil != err {
		volume.logger.Printf("Volume %s DoMount() got error (%v) from fusermount: %s", volume.volumeName, err, mountCmdCombinedOutput.String())
		return
	}

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
	if nil != err {
		volume.logger.Printf("DoUnmount() unable to find program `fusermount`: %v", err)
		return
	}

	unmountCmd = exec.Command(fusermountProgramPath, "-u", volume.mountpointDirPath)
	unmountCmdCombinedOutput, err = unmountCmd.CombinedOutput()
	if nil != err {
		volume.logger.Printf("DoUnmount() unable to unmount %s (%v): %s", volume.volumeName, err, string(unmountCmdCombinedOutput[:]))
		return
	}

	err = syscall.Close(volume.devFuseFD)
	if nil != err {
		volume.logger.Printf("DoUnmount() unable to close /dev/fuse: %v", err)
		return
	}

	volume.devFuseFDReaderWG.Wait()

	volume.logger.Printf("Volume %s unmounted from mountpoint %s", volume.volumeName, volume.mountpointDirPath)

	err = nil
	return
}
*/

/* - from ../../jacobsa/fuse/mount_darwin.go
import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/jacobsa/fuse/internal/buffer"
)

var errNoAvail = errors.New("no available fuse devices")
var errNotLoaded = errors.New("osxfuse is not loaded")

// errOSXFUSENotFound is returned from Mount when the OSXFUSE installation is
// not detected. Make sure OSXFUSE is installed.
var errOSXFUSENotFound = errors.New("cannot locate OSXFUSE")

// osxfuseInstallation describes the paths used by an installed OSXFUSE
// version.
type osxfuseInstallation struct {
	// Prefix for the device file. At mount time, an incrementing number is
	// suffixed until a free FUSE device is found.
	DevicePrefix string

	// Path of the load helper, used to load the kernel extension if no device
	// files are found.
	Load string

	// Path of the mount helper, used for the actual mount operation.
	Mount string

	// Environment variable used to pass the path to the executable calling the
	// mount helper.
	DaemonVar string
}

var (
	osxfuseInstallations = []osxfuseInstallation{
		// v3
		{
			DevicePrefix: "/dev/osxfuse",
			Load:         "/Library/Filesystems/osxfuse.fs/Contents/Resources/load_osxfuse",
			Mount:        "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse",
			DaemonVar:    "MOUNT_OSXFUSE_DAEMON_PATH",
		},

		// v2
		{
			DevicePrefix: "/dev/osxfuse",
			Load:         "/Library/Filesystems/osxfusefs.fs/Support/load_osxfusefs",
			Mount:        "/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs",
			DaemonVar:    "MOUNT_FUSEFS_DAEMON_PATH",
		},
	}
)

func loadOSXFUSE(bin string) error {
	cmd := exec.Command(bin)
	cmd.Dir = "/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func openOSXFUSEDev(devPrefix string) (dev *os.File, err error) {
	// Try each device name.
	for i := uint64(0); ; i++ {
		path := devPrefix + strconv.FormatUint(i, 10)
		dev, err = os.OpenFile(path, os.O_RDWR, 0000)
		if os.IsNotExist(err) {
			if i == 0 {
				// Not even the first device was found. Fuse must not be loaded.
				err = errNotLoaded
				return
			}

			// Otherwise we've run out of kernel-provided devices
			err = errNoAvail
			return
		}

		if err2, ok := err.(*os.PathError); ok && err2.Err == syscall.EBUSY {
			// This device is in use; try the next one.
			continue
		}

		return
	}
}

func callMount(
	bin string,
	daemonVar string,
	dir string,
	cfg *MountConfig,
	dev *os.File,
	ready chan<- error) (err error) {

	// The mount helper doesn't understand any escaping.
	for k, v := range cfg.toMap() {
		if strings.Contains(k, ",") || strings.Contains(v, ",") {
			return fmt.Errorf(
				"mount options cannot contain commas on darwin: %q=%q",
				k,
				v)
		}
	}

	// Call the mount helper, passing in the device file and saving output into a
	// buffer.
	cmd := exec.Command(
		bin,
		"-o", cfg.toOptionsString(),
		// Tell osxfuse-kext how large our buffer is. It must split
		// writes larger than this into multiple writes.
		//
		// OSXFUSE seems to ignore InitResponse.MaxWrite, and uses
		// this instead.
		"-o", "iosize="+strconv.FormatUint(buffer.MaxWriteSize, 10),
		// refers to fd passed in cmd.ExtraFiles
		"3",
		dir,
	)
	cmd.ExtraFiles = []*os.File{dev}
	cmd.Env = os.Environ()
	// OSXFUSE <3.3.0
	cmd.Env = append(cmd.Env, "MOUNT_FUSEFS_CALL_BY_LIB=")
	// OSXFUSE >=3.3.0
	cmd.Env = append(cmd.Env, "MOUNT_OSXFUSE_CALL_BY_LIB=")

	daemon := os.Args[0]
	if daemonVar != "" {
		cmd.Env = append(cmd.Env, daemonVar+"="+daemon)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err = cmd.Start()
	if err != nil {
		return
	}

	// In the background, wait for the command to complete.
	go func() {
		err := cmd.Wait()
		if err != nil {
			if buf.Len() > 0 {
				output := buf.Bytes()
				output = bytes.TrimRight(output, "\n")
				err = fmt.Errorf("%v: %s", err, output)
			}
		}

		ready <- err
	}()

	return
}

// Begin the process of mounting at the given directory, returning a connection
// to the kernel. Mounting continues in the background, and is complete when an
// error is written to the supplied channel. The file system may need to
// service the connection in order for mounting to complete.
func mount(
	dir string,
	cfg *MountConfig,
	ready chan<- error) (dev *os.File, err error) {
	// Find the version of osxfuse installed on this machine.
	for _, loc := range osxfuseInstallations {
		if _, err := os.Stat(loc.Mount); os.IsNotExist(err) {
			// try the other locations
			continue
		}

		// Open the device.
		dev, err = openOSXFUSEDev(loc.DevicePrefix)

		// Special case: we may need to explicitly load osxfuse. Load it, then
		// try again.
		if err == errNotLoaded {
			err = loadOSXFUSE(loc.Load)
			if err != nil {
				err = fmt.Errorf("loadOSXFUSE: %v", err)
				return
			}

			dev, err = openOSXFUSEDev(loc.DevicePrefix)
		}

		// Propagate errors.
		if err != nil {
			err = fmt.Errorf("openOSXFUSEDev: %v", err)
			return
		}

		// Call the mount binary with the device.
		err = callMount(loc.Mount, loc.DaemonVar, dir, cfg, dev, ready)
		if err != nil {
			dev.Close()
			err = fmt.Errorf("callMount: %v", err)
			return
		}

		return
	}

	err = errOSXFUSENotFound
	return
}
*/
