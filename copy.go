//go:build freebsd && amd64

package vmm

import "fmt"

// maxGuestCopy bounds a single guest virtual copy.
//
// The limit is this library's, not the kernel's: the copy is one
// uninterruptible pass through the guest's page tables, and a caller
// asking for an unbounded amount would hold the vCPU for as long as it
// took. Larger transfers are a loop in the caller, where they can be
// paced.
const maxGuestCopy = 16 << 20

// ReadGuestVirtual copies len(dst) bytes from a guest linear address
// into dst.
//
// The address is translated through the guest's page tables under the
// given paging state, so this reads what the guest would read at that
// address rather than at a physical one. Translation and copy happen in
// a single kernel call.
//
// If the guest's tables do not map the whole range for reading, nothing
// is copied, the kernel injects a page fault into the guest, and the
// error wraps ErrGuestFault.
//
// The bytes are guest-controlled and must be treated as untrusted
// input.
func (cpu *VCPU) ReadGuestVirtual(addr uint64, dst []byte, paging PagingState) error {
	const op = "vm_copyin"

	if len(dst) == 0 {
		return nil
	}
	if err := checkGuestCopy(len(dst), paging); err != nil {
		return err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	fault, errno := h.ReadGuestVirtual(paging.bridge(), addr, dst)
	if errno != 0 {
		return cpu.err(op, errno)
	}
	if fault {
		return fmt.Errorf("vmm: %s %#x+%d: %w", op, addr, len(dst),
			ErrGuestFault)
	}
	return nil
}

// WriteGuestVirtual copies src to a guest linear address.
//
// The address is translated under the given paging state and checked for
// write access. If the guest's tables do not allow the whole range,
// nothing is written, a page fault is injected into the guest, and the
// error wraps ErrGuestFault.
//
// Nothing synchronises this against the guest: a vCPU may be reading the
// same addresses while the write lands.
func (cpu *VCPU) WriteGuestVirtual(addr uint64, src []byte, paging PagingState) error {
	const op = "vm_copyout"

	if len(src) == 0 {
		return nil
	}
	if err := checkGuestCopy(len(src), paging); err != nil {
		return err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	fault, errno := h.WriteGuestVirtual(paging.bridge(), addr, src)
	if errno != 0 {
		return cpu.err(op, errno)
	}
	if fault {
		return fmt.Errorf("vmm: %s %#x+%d: %w", op, addr, len(src),
			ErrGuestFault)
	}
	return nil
}

// checkGuestCopy validates a copy request before it reaches C.
func checkGuestCopy(n int, paging PagingState) error {
	if n > maxGuestCopy {
		return fmt.Errorf("vmm: guest copy of %d bytes exceeds the %d byte limit: %w",
			n, maxGuestCopy, ErrInvalidArgument)
	}
	return paging.validate()
}
