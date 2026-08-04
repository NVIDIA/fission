# Security Policy: fission

## Reporting a Vulnerability

If you discover a potential security vulnerability, please **do not open a public
GitHub issue, discussion, or pull request.**

- **Web (preferred):** [NVIDIA Vulnerability Disclosure Program](https://www.nvidia.com/en-us/security/)
- **E-mail:** [psirt@nvidia.com](mailto:psirt@nvidia.com)
  - For secure communication, use the [NVIDIA public PGP key](https://www.nvidia.com/en-us/security/pgp-key).
- **GitHub:** Use this repository's **Security** tab and select **Report a vulnerability**.

Please include:

- Project name (`fission`) and the affected branch, commit, or module version
- Affected surface (core `Volume`/`Callbacks` API, mount path, or an example such as
  `examples/fission-swiftfs`)
- Host kernel / FUSE availability and privilege level when relevant
- Vulnerability type, reproduction steps, proof-of-concept if available, and impact assessment

NVIDIA's Product Security Incident Response Team (PSIRT) will acknowledge the report,
validate severity, coordinate remediation, and publish a security bulletin when
appropriate. See [PSIRT policies](https://www.nvidia.com/en-us/security/psirt-policies/).

## Supported Versions

`fission` is developed on the `development` branch. Security fixes land there
unless a release branch is explicitly announced.

| Version or branch | Supported |
| --- | --- |
| `development` | Yes |
| Older tags / branches | No, unless explicitly stated |

## Security Architecture & Context

`fission` is a Go library for implementing multi-threaded low-level FUSE file
systems. Callers provision a `Volume`, implement the `Callbacks` interface, and
drive `DoMount` / `DoUnmount`. The library talks to the kernel through
`/dev/fuse` and manages worker pools around fuse upcalls. Example programs (for
example `examples/fission-swiftfs`) demonstrate building a filesystem on top of
remote object storage; those examples are samples, not a production service
shipped by this repository.

This software operates at the **library** level. Its primary security
responsibilities are correct fuse protocol framing with the kernel and clear
delegation of filesystem semantics—and any remote credentials—to the caller's
callback implementation.

**Repository Exposure Classification:** Public.
Basis: origin remote is the publicly accessible `NVIDIA/fission` GitHub repository.

**Service Exposure Classification:** External / Regulated (high confidence).
Basis: externally distributed open-source Go FUSE library under the NVIDIA GitHub
organization; mounts typically require privileged access to `/dev/fuse`.

Key security boundaries:

- Mounting requires access to `/dev/fuse` and typically elevated privileges; the
  host mount namespace is an OS trust boundary.
- Fuse request/response parsing and buffer pooling live in the library; semantic
  authorization lives in `Callbacks` implementors.
- Examples may use HTTP clients and storage credentials; those secrets are
  outside the core library's control.
- Dev containers that add `SYS_ADMIN` and `/dev/fuse` are for local development.

### Threat Model

1. **Fuse protocol / buffer handling defects:** Bugs parsing or serializing fuse
   messages, or mismanaging buffer pools/workers, cause memory corruption,
   panics, or incorrect replies to the kernel.
2. **Callback-driven host impact:** A buggy or malicious `Callbacks`
   implementation combined with a privileged mount can expose, modify, or deny
   host filesystem data beyond the intended mount.
3. **Mount / device lifecycle errors:** Failures in `DoMount`/`DoUnmount`,
   socketpair FD passing, or worker shutdown leave elevated state or leaked
   `/dev/fuse` resources.
4. **Example object-store credential misuse:** Sample filesystems that carry
   access keys/tokens mishandle secrets (logs, overly broad mounts, or weak TLS
   settings) when copied into real deployments.
5. **Confused deputy via fuse upcalls:** Unexpected fuse operation sequences
   (lookup/rename/open races) are mishandled by the library or example and yield
   inconsistent inode/file views.

### Critical Security Assumptions

- Operators who mount a `fission` volume understand the privilege implications of
  FUSE and isolate untrusted callback code appropriately.
- `Callbacks` implementors enforce their own authentication, authorization, and
  path safety for the filesystem they expose.
- Example programs and docker-compose stacks are development aids, not hardened
  production deployments.
- The Linux FUSE subsystem and Go runtime are trusted substrates.
- `fission` itself is not a multi-tenant authorization service.

## Out of Scope

- The need for privileged access to `/dev/fuse` or `SYS_ADMIN` in development
  containers.
- Security bugs solely in application filesystems built on `fission` unless the
  library's protocol or mount handling is at fault.
- Kernel FUSE vulnerabilities unless `fission` introduces a project-specific
  trigger or unsafe handling pattern.
- Using example credentials or published compose defaults against untrusted
  networks (operational misconfiguration).
