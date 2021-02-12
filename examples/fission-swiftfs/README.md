# fission/examples/fission-swiftfs

Example fission (FUSE) file system implementing (for now) a read-only file system
presentation of an OpenStack Swift Container. A clean exit is triggered by sending
the process a SIGHUP, SIGINT, or SIGTERM.
