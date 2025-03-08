// Copyright (c) 2015-2025, NVIDIA CORPORATION.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"syscall"

	"github.com/NVIDIA/fission"
)

func (*globalsStruct) DoLookup(inHeader *fission.InHeader, lookupIn *fission.LookupIn) (lookupOut *fission.LookupOut, errno syscall.Errno) {
	if inHeader.NodeID != rootInodeIno {
		errno = syscall.ENOENT
		return
	}

	if !bytes.Equal(helloFileName, lookupIn.Name) {
		errno = syscall.ENOENT
		return
	}

	lookupOut = &fission.LookupOut{
		EntryOut: fission.EntryOut{
			NodeID:         globals.helloInodeAttr.Ino,
			Generation:     0,
			EntryValidSec:  0,
			AttrValidSec:   0,
			EntryValidNSec: 0,
			AttrValidNSec:  0,
			Attr: fission.Attr{
				Ino:       globals.helloInodeAttr.Ino,
				Size:      globals.helloInodeAttr.Size,
				Blocks:    globals.helloInodeAttr.Blocks,
				ATimeSec:  globals.helloInodeAttr.ATimeSec,
				MTimeSec:  globals.helloInodeAttr.MTimeSec,
				CTimeSec:  globals.helloInodeAttr.CTimeSec,
				ATimeNSec: globals.helloInodeAttr.ATimeNSec,
				MTimeNSec: globals.helloInodeAttr.MTimeNSec,
				CTimeNSec: globals.helloInodeAttr.CTimeNSec,
				Mode:      globals.helloInodeAttr.Mode,
				NLink:     globals.helloInodeAttr.NLink,
				UID:       globals.helloInodeAttr.UID,
				GID:       globals.helloInodeAttr.GID,
				RDev:      globals.helloInodeAttr.RDev,
				BlkSize:   globals.helloInodeAttr.BlkSize,
				Padding:   globals.helloInodeAttr.Padding,
			},
		},
	}

	errno = 0
	return
}

func (*globalsStruct) DoForget(_ *fission.InHeader, _ *fission.ForgetIn) {}

