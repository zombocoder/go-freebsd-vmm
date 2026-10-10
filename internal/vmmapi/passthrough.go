//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// pptAddr converts a PCI address for the passthrough calls.
func pptAddr(bus, slot, fn int) (cbus, cslot, cfn C.int, ok bool) {
	if cbus, ok = toCInt(bus); !ok {
		return 0, 0, 0, false
	}
	if cslot, ok = toCInt(slot); !ok {
		return 0, 0, 0, false
	}
	if cfn, ok = toCInt(fn); !ok {
		return 0, 0, 0, false
	}
	return cbus, cslot, cfn, true
}

// AssignPPTDev hands a host PCI device to the guest.
func (c *Context) AssignPPTDev(bus, slot, fn int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_assign_pptdev(c.ptr, cbus, cslot, cfn))
}

// UnassignPPTDev takes a host PCI device back from the guest.
func (c *Context) UnassignPPTDev(bus, slot, fn int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_unassign_pptdev(c.ptr, cbus, cslot, cfn))
}

// MapPPTDevMMIO maps a region of host physical memory belonging to a
// passthrough device into the guest address space.
func (c *Context) MapPPTDevMMIO(bus, slot, fn int, gpa, length, hpa uint64) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	clen, ok := toCSize(length)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_map_pptdev_mmio(c.ptr, cbus, cslot, cfn,
		C.uint64_t(gpa), clen, C.uint64_t(hpa)))
}

// UnmapPPTDevMMIO removes a passthrough device MMIO mapping.
func (c *Context) UnmapPPTDevMMIO(bus, slot, fn int, gpa, length uint64) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	clen, ok := toCSize(length)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_unmap_pptdev_mmio(c.ptr, cbus, cslot, cfn,
		C.uint64_t(gpa), clen))
}

// SetupPPTDevMSI configures MSI delivery for a passthrough device.
func (c *Context) SetupPPTDevMSI(bus, slot, fn int, addr, msg uint64, numvec int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	cnumvec, ok := toCInt(numvec)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_setup_pptdev_msi(c.ptr, cbus, cslot, cfn,
		C.uint64_t(addr), C.uint64_t(msg), cnumvec))
}

// SetupPPTDevMSIX configures one MSI-X table entry for a passthrough
// device.
func (c *Context) SetupPPTDevMSIX(bus, slot, fn, idx int, addr, msg uint64, vectorControl uint32) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	cidx, ok := toCInt(idx)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_setup_pptdev_msix(c.ptr, cbus, cslot, cfn, cidx,
		C.uint64_t(addr), C.uint64_t(msg), C.uint32_t(vectorControl)))
}

// DisablePPTDevMSIX turns MSI-X off for a passthrough device.
func (c *Context) DisablePPTDevMSIX(bus, slot, fn int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cbus, cslot, cfn, ok := pptAddr(bus, slot, fn)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_disable_pptdev_msix(c.ptr, cbus, cslot, cfn))
}
