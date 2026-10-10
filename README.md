# go-freebsd-vmm

Go bindings to FreeBSD's virtual machine monitor, `vmm(4)`, through the base
system's `libvmmapi`.

This is a systems binding library, not a virtualization runtime. It exposes
what the kernel exposes -- virtual machines, virtual CPUs, guest memory,
registers, CPU topology, capabilities, statistics, guest execution and VM
exits, interrupt injection, PCI passthrough -- and nothing about how a
virtual machine ought to be configured. Nothing here runs `bhyve`,
`bhyvectl` or `bhyveload`, and nothing here shells out at all.

```go
vm, err := vmm.Open("vm01")
if err != nil {
	return err
}
defer vm.Close()

cpu, err := vm.VCPU(0)
if err != nil {
	return err
}
defer cpu.Close()

rip, err := cpu.Register(vmm.RegRIP)
if err != nil {
	return err
}
fmt.Printf("RIP: %#x\n", rip)
```

## Architecture

```
            application
                 |
                 v
          go-freebsd-vmm        Go API, ownership, errors
                 |
                 v
            libvmmapi           base system C library
                 |
                 v
              vmm(4)            /dev/vmm/<name>
                 |
                 v
        VT-x / AMD-V
```

`import "C"` appears only in `internal/vmmapi`, and no C type crosses out of
it. The public API is ordinary Go: `*VM`, `*VCPU`, `Register`, `CPUSet`.

The C side is a thin layer in `internal/vmmapi/compat.c`. It exists to capture
`errno` immediately after each call, to keep cgo call shapes regular, and to
collapse loops that would otherwise cost one cgo transition per iteration. It
is not a second implementation of `libvmmapi`.

## Requirements

- FreeBSD 14 or newer, amd64
- `libvmmapi` and `<vmmapi.h>` from the base system
- Go 1.22 or newer, with cgo enabled
- hardware virtualization, and privilege, to do anything with a real VM

Other platforms fail to build with an explanation rather than offering an API
that cannot work.

## Installation

```sh
go get github.com/zombocoder/go-freebsd-vmm
```

Most FreeBSD installations ship `libvmmapi` and need nothing further.

Some are built without bhyve, and then carry `<vmmapi.h>` but no library to
link against. For development on such a host, build one from the system
sources into `build/`:

```sh
make libvmmapi
make test
```

Nothing from FreeBSD is vendored into this repository; `build/` is ignored by
git.

## Examples

Seven runnable programs live under `examples/`:

| Example | What it shows |
| --- | --- |
| `create` | the VM lifecycle, from create to destroy |
| `inspect` | host and library facts, then one VM's state |
| `topology` | setting and reading CPU topology |
| `registers` | bulk register reads and segment descriptors |
| `memory` | allocating guest memory and walking the address space |
| `stats` | per-vCPU statistics counters |
| `run` | loading a tiny real-mode payload and executing it |

```sh
go run ./examples/inspect vm01
```

## Supported FreeBSD versions

| Release | `VMMAPI_VERSION` | `APIVersion()` | Status |
| --- | --- | --- | --- |
| 14-STABLE | `0200` | 2.0 | supported, developed against |
| 15 | `0300` | 3.0 | expected to work |
| CURRENT | varies | varies | best effort |

The bridge calls only the symbol set that `VMMAPI_VERSION` 2.0 defines, which
is the full post-`struct vcpu` API and is present unchanged in later releases.
There are no version conditionals in the C layer, so one build serves all
three.

`VMMAPI_VERSION` is written as a leading-zero literal, which C reads as octal:
`0200` is 128 and `0300` is 192. `APIVersion()` therefore decodes the digits of
the macro rather than doing arithmetic on its value.

Where releases differ in what they can do, ask rather than compare versions:

```go
if vmm.Supports(vmm.FeatureSparseMemory) {
	// ...
}
```

## Error handling

Failures from the kernel keep their `errno`:

```go
if _, err := vmm.Open("db01"); err != nil {
	// vmm: vm_open "db01": no such file or directory
	if errors.Is(err, syscall.ENOENT) {
		// the VM does not exist
	}
}
```

