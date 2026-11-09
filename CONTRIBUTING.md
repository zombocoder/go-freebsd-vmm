# Contributing to go-freebsd-vmm

Thanks for your interest in improving the library! Bug reports, fixes,
bindings for `libvmmapi` calls that are still missing, tests on hardware we
have not seen, examples and documentation are all welcome.

By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
Security problems should be reported privately, see [SECURITY.md](SECURITY.md).

## Table of Contents

- [Getting Started](#getting-started)
- [Running Tests](#running-tests)
- [Continuous Integration](#continuous-integration)
- [Project rules](#project-rules)
- [The C bridge](#the-c-bridge)
- [Performance](#performance)
- [Tests](#tests)
- [Style](#style)
- [Submitting Changes](#submitting-changes)
- [Where Help Is Wanted](#where-help-is-wanted)

## Getting Started

### Prerequisites

- FreeBSD 14 or 15 on amd64
- Go 1.22+ (`pkg install go`)
- The base system C compiler (cgo), `<vmmapi.h>` and `libvmmapi`
- For the integration levels: root (or `doas`), `vmm(4)` loaded, and a CPU
  with VT-x or AMD-V. Nested virtualization works if the outer hypervisor
  passes it through.

The library does not build anywhere else; on other platforms the build stops
with an explanation. Formatting and `go mod tidy` can still be checked there.

### Setup

```sh
git clone https://github.com/<your-username>/go-freebsd-vmm.git
cd go-freebsd-vmm
make build
make test
```

On a FreeBSD installation built without bhyve, `<vmmapi.h>` is present but
there is no library to link against. Build one from the system sources into
`build/` first (nothing is vendored):

```sh
make libvmmapi
```

## Running Tests

| Command | Runs as | What it covers |
|---|---|---|
| `make test` | any user | Level 1: pure Go, no kernel. Level 2 tests skip themselves. |
| `make race` | any user | The same under the race detector. |
| `make check` | any user | Formatting, `go vet`, the C bridge under `-Werror`, and `make test`. |
| `sudo -E make test` | root | Adds level 2 (`tests/`): creates, configures and destroys real VMs. |
| `sudo -E make privileged` | root | Level 3 (`tests/privileged`): enters the guest, injects interrupts, touches host hardware. |
| `make bench` | root for most | Register and statistics benchmarks. |

With `doas`, which resets the environment, pass `PATH` through:
`doas env PATH=$PATH make test`.

```sh
kldload vmm
sudo -E go test -count=1 -run Stats -v ./tests/vcpu
```

Integration tests only touch VMs they create, named `govmm-test-<pid>-<random>`,
and destroy them through `t.Cleanup`. Each package also sweeps leftovers from
processes that are no longer running. Still, they run as root and load
`vmm(4)`: prefer a test machine or a VM with nested virtualization over a
host that runs production guests.

## Continuous Integration

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on every push to
`main` and every pull request:

- **Lint** (Linux): `gofmt` and `go mod tidy -diff`.
- **FreeBSD 14.5 and 15.1** (QEMU VMs via
  [vmactions/freebsd-vm](https://github.com/vmactions/freebsd-vm)): installs
  Go, checks the C bridge under `-Werror`, runs `go vet`, builds all packages
  and examples, and runs every test level as root, including under the race
  detector. It fails if any `govmm-test-*` VM is left behind.

The FreeBSD VMs only get hardware virtualization if the runner passes it
through. When they do not, `vmm(4)` does not attach and levels 2 and 3 skip
with the reason; the job log says which happened. That makes CI a guarantee
for building, vetting and level 1 on both releases, and not a substitute for
running levels 2 and 3 on real hardware. Say in the pull request where you
ran them.

## Project rules

These are invariants rather than preferences. A change that breaks one of them
needs a much better reason than convenience.

1. **The library knows about `vmm(4)`, not about bhyve configuration.**
   Device models, networking, storage and command-line construction belong to
   a higher layer. If a feature needs a `VMConfig` struct, it does not belong
   here.

2. **No C type crosses the public API.** `import "C"` appears only in
   `internal/vmmapi`. `*C.struct_vmctx`, `vm_paddr_t` and `cpuset_t` stay
   inside it.

3. **`libvmmapi` is the interface.** Do not reach for `ioctl` directly unless
   `libvmmapi` genuinely cannot express something, and document the decision
   where the code is.

4. **Ownership is explicit.** No finalizers, no implicit cleanup. A handle is
   released by the call that says so.

5. **Kernel errors stay kernel errors.** Preserve `errno`. Do not flatten a
   failure into a string, and do not invent a sentinel for an `errno` that
   `syscall` already has.

6. **Architecture differences are compile-time differences.** Put them in
   `*_GOARCH.go` files behind build constraints, not in runtime switches.

7. **No hidden subprocesses.** The library must never run `bhyve`,
   `bhyvectl`, `sysctl` or `pciconf` to make up for a missing API.

8. **The library stays low level.** Policy belongs to the caller.

## The C bridge

`internal/vmmapi/compat.c` exists for three reasons: capturing `errno`
immediately after each call, keeping cgo call shapes regular, and collapsing
loops that would otherwise cost one cgo transition per iteration. It must stay
thin and must not become a second implementation of `libvmmapi`.

Write it like FreeBSD code, not like generated glue: FreeBSD style, tabs,
return values parenthesised, one declaration block at the top of a function.
It has to compile clean under `-Wall -Wextra -Werror`, which `make cwarn`
checks.

Capture `errno` in the statement immediately after the call it belongs to. The
Go runtime makes no promise that it survives anything else.

## Performance

A cgo transition costs far more than a Go call, so an API that turns one kernel
operation into hundreds of them is a bug rather than a style question. Where
`libvmmapi` offers a bulk call, use it: `vm_get_register_set` over a loop of
`vm_get_register`, one statistics call over one per counter, the address space
walk in C over a loop from Go.

`make bench` has the comparisons.

## Tests

Three levels:

- **Level 1**, in the root package: pure Go, no kernel. Name validation,
  error wrapping, version decoding, CPU set arithmetic, topology validation,
  argument checks and the state machine. These run everywhere and are where
  most new tests belong.
- **Level 2**, in `tests/`: a real kernel, root, `vmm(4)`. Use the helpers in
  `tests/harness` so that every VM is named with the sweep prefix and
  destroyed afterwards, including when the test fails.
- **Level 3**, in `tests/privileged`, behind the `vmm_privileged` tag and off
  by default: entering the guest, interrupt injection, the RTC, address
  translation, guest copies, snapshots and passthrough. Run it with
  `sudo -E make privileged`. The guest payload is five bytes of real-mode
  code; keep it that way. A test guest that needs firmware or a device model
  is testing bhyve, not this library.

A test that creates a VM must clean it up through `t.Cleanup`. CI must not
accumulate objects under `/dev/vmm`.

## Style

Go code is `gofmt`-clean and passes `go vet` and `staticcheck`.

Every exported symbol has a doc comment. Say what it does and what the caller
has to know that is not obvious from the signature — which errors mean what,
what has to be set up first, what the kernel will refuse.

The library does not log. Return errors and let the caller decide.

Dependencies are the standard library, cgo and the system `libvmmapi`. No
logging framework, no CLI framework, no configuration or validation library.

## Commits

Explain why, not just what. If a change works around a behaviour of
`libvmmapi` or the kernel, name it: the next person to read the code will
otherwise take the workaround for an accident.

## Submitting Changes

1. Create a branch from `main`.
2. Make the change with tests; update doc comments, README and examples when
   the public API changes, and add an entry to [CHANGELOG.md](CHANGELOG.md)
   under "Unreleased".
3. Run `make check` and, if the change touches anything below the pure-Go
   layer, `sudo -E make test` and `sudo -E make privileged` on FreeBSD.
4. Open a pull request; the template has a short checklist. CI must be green.

Commit messages: `type(scope): summary` (e.g. `fix(memory): include
dev/vmm/vmm_mem.h on FreeBSD 15`), a blank line, then what changed and why.
Common types are `feat`, `fix`, `docs`, `test`, `refactor`, `ci`, `chore`.

Avoid breaking public API changes within 1.x. When a FreeBSD release removes
something, keep the Go API, report `ErrUnsupported`, and add a `Feature` that
answers in advance, as `SetLowMemoryLimit` does on FreeBSD 15.

## Where Help Is Wanted

Most of these need hardware or a kernel configuration the maintainer does not
have, which makes them good first contributions for someone who does:

- **Snapshots** on a kernel built with `options BHYVE_SNAPSHOT`: the save and
  restore primitives exist but have never been seen to succeed.
- **PCI passthrough** with a real device bound to `ppt(4)`: assignment, MMIO
  mapping, MSI and MSI-X.
- **A guest in protected or long mode**: a small payload that enables paging,
  to test address translation and `ReadGuestVirtual` across real page tables.
- **`arm64` and `riscv64`**: bindings written against those architectures'
  headers (VGIC, APLIC), on a host that can run them.
- **Reports from other CPUs and releases**: if the levels 2 and 3 tests pass
  (or fail) on hardware not listed in the README, open an issue with the
  output.

Questions and design discussions are welcome in GitHub issues.
