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

// MemMap is one range of the guest address space.
type MemMap struct {
	GPA    uint64
	SegOff uint64
	Length uint64
	SegID  int
	Prot   int
	Flags  uint32
}

// memMapInitial is the number of map entries requested on the first
// attempt. Real guests have a handful; the call retries if a VM somehow
// has more.
const memMapInitial = 32

// SetupMemory allocates the system memory segment and maps it into the
// controlling process.
//
// Only MMapAll is accepted by libvmmapi on this release: the other
// styles hit an assertion inside vm_setup_memory(), which would abort
// the process. Callers screen the style before getting here.
func (c *Context) SetupMemory(length uint64, style int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	if style != MMapAll {
		return syscall.ENOTSUP
	}
	clen, ok := toCSize(length)
	if !ok {
		return syscall.EINVAL
	}
	cstyle, ok := toCInt(style)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_setup_memory(c.ptr, clen, cstyle))
}

// LowMemSize returns the size of the guest's low memory region.
func (c *Context) LowMemSize() uint64 {
	if !c.valid() {
		return 0
	}
	return uint64(C.govmm_vm_get_lowmem_size(c.ptr))
}

// HighMemSize returns the size of the guest's high memory region, which
// is zero unless the VM was given more memory than the low memory limit.
func (c *Context) HighMemSize() uint64 {
	if !c.valid() {
		return 0
	}
	return uint64(C.govmm_vm_get_highmem_size(c.ptr))
}

// LowMemLimit returns the guest physical address at which low memory
// stops and the high memory region begins.
func (c *Context) LowMemLimit() uint32 {
	if !c.valid() {
		return 0
	}
	return uint32(C.govmm_vm_get_lowmem_limit(c.ptr))
}

// SetLowMemLimit changes the low memory limit. It only affects
// subsequent SetupMemory calls.
func (c *Context) SetLowMemLimit(limit uint32) {
	if !c.valid() {
		return
	}
	C.govmm_vm_set_lowmem_limit(c.ptr, C.uint32_t(limit))
}

// MemFlags returns the current guest memory flags.
func (c *Context) MemFlags() int {
	if !c.valid() {
		return 0
	}
	return int(C.govmm_vm_get_memflags(c.ptr))
}

// SetMemFlags sets the guest memory flags. It only affects subsequent
// mapping operations.
func (c *Context) SetMemFlags(flags int) {
	if !c.valid() {
		return
	}
	cflags, ok := toCInt(flags)
	if !ok {
		return
	}
	C.govmm_vm_set_memflags(c.ptr, cflags)
}

// MapGPA returns the host mapping of [gpa, gpa+length) in guest
// physical memory.
func (c *Context) MapGPA(gpa, length uint64) (*Region, syscall.Errno) {
	if !c.valid() {
		return nil, syscall.EINVAL
	}
	if length == 0 {
		return nil, syscall.EINVAL
	}
	clen, ok := toCSize(length)
	if !ok {
		return nil, syscall.EINVAL
	}

	r := C.govmm_vm_map_gpa(c.ptr, C.uint64_t(gpa), clen)
	if r.ptr == nil {
		return nil, errnoOf(r.err)
	}
	return newRegion(r.ptr, length), 0
}

// RevMapGPA returns the guest physical address corresponding to offset
// off within region.
func (c *Context) RevMapGPA(region *Region, off uint64) (uint64, syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}
	ptr, ok := region.pointerAt(off)
	if !ok {
		return 0, syscall.EINVAL
	}

	r := C.govmm_vm_rev_map_gpa(c.ptr, ptr)
	if r.rc != 0 {
		return 0, errnoOf(r.err)
	}
	return uint64(r.val), 0
}

// MemSeg returns the length and name of a memory segment. System memory
// segments report an empty name.
func (c *Context) MemSeg(segid int) (length uint64, name string, errno syscall.Errno) {
	if !c.valid() {
		return 0, "", syscall.EINVAL
	}
	cseg, ok := toCInt(segid)
	if !ok {
		return 0, "", syscall.EINVAL
	}

	buf := make([]byte, SegNameLen)
	var clen C.size_t
	r := C.govmm_vm_get_memseg(c.ptr, cseg, &clen,
		(*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	if errno := res(r); errno != 0 {
		return 0, "", errno
	}

	if i := strings.IndexByte(string(buf), 0); i >= 0 {
		name = string(buf[:i])
	} else {
		name = string(buf)
	}
	return uint64(clen), name, 0
}

// CreateDevMem creates a device memory segment and maps it into the
// controlling process.
func (c *Context) CreateDevMem(segid int, name string, length uint64) (*Region, syscall.Errno) {
	if !c.valid() {
		return nil, syscall.EINVAL
	}
	if length == 0 || strings.IndexByte(name, 0) >= 0 {
		return nil, syscall.EINVAL
	}
	cseg, ok := toCInt(segid)
	if !ok {
		return nil, syscall.EINVAL
	}
	clen, ok := toCSize(length)
	if !ok {
		return nil, syscall.EINVAL
	}

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	r := C.govmm_vm_create_devmem(c.ptr, cseg, cname, clen)
	if r.ptr == nil {
		return nil, errnoOf(r.err)
	}
	return newRegion(r.ptr, length), 0
}

// MMapMemSeg maps part of a memory segment into the guest address space.
func (c *Context) MMapMemSeg(gpa uint64, segid int, segoff, length uint64, prot int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cseg, ok := toCInt(segid)
	if !ok {
		return syscall.EINVAL
	}
	cprot, ok := toCInt(prot)
	if !ok {
		return syscall.EINVAL
	}
	clen, ok := toCSize(length)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_mmap_memseg(c.ptr, C.uint64_t(gpa), cseg,
		C.uint64_t(segoff), clen, cprot))
}

// MUnmapMemSeg removes a mapping from the guest address space.
func (c *Context) MUnmapMemSeg(gpa, length uint64) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	clen, ok := toCSize(length)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_munmap_memseg(c.ptr, C.uint64_t(gpa), clen))
}

// MemMaps enumerates the guest address space.
//
// The walk happens in C so that a guest with many mappings costs one cgo
// transition rather than one per range.
func (c *Context) MemMaps() ([]MemMap, syscall.Errno) {
	if !c.valid() {
		return nil, syscall.EINVAL
	}

	capacity := memMapInitial
	for {
		buf := make([]C.struct_govmm_memmap, capacity)
		cmax, ok := toCInt(capacity)
		if !ok {
			return nil, syscall.EINVAL
		}

		var count C.int
		r := C.govmm_vm_memmaps(c.ptr, &buf[0], cmax, &count)
		if errno := res(r); errno != 0 {
			if errno == syscall.ENOSPC && int(count) > capacity {
				capacity = int(count)
				continue
			}
			return nil, errno
		}

		n := int(count)
		if n > capacity {
			// Cannot happen: a short buffer reports ENOSPC.
			return nil, syscall.EIO
		}
		out := make([]MemMap, n)
		for i := 0; i < n; i++ {
			out[i] = MemMap{
				GPA:    uint64(buf[i].gpa),
				SegOff: uint64(buf[i].segoff),
				Length: uint64(buf[i].len),
				SegID:  int(buf[i].segid),
				Prot:   int(buf[i].prot),
				Flags:  uint32(buf[i].flags),
			}
		}
		return out, 0
	}
}