`*Error` carries the operation, the VM, and the vCPU where one is involved.
Misuse of the API, as distinct from a kernel failure, wraps `ErrClosed`,
`ErrInvalidArgument` or `ErrUnsupported`. There is no sentinel per `errno`
value, because `syscall.Errno` already compares correctly with `errors.Is`.

Nothing panics because of a kernel error, an invalid VM state, a failed ioctl
or a missing device.

## Executing a guest

`VCPU.Run` enters the guest and returns the exit that brought the vCPU back,
decoded into typed values. `VCPU.RunLoop` does that repeatedly on a pinned
host thread:

```go
err := cpu.RunLoop(func(exit vmm.Exit) error {
	if exit.Code.Spurious() {
		return nil
	}
	switch exit.Code {
	case vmm.ExitInOut:
		servicePort(exit.IO)
	case vmm.ExitHLT:
		return vmm.ErrStopRunLoop
	}
	return nil
})
```

Exactly one payload pointer on an `Exit` is non-nil, the one matching `Code`;
an exit that carries no detail leaves them all nil. A reason a future FreeBSD
adds still decodes, keeping its numeric value, so an unknown exit is something
to handle rather than something that breaks.

Two things catch people out. The kernel leaves the guest whenever the calling
thread has a signal pending, and the Go runtime signals its threads routinely,
so `ExitBogus` arrives far more often here than under bhyve -- that is what
`ExitCode.Spurious` is for. And `Run` cannot be cancelled, which is why it
takes no `context.Context`: a vCPU comes back when the guest exits, when the
VM or the vCPU is suspended from elsewhere, or on a signal.

## Resource ownership

Ownership is explicit; there are no finalizers.

A `VM` owns one `libvmmapi` context, and the vCPU handles opened from it belong
to that VM. `Close` releases the context and keeps the VM in the kernel;
`Destroy` releases it and removes the VM. Both also release every vCPU handle,
because `libvmmapi` frees the context those handles point into.

Whichever of the two runs first wins and the other does nothing, so this is
correct:

```go
vm, err := vmm.CreateAndOpen(name)
if err != nil {
	return err
}
defer vm.Close()

// ...

return vm.Destroy()
```

`VM` and `VCPU` methods are safe to call concurrently. Work on independent
vCPUs does not serialise. Statistics are the exception: `libvmmapi` keeps them
in static storage, so `VCPU.Stats` is serialised across the process.

## Security considerations

A guest is untrusted input. Anything read out of guest memory can change
between one instruction and the next, and none of it should be used as a slice
index, an allocation size or a pointer offset without checking it first.

`MemoryRegion.Bytes` aliases guest physical memory. Writing to it changes what
the guest sees, immediately and with no synchronisation against the vCPUs that
may be reading the same addresses.

Every field of an `Exit` is guest-controlled, including the instruction length
and the instruction bytes of `ExitInstEmul`. The decoder clamps the one length
it uses as a slice bound; everything else is handed over as reported and must
be checked before it is trusted.

Strings bound for C are checked for embedded NUL, which would otherwise be
silently truncated. VM names are additionally refused if they contain a path
separator, since `libvmmapi` builds the device path as `/dev/vmm/` plus the
name.

Integer conversions across the cgo boundary are range-checked rather than
assumed to fit.

## API stability

Semantic versioning. The API may change before `v1.0.0`; after it, a breaking
Go API change takes a major version.

What cannot be promised is that every operation exists on every FreeBSD
release, past or future. That is what `Supports` is for.

## Testing

Three levels:

```sh
make test     # level 1: pure Go, no kernel needed
make race     # the same, under the race detector
make lint     # go vet, staticcheck, and the C bridge under -Werror
```

Level 2 lives in `tests/` and drives a real kernel. It needs root and
`vmm(4)`, and skips with the reason when either is missing:

```sh
kldload vmm
sudo -E make test
```

Every test destroys the VMs it creates, including when it fails, and each
package sweeps leftover `/dev/vmm/govmm-test-*` objects before and after the
run.

Level 3 enters the guest and touches host hardware, so it is excluded from
an ordinary run:

```sh
sudo -E make privileged
```

It executes a five-byte real-mode payload and checks the exits it produces,
which is what makes `VM_RUN` and exit decoding testable without a guest
operating system. A second payload spins forever, so that a blocked `Run`
returning on `Suspend` can be observed.

