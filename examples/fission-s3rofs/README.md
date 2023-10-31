# fission/examples/fission-swiftfs

Example fission (FUSE) File System implementing a read-only File System
presentation of an S3 Bucket (with optional Prefix). A clean exit is
triggered by sending the process a SIGHUP, SIGINT, or SIGTERM.

## Configuration

The program is launched with a single argument specifying the path to a JSON-formatted document with the following structure:
```
{
  "MountPoint"     : "/mnt",
  "S3AccessKey"    : "test:tester",
  "S3SecretKey"    : "testing",
  "S3Endpoint"     : "http://swift:8080",
  "S3Region"       : "us-east-1",
  "S3Attempts"     : 5,
  "S3Backoff"      : 60,
  "S3Bucket"       : "<bucket_name>",
  "S3Prefix"       : "<object_name_prefix>",
  "CacheDirPath"   : "/dev/null",
  "FileCacheLines" : 0,
  "RAMCacheLines"  : 1024,
  "CacheLineSize"  : 1048576
}
```
If `S3AccessKey` and/or `S3SecretKey` are specified as empty strings, their values will be fetched from the ENV.

If `FileCacheLines` is specified as zero, `CacheDirPath`'s value is ignored. But if `FileCacheLines` is non-zero, cache lines will spill out of RAM to `CacheLineSize` sized files in the `CacheDirPath` specified directory.

## Notes

* Not only is the presented File System read-only, it is assumed that the underlying Objects within the Container are unchanging.
* Object names should not contain invalid File basename characters (in particular, no slashes that would suggest a hierarchical directory structure).
* An emulation of the DoGetAttr() / DoLookup() behavior is performed whereby the Attr structure to be returned is computed by means of an Object HEAD response. In particular, the Content-Length in that response is used to initialize the Attr.Size value. The resultant Attr is subsequently cached and there is no cache eviction logic.
* Knowing that the Container contents are unchanging, it would be acceptable to load the Attr.Size values at mount time. But to better emulate a system supporting Attr cache eviction, this mount-time optimization is not performed.
