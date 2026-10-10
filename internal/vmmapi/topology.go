//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// Topology reads the VM's CPU topology.
func (c *Context) Topology() (sockets, cores, threads, maxcpus uint16, errno syscall.Errno) {
	if !c.valid() {
		return 0, 0, 0, 0, syscall.EINVAL
	}

	var cs, cc, ct, cm C.uint16_t
	r := C.govmm_vm_get_topology(c.ptr, &cs, &cc, &ct, &cm)
	if errno := res(r); errno != 0 {
		return 0, 0, 0, 0, errno
	}
	return uint16(cs), uint16(cc), uint16(ct), uint16(cm), 0
}

// SetTopology sets the VM's CPU topology.
func (c *Context) SetTopology(sockets, cores, threads, maxcpus uint16) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_set_topology(c.ptr, C.uint16_t(sockets),
		C.uint16_t(cores), C.uint16_t(threads), C.uint16_t(maxcpus)))
}
