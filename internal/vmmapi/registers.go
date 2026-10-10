//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// GetRegister reads one guest register.
func (v *VCPU) GetRegister(reg int) (uint64, syscall.Errno) {
	if !v.valid() {
		return 0, syscall.EINVAL
	}
	creg, ok := toCInt(reg)
	if !ok {
		return 0, syscall.EINVAL
	}

	var val C.uint64_t
	if errno := res(C.govmm_vm_get_register(v.ptr, creg, &val)); errno != 0 {
		return 0, errno
	}
	return uint64(val), 0
}

// SetRegister writes one guest register.
func (v *VCPU) SetRegister(reg int, val uint64) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	creg, ok := toCInt(reg)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_set_register(v.ptr, creg, C.uint64_t(val)))
}

// GetRegisterSet reads len(regs) registers in a single transition and
// writes the values into vals, which must be at least as long.
//
// This is the reason the bulk API exists: one cgo crossing for the whole
// set instead of one per register.
func (v *VCPU) GetRegisterSet(regs []int, vals []uint64) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	if len(regs) == 0 {
		return 0
	}
	if len(vals) < len(regs) {
		return syscall.EINVAL
	}
	count, ok := toCUInt(len(regs))
	if !ok {
		return syscall.EINVAL
	}

	cregs, errno := cRegNums(regs)
	if errno != 0 {
		return errno
	}
	cvals := make([]C.uint64_t, len(regs))

	r := C.govmm_vm_get_register_set(v.ptr, count, &cregs[0], &cvals[0])
	if errno := res(r); errno != 0 {
		return errno
	}
	for i := range cregs {
		vals[i] = uint64(cvals[i])
	}
	return 0
}

// SetRegisterSet writes len(regs) registers in a single transition.
func (v *VCPU) SetRegisterSet(regs []int, vals []uint64) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	if len(regs) == 0 {
		return 0
	}
	if len(vals) < len(regs) {
		return syscall.EINVAL
	}
	count, ok := toCUInt(len(regs))
	if !ok {
		return syscall.EINVAL
	}

	cregs, errno := cRegNums(regs)
	if errno != 0 {
		return errno
	}
	cvals := make([]C.uint64_t, len(regs))
	for i := range regs {
		cvals[i] = C.uint64_t(vals[i])
	}

	return res(C.govmm_vm_set_register_set(v.ptr, count, &cregs[0],
		&cvals[0]))
}

// cRegNums converts register identifiers for the bulk calls, rejecting
// any value that would not survive the narrowing to a C int.
func cRegNums(regs []int) ([]C.int, syscall.Errno) {
	out := make([]C.int, len(regs))
	for i, reg := range regs {
		creg, ok := toCInt(reg)
		if !ok {
			return nil, syscall.EINVAL
		}
		out[i] = creg
	}
	return out, 0
}

// GetDesc reads a segment descriptor.
func (v *VCPU) GetDesc(reg int) (base uint64, limit, access uint32, errno syscall.Errno) {
	if !v.valid() {
		return 0, 0, 0, syscall.EINVAL
	}
	creg, ok := toCInt(reg)
	if !ok {
		return 0, 0, 0, syscall.EINVAL
	}

	var cbase C.uint64_t
	var climit, caccess C.uint32_t
	r := C.govmm_vm_get_desc(v.ptr, creg, &cbase, &climit, &caccess)
	if errno := res(r); errno != 0 {
		return 0, 0, 0, errno
	}
	return uint64(cbase), uint32(climit), uint32(caccess), 0
}

// SetDesc writes a segment descriptor.
func (v *VCPU) SetDesc(reg int, base uint64, limit, access uint32) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	creg, ok := toCInt(reg)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_set_desc(v.ptr, creg, C.uint64_t(base),
		C.uint32_t(limit), C.uint32_t(access)))
}
