//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// VCPUOpen creates a vCPU handle for vcpuid on ctx.
//
// vm_vcpu_open(3) performs no ioctl: it only allocates and fills in
// {ctx, vcpuid}. It therefore cannot tell a valid vcpuid from an invalid
// one, and an out-of-range id surfaces much later as a puzzling EINVAL
// from the first register access. Callers are expected to range-check
// against the VM topology before getting here.
func VCPUOpen(c *Context, vcpuid int) (*VCPU, syscall.Errno) {
	if !c.valid() {
		return nil, syscall.EINVAL
	}
	cid, ok := toCInt(vcpuid)
	if !ok {
		return nil, syscall.EINVAL
	}

	r := C.govmm_vcpu_open(c.ptr, cid)
	if r.ptr == nil {
		return nil, errnoOf(r.err)
	}
	return &VCPU{ptr: (*C.struct_vcpu)(r.ptr)}, 0
}

// Close releases the vCPU handle. Calling it twice is harmless.
func (v *VCPU) Close() {
	if !v.valid() {
		return
	}
	ptr := v.ptr
	v.ptr = nil
	C.govmm_vcpu_close(ptr)
}

// Invalidate drops the handle without freeing it.
//
// Used when the owning context has already been closed or destroyed:
// libvmmapi frees the struct vmctx at that point, and struct vcpu holds
// a pointer to it, so touching the vCPU afterwards is use-after-free.
// Leaking the small allocation is the lesser evil, and the owner above
// only reaches this path when the caller skipped its own cleanup.
func (v *VCPU) Invalidate() {
	if v != nil {
		v.ptr = nil
	}
}

// ID returns the vcpuid the handle was opened with.
func (v *VCPU) ID() int {
	if !v.valid() {
		return -1
	}
	return int(C.govmm_vcpu_id(v.ptr))
}

// Reset resets the vCPU register state.
func (v *VCPU) Reset() syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vcpu_reset(v.ptr))
}

// Activate marks the vCPU active in the VM.
func (v *VCPU) Activate() syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_activate_cpu(v.ptr))
}

// Suspend suspends this vCPU.
func (v *VCPU) Suspend() syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_suspend_cpu(v.ptr))
}

// Resume resumes this vCPU.
func (v *VCPU) Resume() syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_resume_cpu(v.ptr))
}

// RestartInstruction arranges for the current instruction to be
// re-executed on the next entry.
func (v *VCPU) RestartInstruction() syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_restart_instruction(v.ptr))
}

// SuspendAllCPUs suspends every vCPU in the VM.
func (c *Context) SuspendAllCPUs() syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_suspend_all_cpus(c.ptr))
}

// ResumeAllCPUs resumes every vCPU in the VM.
func (c *Context) ResumeAllCPUs() syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_resume_all_cpus(c.ptr))
}
