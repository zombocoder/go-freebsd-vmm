# Security Policy

## Supported versions

Security fixes are made on the `main` branch and released as a new patch or
minor version of the latest major release (currently 1.x).

## Reporting a vulnerability

Please **do not open a public issue** for security problems.

Report them privately through GitHub:
[Report a vulnerability](https://github.com/zombocoder/go-freebsd-vmm/security/advisories/new)
(Security tab → "Report a vulnerability").

Include what you can of:

- the affected function and the commit you tested;
- the FreeBSD version (`freebsd-version -ku`) and CPU (`sysctl -n hw.model`);
- steps or a minimal program that reproduces the problem, including any guest
  code it needs;
- the impact you expect (host memory corruption, a guest influencing the host
  process, acting on the wrong VM or vCPU, ...).

You should get a first response within a week. Once a fix is available, the
advisory is published with credit to the reporter unless you prefer otherwise.

## Scope

This library is used by privileged processes that control virtual machines,
and much of what it returns is written by the guest. Of particular interest:

- memory-safety bugs in the cgo layer (`internal/vmmapi`), such as
  out-of-bounds reads or writes of exit payloads, statistics or register
  buffers, or Go pointers retained by C;
- guest-controlled values from a VM exit or from guest memory being trusted
  as lengths, indices or offsets on the host side, for example instruction
  bytes and lengths in `ExitInstEmul`;
- use of a `MemoryRegion` or vCPU handle after its VM was closed or
  destroyed;
- input that reaches `libvmmapi` without the validation the API documents
  (VM names with path separators or embedded NUL, vCPU ids, integer ranges
  across the cgo boundary);
- operations that act on a different VM, vCPU or device than requested.

Bugs in `vmm(4)`, `libvmmapi` or `bhyve` themselves should be reported to the
[FreeBSD security team](https://www.freebsd.org/security/).
