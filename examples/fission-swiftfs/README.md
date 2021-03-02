# fission/examples/fission-swiftfs

Example fission (FUSE) File System implementing (for now) a read-only File System
presentation of an OpenStack Swift Container. A clean exit is triggered by sending
the process a SIGHUP, SIGINT, or SIGTERM.

## Configuration

The program is launched with a single argument specifying the path to a JSON-formatted document with the following structure:
```
{
  "MountPoint"              : "MountPoint",
  "ContainerURL"            : "http://127.0.0.1:8080/v1/AUTH_test/C",
  "AuthURL"                 : "http://127.0.0.1:8080/auth/v1.0",
  "AuthUser"                : "test:tester",
  "AuthKey"                 : "testing",
  "AuthToken"               : "",
  "SwiftTimeout"            : "10m",
  "SwiftConnectionPoolSize" : 100,
  "NumCacheLines"           : 1024,
  "CacheLineSize"           : 1048576
}
```
If Auth{URL|User|Key} are provided (and AuthToken is not), normal OpenStack Swift Authorization will be performed to compute the AuthToken.

Alternatively, you may supply a separately obtained AuthToken (in which case Auth{URL|User|Key} should not be provided).

Finally, if none of Auth{URL|User|Key|Token} are provided, the program will not supply an AuthToken in requests issued to ContainerURL. Thus, not `auth` middleware should be in the Swift Proxy pipeline.

## Notes

* Not only is the presented File System read-only, it is assumed that the underlying Objects within the Container are unchanging.
* Object names should not contain invalid File basename characters (in particular, no slashes that would suggest a hierarchical directory structure).
* Only a single Container GET is performed thus the presented directory will only contain the Files (Objects) returned in that single Container GET response.
* An emulation of the DoGetAttr() / DoLookup() behavior is performed whereby the Attr structure to be returned is computed by means of an Object HEAD response. In particular, the Content-Length in that response is used to initialize the Attr.Size value. The resultant Attr is subsequently cached and there is no cache eviction logic.
* Knowing that the Container contents are unchanging, it would be acceptable to load the Attr.Size values at mount time. But to better emulate a system supporting Attr cache eviction, this mount-time optimization is not performed.
