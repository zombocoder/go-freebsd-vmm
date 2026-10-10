//go:build freebsd && amd64

package vmmapi

/*
#include <stdlib.h>
#include "compat.h"
*/
import "C"

import (
	"strings"
	"syscall"
	"unsafe"
)

// SnapshotReq asks the kernel to save or restore one of its own
// structures through the caller's buffer.
//
// On a save, the number of bytes written is returned. On a restore, the
// buffer must hold what a previous save produced for the same structure.
func (c *Context) SnapshotReq(devName string, devReq, op int, buf []byte) (used uint64, errno syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}
	if len(buf) == 0 {
		return 0, syscall.EINVAL
	}
	if strings.IndexByte(devName, 0) >= 0 {
		return 0, syscall.EINVAL
	}
	creq, ok := toCInt(devReq)
	if !ok {
		return 0, syscall.EINVAL
	}
	cop, ok := toCInt(op)
	if !ok {
		return 0, syscall.EINVAL
	}
	clen, ok := toCSize(uint64(len(buf)))
	if !ok {
		return 0, syscall.EINVAL
	}

	// An empty device name means "no userspace device", which the
	// kernel expects as a null pointer rather than an empty string.
	var cname *C.char
	if devName != "" {
		cname = C.CString(devName)
		defer C.free(unsafe.Pointer(cname))
	}

	var cused C.size_t
	r := C.govmm_vm_snapshot_req(c.ptr, cname, creq, cop,
		unsafe.Pointer(&buf[0]), clen, &cused)
	if errno := res(r); errno != 0 {
		return uint64(cused), errno
	}
	return uint64(cused), 0
}

// RestoreTime tells the kernel to re-derive the VM's notion of time
// after a restore.
func (c *Context) RestoreTime() syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_restore_time(c.ptr))
}

// LimitRights drops the Capsicum rights on the VM's descriptor to the
// set vmm(4) needs, so that the process can enter capability mode.
func (c *Context) LimitRights() syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_limit_rights(c.ptr))
}
