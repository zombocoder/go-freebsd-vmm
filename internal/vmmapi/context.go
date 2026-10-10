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

// Create creates a new VM, loading the vmm(4) module first if needed.
// The VM exists in the kernel afterwards but is not opened.
func Create(name string) syscall.Errno {
	if strings.IndexByte(name, 0) >= 0 {
		return syscall.EINVAL
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	return res(C.govmm_vm_create(cname))
}

// Open opens an existing VM and returns a context owning its
// struct vmctx.
func Open(name string) (*Context, syscall.Errno) {
	if strings.IndexByte(name, 0) >= 0 {
		return nil, syscall.EINVAL
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	r := C.govmm_vm_open(cname)
	if r.ptr == nil {
		return nil, errnoOf(r.err)
	}
	return &Context{ptr: (*C.struct_vmctx)(r.ptr)}, 0
}

// Close releases the context without destroying the VM.
//
// vm_close(3) frees the struct vmctx, so the pointer is dropped here and
// the context is inert afterwards. Calling Close twice is harmless.
func (c *Context) Close() {
	if !c.valid() {
		return
	}
	ptr := c.ptr
	c.ptr = nil
	C.govmm_vm_close(ptr)
}

// Destroy destroys the VM and releases the context.
//
// vm_destroy(3) closes the descriptor, asks the kernel to destroy the VM
// and frees the struct vmctx, all in one call. That makes Destroy and
// Close mutually exclusive: whichever runs first consumes the pointer.
//
// vm_destroy(3) returns void on this libvmmapi, so a kernel-side failure
// is not observable here.
func (c *Context) Destroy() {
	if !c.valid() {
		return
	}
	ptr := c.ptr
	c.ptr = nil
	C.govmm_vm_destroy(ptr)
}

// Reinit resets the VM to its freshly created state.
func (c *Context) Reinit() syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_reinit(c.ptr))
}

// Suspend suspends the VM with one of the SuspendXxx reasons.
func (c *Context) Suspend(how int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	chow, ok := toCInt(how)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_suspend(c.ptr, chow))
}

// Name returns the VM name held by the context.
func (c *Context) Name() string {
	if !c.valid() {
		return ""
	}
	return C.GoString(C.govmm_vm_get_name(c.ptr))
}
