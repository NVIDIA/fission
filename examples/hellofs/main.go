package main

import (
	"flag"
	"fmt"
)

type globalsStruct struct {
	mountpointDirPath string
}

var globals globalsStruct

func init() {
	var (
		mountPointDirPathPtr *string
	)
	mountPointDirPathPtr = flag.String("m", "", "path to empty directory where mounted hellofs will appear")
	flag.Parse()
	globals.mountpointDirPath = *mountPointDirPathPtr
}

func main() {
	fmt.Printf("globals.mountpointDirPath == \"%s\"\n", globals.mountpointDirPath)
}
