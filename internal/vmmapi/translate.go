//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import (
	"syscall"
	"unsafe"
)

// Paging is the guest paging state a translation has to be performed
// under: the page table root, the privilege level, and the mode the
// processor is in.
type Paging struct {
	CR3        uint64
	CPL        int
	CPUMode    int
	PagingMode int
}

// cPaging converts the paging state for the bridge.
func (p Paging) cPaging() (C.struct_govmm_paging, bool) {
	cpl, ok := toCInt(p.CPL)
	if !ok {
		return C.struct_govmm_paging{}, false
	}
	mode, ok := toCInt(p.CPUMode)
	if !ok {
		return C.struct_govmm_paging{}, false
	}
	paging, ok := toCInt(p.PagingMode)
	if !ok {
		return C.struct_govmm_paging{}, false
	}
	return C.struct_govmm_paging{
		cr3:         C.uint64_t(p.CR3),
		cpl:         C.int32_t(cpl),
		cpu_mode:    C.int32_t(mode),
		paging_mode: C.int32_t(paging),
	}, true
}

// GLA2GPA translates a guest linear address to a guest physical one,
// walking the guest's page tables.
//
// A guest paging fault is reported through the returned fault flag, not
// as an error: the kernel has injected an exception into the guest and
// no translation happened.
func (v *VCPU) GLA2GPA(paging Paging, gla uint64, prot int) (gpa uint64, fault bool, errno syscall.Errno) {
	return v.gla2gpa(paging, gla, prot, false)
}

// GLA2GPANoFault translates without injecting an exception on failure,
// which is what an inspection tool wants.
func (v *VCPU) GLA2GPANoFault(paging Paging, gla uint64, prot int) (gpa uint64, fault bool, errno syscall.Errno) {
	return v.gla2gpa(paging, gla, prot, true)
}

func (v *VCPU) gla2gpa(paging Paging, gla uint64, prot int, nofault bool) (uint64, bool, syscall.Errno) {
	if !v.valid() {
		return 0, false, syscall.EINVAL
	}
	cpaging, ok := paging.cPaging()
	if !ok {
		return 0, false, syscall.EINVAL
	}
	cprot, ok := toCInt(prot)
	if !ok {
		return 0, false, syscall.EINVAL
	}

	var cgpa C.uint64_t
	var cfault C.int

	var r C.struct_govmm_result
	if nofault {
		r = C.govmm_vm_gla2gpa_nofault(v.ptr, &cpaging,
			C.uint64_t(gla), cprot, &cgpa, &cfault)
	} else {
		r = C.govmm_vm_gla2gpa(v.ptr, &cpaging, C.uint64_t(gla),
			cprot, &cgpa, &cfault)
	}
	if errno := res(r); errno != 0 {
		return 0, false, errno
	}
	return uint64(cgpa), cfault != 0, 0
}

// GPAPageMapEntries is the number of page table entries GPAPageMap can
// report, one per level of the host page table walk.
const GPAPageMapEntries = 4

// GPAPageMap returns the host page table entries backing a guest
// physical address, outermost level first.
func (c *Context) GPAPageMap(gpa uint64) ([]uint64, syscall.Errno) {
	if !c.valid() {
		return nil, syscall.EINVAL
	}

	pte := make([]C.uint64_t, GPAPageMapEntries)
	num := C.int(len(pte))

	r := C.govmm_vm_get_gpa_pmap(c.ptr, C.uint64_t(gpa), &pte[0], &num)
	if errno := res(r); errno != 0 {
		return nil, errno
	}

	n := int(num)
	if n < 0 || n > len(pte) {
		return nil, syscall.EIO
	}
	out := make([]uint64, n)
	for i := 0; i < n; i++ {
		out[i] = uint64(pte[i])
	}
	return out, 0
}

// ReadGuestVirtual copies from a guest linear address into dst.
//
// Translation, copy and teardown happen in one transition. A guest
// paging fault is reported through the fault flag rather than as an
// error; the kernel has injected an exception and nothing was copied.
func (v *VCPU) ReadGuestVirtual(paging Paging, gla uint64, dst []byte) (fault bool, errno syscall.Errno) {
	if !v.valid() {
		return false, syscall.EINVAL
	}
	if len(dst) == 0 {
		return false, 0
	}
	cpaging, ok := paging.cPaging()
	if !ok {
		return false, syscall.EINVAL
	}
	clen, ok := toCSize(uint64(len(dst)))
	if !ok {
		return false, syscall.EINVAL
	}

	var cfault C.int
	r := C.govmm_vm_read_guest_virtual(v.ptr, &cpaging, C.uint64_t(gla),
		unsafe.Pointer(&dst[0]), clen, &cfault)
	if errno := res(r); errno != 0 {
		return cfault != 0, errno
	}
	return cfault != 0, 0
}

// WriteGuestVirtual copies src to a guest linear address.
func (v *VCPU) WriteGuestVirtual(paging Paging, gla uint64, src []byte) (fault bool, errno syscall.Errno) {
	if !v.valid() {
		return false, syscall.EINVAL
	}
	if len(src) == 0 {
		return false, 0
	}
	cpaging, ok := paging.cPaging()
	if !ok {
		return false, syscall.EINVAL
	}
	clen, ok := toCSize(uint64(len(src)))
	if !ok {
		return false, syscall.EINVAL
	}

	var cfault C.int
	r := C.govmm_vm_write_guest_virtual(v.ptr, &cpaging, C.uint64_t(gla),
		unsafe.Pointer(&src[0]), clen, &cfault)
	if errno := res(r); errno != 0 {
		return cfault != 0, errno
	}
	return cfault != 0, 0
}
