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

// GetCapability reads the value of a VMM capability for this vCPU.
func (v *VCPU) GetCapability(cap int) (int, syscall.Errno) {
	if !v.valid() {
		return 0, syscall.EINVAL
	}
	ccap, ok := toCInt(cap)
	if !ok {
		return 0, syscall.EINVAL
	}

	var val C.int
	if errno := res(C.govmm_vm_get_capability(v.ptr, ccap, &val)); errno != 0 {
		return 0, errno
	}
	return int(val), 0
}

// SetCapability sets the value of a VMM capability for this vCPU.
func (v *VCPU) SetCapability(cap, val int) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	ccap, ok := toCInt(cap)
	if !ok {
		return syscall.EINVAL
	}
	cval, ok := toCInt(val)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_set_capability(v.ptr, ccap, cval))
}

// CapabilityByName resolves a capability name as FreeBSD spells it,
// returning a negative value when the name is unknown.
func CapabilityByName(name string) int {
	if strings.IndexByte(name, 0) >= 0 {
		return -1
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	return int(C.govmm_vm_capability_name2type(cname))
}

// CapabilityName returns FreeBSD's name for a capability, or the empty
// string if it has none.
func CapabilityName(cap int) string {
	ccap, ok := toCInt(cap)
	if !ok {
		return ""
	}
	return C.GoString(C.govmm_vm_capability_type2name(ccap))
}
