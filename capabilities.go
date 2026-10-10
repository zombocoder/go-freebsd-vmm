//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Capability identifies a per-vCPU VMM capability: a knob on how the
// virtual machine monitor handles the guest, such as whether a given
// instruction causes a VM exit.
type Capability int

// amd64 VMM capabilities, numbered by FreeBSD's enum vm_cap_type.
const (
	// CapHaltExit makes the guest's HLT instruction exit to the VMM
	// rather than idle inside the guest.
	CapHaltExit Capability = vmmapi.CapHaltExit

	// CapMTrapExit makes monitor trap flag events exit to the VMM.
	CapMTrapExit Capability = vmmapi.CapMTrapExit

	// CapPauseExit makes the guest's PAUSE instruction exit to the
	// VMM.
	CapPauseExit Capability = vmmapi.CapPauseExit

	// CapUnrestrictedGuest allows the guest to run in unpaged
	// protected mode and real mode.
	CapUnrestrictedGuest Capability = vmmapi.CapUnrestrictedGuest

	// CapEnableINVPCID exposes the INVPCID instruction to the guest.
	CapEnableINVPCID Capability = vmmapi.CapEnableINVPCID

	// CapBPTExit makes breakpoint exceptions exit to the VMM, which
	// is what a debugger attached to the guest needs.
	CapBPTExit Capability = vmmapi.CapBPTExit

	// CapRDPID exposes the RDPID instruction to the guest.
	CapRDPID Capability = vmmapi.CapRDPID

	// CapRDTSCP exposes the RDTSCP instruction to the guest.
	CapRDTSCP Capability = vmmapi.CapRDTSCP

	// CapIPIExit makes inter-processor interrupts exit to the VMM
	// instead of being delivered by the kernel.
	CapIPIExit Capability = vmmapi.CapIPIExit

	// CapMaskHWIntr masks hardware interrupt delivery to the guest.
	CapMaskHWIntr Capability = vmmapi.CapMaskHWIntr

	// CapRFlagsTF controls the guest's trap flag, for single
	// stepping.
	CapRFlagsTF Capability = vmmapi.CapRFlagsTF
)

// valid reports whether the capability is one this FreeBSD release has.
func (c Capability) valid() bool {
	return c >= 0 && int(c) < vmmapi.CapMax
}

// String returns FreeBSD's name for the capability, as accepted by
// CapabilityByName, or its number when the running release has a
// capability this package does not name.
func (c Capability) String() string {
	if name := vmmapi.CapabilityName(int(c)); name != "" {
		return name
	}
	return fmt.Sprintf("cap(%d)", int(c))
}

// CapabilityByName resolves a capability by the name FreeBSD knows it
// by, for example "hlt_exit".
//
// The mapping comes from libvmmapi rather than from a table here, so it
// follows whatever the running release supports.
func CapabilityByName(name string) (Capability, error) {
	t := vmmapi.CapabilityByName(name)
	if t < 0 {
		return 0, fmt.Errorf("vmm: unknown capability %q: %w",
			name, ErrInvalidArgument)
	}
	return Capability(t), nil
}

// Capability reads the current value of a capability for this vCPU.
func (cpu *VCPU) Capability(capability Capability) (int, error) {
	const op = "vm_get_capability"

	if !capability.valid() {
		return 0, fmt.Errorf("vmm: capability %d is out of range: %w",
			int(capability), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return 0, err
	}
	defer release()

	val, errno := h.GetCapability(int(capability))
	if errno != 0 {
		return 0, cpu.err(op, errno)
	}
	return val, nil
}

// SetCapability sets a capability for this vCPU.
//
// A capability the host processor does not implement is reported by the
// kernel as EINVAL.
func (cpu *VCPU) SetCapability(capability Capability, value int) error {
	const op = "vm_set_capability"

	if !capability.valid() {
		return fmt.Errorf("vmm: capability %d is out of range: %w",
			int(capability), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.SetCapability(int(capability), value))
}
