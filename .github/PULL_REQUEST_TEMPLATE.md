## What and why

<!-- What does this change do, and what problem does it solve? Link issues with "Fixes #123".
     If it works around a behaviour of libvmmapi or the kernel, name it. -->

## How it was tested

<!-- CI builds and runs the pure-Go tests on FreeBSD 14.x and 15.x VMs; the
     integration and privileged levels only run there when the VM gets nested
     virtualization. If you ran them on real hardware, say where, e.g.:
     doas make test && doas make privileged on FreeBSD 15.1, AMD Ryzen (SVM) -->

## Checklist

- [ ] New behaviour is covered by tests (a failing test first, then the fix), at the lowest level that can see it
- [ ] Tests that create VMs use `tests/harness` and clean up through `t.Cleanup`
- [ ] `import "C"` stays in `internal/vmmapi`, and no C type reaches the public API
- [ ] `compat.c` captures `errno` right after each call and is clean under `make cwarn`
- [ ] Release differences are exposed through `Supports(Feature...)`, not version checks in callers
- [ ] Public API changes are documented (doc comments, README, examples) and listed in `CHANGELOG.md`
- [ ] `gofmt -l .` prints nothing
