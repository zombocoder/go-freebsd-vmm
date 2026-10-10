//go:build !freebsd || !amd64

// Package vmm has no implementation for this platform.
//
// The library binds FreeBSD's vmm(4) through libvmmapi, so there is
// nothing it could usefully do elsewhere. Rather than offer an API that
// compiles and then fails at run time, the build stops here with the
// reason.
//
// amd64 is the only architecture implemented so far. FreeBSD's arm64 and
// riscv64 VMM interfaces differ enough -- a different register set, a
// VGIC or an APLIC in place of the LAPIC and IOAPIC -- that they need
// bindings written against those headers rather than a translation of
// these. The groundwork is here for that: everything architecture
// specific already lives behind a build constraint, so adding an
// architecture means adding files, not unpicking runtime switches.
package vmm

// The line below is a deliberate type error. It is the only way to make
// the compiler print an explanation rather than a list of undefined
// symbols from whatever imported this package.
//
// Expect:
//
//	invalid operation: "go-freebsd-vmm requires FreeBSD on amd64" - 0
//	(mismatched types untyped string and untyped int)
var _ = "go-freebsd-vmm requires FreeBSD on amd64" - 0
