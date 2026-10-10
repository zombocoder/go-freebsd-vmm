//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// cpuSetKind selects which of the three CPU state queries to run.
type cpuSetKind int

const (
	cpuSetActive cpuSetKind = iota
	cpuSetSuspended
	cpuSetDebug
)

// cpuSet runs one of the CPU state queries and returns the result as an
// array of 64-bit words, bit n of word n/64 standing for CPU n.
func (c *Context) cpuSet(kind cpuSetKind) ([]uint64, syscall.Errno) {
	if !c.valid() {
		return nil, syscall.EINVAL
	}

	nwords := CPUSetWords()
	if nwords <= 0 {
		return nil, syscall.EIO
	}
	words := make([]uint64, nwords)

	cn, ok := toCInt(nwords)
	if !ok {
		return nil, syscall.EINVAL
	}
	p := (*C.uint64_t)(&words[0])

	var r C.struct_govmm_result
	switch kind {
	case cpuSetActive:
		r = C.govmm_vm_active_cpus(c.ptr, p, cn)
	case cpuSetSuspended:
		r = C.govmm_vm_suspended_cpus(c.ptr, p, cn)
	case cpuSetDebug:
		r = C.govmm_vm_debug_cpus(c.ptr, p, cn)
	default:
		return nil, syscall.EINVAL
	}

	if errno := res(r); errno != 0 {
		return nil, errno
	}
	return words, 0
}

// ActiveCPUs returns the set of vCPUs the kernel considers active.
func (c *Context) ActiveCPUs() ([]uint64, syscall.Errno) {
	return c.cpuSet(cpuSetActive)
}

// SuspendedCPUs returns the set of suspended vCPUs.
func (c *Context) SuspendedCPUs() ([]uint64, syscall.Errno) {
	return c.cpuSet(cpuSetSuspended)
}

// DebugCPUs returns the set of vCPUs halted for debugging.
func (c *Context) DebugCPUs() ([]uint64, syscall.Errno) {
	return c.cpuSet(cpuSetDebug)
}