func (*globalsStruct) DoGetAttr(inHeader *fission.InHeader, _ *fission.GetAttrIn) (getAttrOut *fission.GetAttrOut, errno syscall.Errno) {
	var (
		inodeAttr *fission.Attr
	)

	switch inHeader.NodeID {
	case rootInodeIno:
		inodeAttr = globals.rootInodeAttr
	case helloInodeIno:
		inodeAttr = globals.helloInodeAttr
	default:
		errno = syscall.ENOENT
		return
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

	errno = 0
	return
}

func (*globalsStruct) DoSetAttr(_ *fission.InHeader, _ *fission.SetAttrIn) (setAttrOut *fission.SetAttrOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoReadLink(_ *fission.InHeader) (readLinkOut *fission.ReadLinkOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoSymLink(_ *fission.InHeader, _ *fission.SymLinkIn) (symLinkOut *fission.SymLinkOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoMkNod(_ *fission.InHeader, _ *fission.MkNodIn) (mkNodOut *fission.MkNodOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoMkDir(_ *fission.InHeader, _ *fission.MkDirIn) (mkDirOut *fission.MkDirOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoUnlink(_ *fission.InHeader, _ *fission.UnlinkIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoRmDir(_ *fission.InHeader, _ *fission.RmDirIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoRename(_ *fission.InHeader, _ *fission.RenameIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoLink(_ *fission.InHeader, _ *fission.LinkIn) (linkOut *fission.LinkOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoOpen(inHeader *fission.InHeader, _ *fission.OpenIn) (openOut *fission.OpenOut, errno syscall.Errno) {
	if inHeader.NodeID == rootInodeIno {
		errno = syscall.EINVAL
		return
	}
	if inHeader.NodeID != helloInodeIno {
		errno = syscall.ENOENT
		return
	}

	openOut = &fission.OpenOut{
		FH:        0,
		OpenFlags: fission.FOpenResponseDirectIO,
		Padding:   0,
	}

	errno = 0
	return
}

func (*globalsStruct) DoRead(inHeader *fission.InHeader, readIn *fission.ReadIn) (readOut *fission.ReadOut, errno syscall.Errno) {
	if inHeader.NodeID != helloInodeIno {
		errno = syscall.ENOENT
		return
	}

	var (
		adjustedOffset uint64
		adjustedSize   uint32
	)

	adjustedOffset = uint64(len(helloInodeFileData))
	if readIn.Offset < adjustedOffset {
		adjustedOffset = readIn.Offset
	}

	adjustedSize = readIn.Size
	if (adjustedOffset + uint64(adjustedSize)) > uint64(len(helloInodeFileData)) {
		adjustedSize = uint32(len(helloInodeFileData)) - uint32(adjustedOffset)
	}

	readOut = &fission.ReadOut{
		Data: cloneByteSlice(helloInodeFileData[adjustedOffset:(adjustedOffset + uint64(adjustedSize))]),
	}

	errno = 0
	return
}

func (*globalsStruct) DoWrite(_ *fission.InHeader, _ *fission.WriteIn) (writeOut *fission.WriteOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
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

func (*globalsStruct) DoRelease(_ *fission.InHeader, _ *fission.ReleaseIn) (errno syscall.Errno) {
	errno = 0
	return
}

func (*globalsStruct) DoFSync(_ *fission.InHeader, _ *fission.FSyncIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoSetXAttr(_ *fission.InHeader, _ *fission.SetXAttrIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoGetXAttr(_ *fission.InHeader, _ *fission.GetXAttrIn) (getXAttrOut *fission.GetXAttrOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoListXAttr(_ *fission.InHeader, _ *fission.ListXAttrIn) (listXAttrOut *fission.ListXAttrOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoRemoveXAttr(_ *fission.InHeader, _ *fission.RemoveXAttrIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoFlush(_ *fission.InHeader, _ *fission.FlushIn) (errno syscall.Errno) {
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
		Unused:               [7]uint32{0, 0, 0, 0, 0, 0, 0},
	}

	errno = 0
	return
}

func (*globalsStruct) DoOpenDir(inHeader *fission.InHeader, _ *fission.OpenDirIn) (openDirOut *fission.OpenDirOut, errno syscall.Errno) {
	if inHeader.NodeID == helloInodeIno {
		errno = syscall.EINVAL
		return
	}
	if inHeader.NodeID != rootInodeIno {
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

func (*globalsStruct) DoReadDir(inHeader *fission.InHeader, readDirIn *fission.ReadDirIn) (readDirOut *fission.ReadDirOut, errno syscall.Errno) {
	var (
		dirEntIndex          int
		dirEntNameLenAligned uint32
		dirEntSize           uint32
		totalSize            uint32
	)

	if inHeader.NodeID == helloInodeIno {
		errno = syscall.EINVAL
		return
	}
	if inHeader.NodeID != rootInodeIno {
		errno = syscall.ENOENT
		return
	}

	readDirOut = &fission.ReadDirOut{
		DirEnt: globals.dirEnt[readDirIn.Offset:],
	}

	totalSize = 0

	for dirEntIndex = range readDirOut.DirEnt {
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
	if inHeader.NodeID != rootInodeIno {
		errno = syscall.EINVAL
		return
	}

	errno = 0
	return
}

func (*globalsStruct) DoFSyncDir(_ *fission.InHeader, _ *fission.FSyncDirIn) (errno syscall.Errno) {
	errno = syscall.ENOSYS
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
	if (accessIn.Mask & accessWOK) != 0 {
		errno = syscall.EACCES
	} else {
		switch inHeader.NodeID {
		case rootInodeIno:
			errno = 0
		case helloInodeIno:
			if (accessIn.Mask & accessXOK) != 0 {
				errno = syscall.EACCES
			} else {
				errno = 0
			}
		default:
			errno = syscall.ENOENT
		}
	}

	return
}

func (*globalsStruct) DoCreate(_ *fission.InHeader, _ *fission.CreateIn) (createOut *fission.CreateOut, errno syscall.Errno) {
	errno = syscall.ENOSYS
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
		dirEntPlusIndex      int
		dirEntNameLenAligned uint32
		dirEntSize           uint32
		totalSize            uint32
	)

	if inHeader.NodeID == helloInodeIno {
		errno = syscall.EINVAL
		return
	}
	if inHeader.NodeID != rootInodeIno {
		errno = syscall.ENOENT
		return
	}

	readDirPlusOut = &fission.ReadDirPlusOut{
		DirEntPlus: globals.dirEntPlus[readDirPlusIn.Offset:],
	}

	totalSize = 0

	for dirEntPlusIndex = range readDirPlusOut.DirEntPlus {
		dirEntNameLenAligned = (uint32(len(readDirPlusOut.DirEntPlus[dirEntPlusIndex].Name)) + (fission.DirEntAlignment - 1)) & ^uint32(fission.DirEntAlignment-1)
		dirEntSize = fission.DirEntPlusFixedPortionSize + dirEntNameLenAligned

		if (totalSize + dirEntSize) > readDirPlusIn.Size {
			// Truncate readDirOut here and return

			readDirPlusOut.DirEntPlus = readDirPlusOut.DirEntPlus[:dirEntPlusIndex]

			errno = 0
			return
		}

		totalSize += dirEntSize
	}

	errno = 0
	return
}

func (*globalsStruct) DoRename2(_ *fission.InHeader, _ *fission.Rename2In) (errno syscall.Errno) {
	errno = syscall.ENOSYS
	return
}

func (*globalsStruct) DoLSeek(_ *fission.InHeader, _ *fission.LSeekIn) (lSeekOut *fission.LSeekOut, errno syscall.Errno) {
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
