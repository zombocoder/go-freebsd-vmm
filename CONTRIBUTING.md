# Contributing

## Building and testing

```sh
make test        # pure-Go tests; no kernel required
make race        # the same under the race detector
make lint        # go vet, staticcheck, and the C bridge under -Werror
make bench       # benchmarks
make privileged  # level 3; needs root and vmm(4)
```

On a FreeBSD installation built without bhyve, `<vmmapi.h>` is present but
there is no library to link against. Build one from the system sources first:

```sh
make libvmmapi
```

Integration tests under `tests/` drive a real kernel. They need root and
`vmm(4)`, and skip with the reason when either is missing:

```sh
kldload vmm
sudo -E make test
```

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
