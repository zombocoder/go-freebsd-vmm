//go:build freebsd && amd64

// Package vmmapi is the cgo bridge to FreeBSD's libvmmapi.
//
// This package is the only place in the repository where "import \"C\"" is
// allowed to appear. Nothing it exports may carry a C type, so that the
// public package above it can be written in ordinary Go.
//
// Functions here report failure as a syscall.Errno, where the zero value
// means success. Decorating that into a richer error, with the operation
// name and the VM and vCPU it happened on, is the caller's job.
package vmmapi

/*
#cgo CFLAGS: -Wall -Wextra
#cgo LDFLAGS: -lvmmapi

#include "compat.h"
*/
import "C"

import (
	"math"
	"syscall"
)

// Context owns one libvmmapi struct vmctx.
//
// The pointer is released by Close or Destroy; both free the underlying
// allocation, so exactly one of them may be called, exactly once. The
// owner above is responsible for enforcing that.
type Context struct {
	ptr *C.struct_vmctx
}

// VCPU owns one libvmmapi struct vcpu. It is only valid while the
// Context it was opened from is.
type VCPU struct {
	ptr *C.struct_vcpu
}

// valid reports whether the context still holds a live pointer.
func (c *Context) valid() bool { return c != nil && c.ptr != nil }

// valid reports whether the vCPU still holds a live pointer.
func (v *VCPU) valid() bool { return v != nil && v.ptr != nil }

// res converts a bridge result into an errno, 0 meaning success.
//
// A failing libvmmapi call that left errno clear is reported as EIO
// rather than as success, so a non-zero rc never turns into a nil error.
func res(r C.struct_govmm_result) syscall.Errno {
	if r.rc == 0 {
		return 0
	}
	if r.err == 0 {
		return syscall.EIO
	}
	return syscall.Errno(r.err)
}

// errnoOf converts a captured errno, substituting EIO for a clear one.
func errnoOf(err C.int) syscall.Errno {
	if err == 0 {
		return syscall.EIO
	}
	return syscall.Errno(err)
}

// toCInt converts a Go int, reporting whether it fits.
func toCInt(v int) (C.int, bool) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, false
	}
	return C.int(v), true
}

// toCUInt converts a Go int to an unsigned C int, reporting whether it fits.
func toCUInt(v int) (C.uint, bool) {
	if v < 0 || v > math.MaxUint32 {
		return 0, false
	}
	return C.uint(v), true
}

// APIVersionString returns VMMAPI_VERSION exactly as it is spelled in
// <vmmapi.h>, for example "0200".
//
// The spelling matters: the macro is a leading-zero C literal, so its
// numeric value is octal and arithmetic on it does not recover the major
// and minor parts. Callers decode the digits instead.
func APIVersionString() string {
	return C.GoString(C.govmm_api_version_string())
}

// CPUSetWords returns the number of 64-bit words needed to hold a
// cpuset_t, read from the C type rather than assumed.
func CPUSetWords() int {
	return int(C.govmm_cpuset_words())
}

// StatDescLen is the fixed width of one statistics description as copied
// out of the kernel.
const StatDescLen = C.GOVMM_STAT_DESC_LEN

// toCSize converts a byte count, reporting whether it survives the
// conversion to size_t. The round trip makes the check correct on any
// architecture rather than relying on size_t being 64 bits wide.
func toCSize(v uint64) (C.size_t, bool) {
	c := C.size_t(v)
	if uint64(c) != v {
		return 0, false
	}
	return c, true
}