All three levels are green on FreeBSD 14.3 amd64: 114 pure-Go tests, 43
integration tests, 20 privileged tests. The snapshot cases skip, because the
development kernel was built without `BHYVE_SNAPSHOT`.

## Performance

A cgo transition costs far more than a Go call, so an API that turns one
kernel operation into many of them is a defect rather than a style question.
Reading 14 registers on an Intel i3-8145U:

| Operation | Time | Allocations |
| --- | --- | --- |
| `Register` x14, one call each | 7314 ns | 28 |
| `Registers` once | 1525 ns | 6 |
| `Register` once, for scale | 638 ns | 2 |

That is the whole argument for the bulk API: 4.8x faster for the same work.
The same reasoning puts the statistics gather and the guest address space walk
in C -- `Stats` returns 42 counters with their descriptions in 15 us, where one
transition per description would cost more than twice that.

`make bench` reproduces these; the register and statistics benchmarks need
root, the rest do not.

## Next steps

Nothing below is required for what the library already does. They are the
known gaps, in the order they would most likely be useful.

- **`arm64` and `riscv64`.** The tree is ready for them: every
  architecture-specific declaration already sits behind a build constraint, so
  adding one means adding files rather than unpicking runtime switches. What
  it does not mean is translating these bindings. FreeBSD's arm64 VMM has a
  different register set and a VGIC in place of the LAPIC and IOAPIC, and
  riscv64 has an APLIC; each needs reading against its own headers, on a host
  that can run it.
- **Verifying snapshots.** The primitives are implemented but have never been
  seen to succeed: snapshot support is a kernel build option, and the
  development host runs a kernel built without it, so the integration tests
  skip on ENOTTY. They need a kernel with `BHYVE_SNAPSHOT` before the save and
  restore paths can be trusted.
- **Passthrough on real hardware.** Assignment, MMIO mapping, MSI and MSI-X
  are implemented, and the error path is tested, but no device has been passed
  through: that needs a host with a spare PCI device bound to `ppt(4)`.
- **Passthrough device reset.** No supported libvmmapi exposes an entry point,
  so `ResetPassthrough` reports `ErrUnsupported`. If FreeBSD 15 adds one, this
  becomes a small change plus a feature flag that can answer honestly.
- **A guest in protected or long mode.** The test payload is five bytes of
  real-mode code, which is enough to exercise entry, exit decoding and
  resumption. Paged-mode address translation and `ReadGuestVirtual` across a
  real page table are only covered by the identity mapping that real mode
  gives, so a payload that enables paging would test them properly.
- **CI.** The pure-Go tests run anywhere; the integration tests need a FreeBSD
  host with `vmm(4)` and privilege. Both levels exist and are green, but
  nothing runs them automatically yet.

## Status

The whole v1.0 surface for FreeBSD on amd64 is implemented: VM and vCPU
lifecycle, CPU topology and state, registers including the bulk calls, guest
and device memory, capabilities, statistics, guest execution with typed VM
exits, interrupt injection, PCI passthrough, the RTC, address translation,
guest memory access through linear addresses, and the low-level snapshot
calls.

Everything except the snapshot calls has been exercised against a real kernel,
including executing guest code and reading it back through the guest's own
address space.

Not implemented: `arm64` and `riscv64`; passthrough device reset, which no
supported libvmmapi exposes; and instruction decoding for emulated MMIO, which
is out of scope as described in the package documentation. See Next steps.

## Relationship to go-freebsd-bhyve

This library is the bottom of an intended stack, and knows nothing about the
layers above it:

```
        go-freebsd-bhyve        VM specification, networking, storage,
               |                the bhyve process
               v
         go-freebsd-vmm         runtime inspection and control
               |
               v
           libvmmapi
```

A higher-level runtime would start a VM its own way and then open it here for
CPU inspection, statistics, register access, suspend and resume. The
dependency only ever points downward: `go-freebsd-vmm` must not depend on
`go-freebsd-bhyve`.

Device models, `tap(4)`, bridges, disk images, ZFS zvols, UEFI paths and
command-line construction are all out of scope here, permanently.

## License

BSD-2-Clause. See [LICENSE](LICENSE).
