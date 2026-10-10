//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"syscall"
)

// ErrGuestFault reports that an address translation or guest memory
// access did not happen because the guest itself faulted.
//
// This is not a failure of the call: the guest's page tables did not map
// the address, or not with the access rights asked for. For the faulting
// variants the kernel has injected an exception into the guest, which
// will see the fault when it next runs. For the non-faulting variants
// nothing was injected and the guest is unaware.
var ErrGuestFault = fmt.Errorf("vmm: guest page fault")

// GuestLinearToPhysical translates a guest linear address to a guest
// physical one by walking the guest's page tables.
//
// prot is the access to check for, built from ProtRead, ProtWrite and
// ProtExec. If the guest's tables do not allow it, the kernel injects a
// page fault into the guest and the returned error wraps ErrGuestFault.
//
// The paging state normally comes from a VM exit that reported one,
// since assembling it from registers means reproducing the processor's
// own rules about which mode it is in.
func (cpu *VCPU) GuestLinearToPhysical(paging PagingState, gla uint64, prot MemoryProtection) (GuestAddress, error) {
	return cpu.gla2gpa("vm_gla2gpa", paging, gla, prot, false)
}

// GuestLinearToPhysicalNoFault translates without injecting anything
// into the guest when the translation fails.
//
// This is what an inspection tool or a debugger wants: a failed lookup
// returns an error wrapping ErrGuestFault and leaves the guest
// undisturbed.
func (cpu *VCPU) GuestLinearToPhysicalNoFault(paging PagingState, gla uint64, prot MemoryProtection) (GuestAddress, error) {
	return cpu.gla2gpa("vm_gla2gpa_nofault", paging, gla, prot, true)
}

func (cpu *VCPU) gla2gpa(op string, paging PagingState, gla uint64, prot MemoryProtection, nofault bool) (GuestAddress, error) {
	if err := paging.validate(); err != nil {
		return 0, err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return 0, err
	}
	defer release()

	var gpa uint64
	var fault bool
	var errno syscall.Errno

	if nofault {
		gpa, fault, errno = h.GLA2GPANoFault(paging.bridge(), gla,
			int(prot))
	} else {
		gpa, fault, errno = h.GLA2GPA(paging.bridge(), gla, int(prot))
	}
	if errno != 0 {
		return 0, cpu.err(op, errno)
	}
	if fault {
		return 0, fmt.Errorf("vmm: %s %#x: %w", op, gla, ErrGuestFault)
	}
	return GuestAddress(gpa), nil
}

// GuestPageMap returns the host page table entries backing a guest
// physical address, outermost level first.
//
// This exposes how the host has mapped the guest's memory, which is what
// a tool inspecting large page use or memory placement needs. An entry's
// meaning is the host processor's, not the guest's.
func (vm *VM) GuestPageMap(gpa GuestAddress) ([]uint64, error) {
	const op = "vm_get_gpa_pmap"

	ctx, release, err := vm.context(op)
	if err != nil {
		return nil, err
	}
	defer release()

	pte, errno := ctx.GPAPageMap(uint64(gpa))
	if errno != 0 {
		return nil, vm.err(op, errno)
	}
	return pte, nil
}
