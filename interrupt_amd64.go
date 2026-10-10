//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"syscall"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Exception vector limits on amd64. Vectors 0 through 31 are
// architecturally defined; the rest of the 256 IDT entries are
// available for interrupts.
const (
	maxExceptionVector = 31
	maxInterruptVector = 255
)

// X2APICState is whether a vCPU's local APIC presents the xAPIC or the
// x2APIC interface to the guest.
type X2APICState int

const (
	// X2APICDisabled leaves the local APIC in xAPIC mode, reached
	// through its MMIO window.
	X2APICDisabled X2APICState = vmmapi.X2APICDisabled

	// X2APICEnabled puts the local APIC in x2APIC mode, reached
	// through model specific registers.
	X2APICEnabled X2APICState = vmmapi.X2APICEnabled
)

// String renders the state.
func (s X2APICState) String() string {
	switch s {
	case X2APICDisabled:
		return "disabled"
	case X2APICEnabled:
		return "enabled"
	default:
		return fmt.Sprintf("x2apic(%d)", int(s))
	}
}

// TriggerMode is whether an interrupt line is edge or level triggered.
type TriggerMode int

const (
	// TriggerEdge raises the interrupt on a transition.
	TriggerEdge TriggerMode = vmmapi.TriggerEdge

	// TriggerLevel keeps the interrupt asserted while the line is
	// held, until the device deasserts it.
	TriggerLevel TriggerMode = vmmapi.TriggerLevel
)

// String renders the mode.
func (m TriggerMode) String() string {
	switch m {
	case TriggerEdge:
		return "edge"
	case TriggerLevel:
		return "level"
	default:
		return fmt.Sprintf("trigger(%d)", int(m))
	}
}

// Bits of the interrupt information words that IntInfo reports and
// SetIntInfo accepts. An entry describes one event pending delivery to
// a vCPU: its vector, its type, and whether it carries an error code.
const (
	// IntInfoValid marks the word as describing a real event. A word
	// without it is empty.
	IntInfoValid uint64 = vmmapi.IntInfoValid

	// IntInfoDeliverErrcode marks the event as carrying an error code
	// to push on the guest's stack.
	IntInfoDeliverErrcode uint64 = vmmapi.IntInfoDelErrcode

	// IntInfoTypeMask selects the event type bits.
	IntInfoTypeMask uint64 = vmmapi.IntInfoTypeMask

	// IntInfoHWInterrupt is an external interrupt.
	IntInfoHWInterrupt uint64 = vmmapi.IntInfoHWIntr

	// IntInfoNMI is a non-maskable interrupt.
	IntInfoNMI uint64 = vmmapi.IntInfoNMI

	// IntInfoHWException is a processor exception.
	IntInfoHWException uint64 = vmmapi.IntInfoHWException

	// IntInfoSWInterrupt is a software interrupt from an INT
	// instruction.
	IntInfoSWInterrupt uint64 = vmmapi.IntInfoSWIntr
)

// IntInfoVector extracts the vector from an interrupt information word.
func IntInfoVector(info uint64) int { return int(info & 0xff) }

// InjectNMI delivers a non-maskable interrupt to the vCPU.
func (cpu *VCPU) InjectNMI() error {
	return cpu.simple("vm_inject_nmi", (*vmmapi.VCPU).InjectNMI)
}

// InjectException delivers an exception to the vCPU.
//
// Pass a non-nil errcode for the exceptions that carry one, such as #PF
// or #GP; nil means no error code is pushed. When restartInstruction is
// set the guest re-executes the instruction that faulted, which is what
// a fault, as opposed to a trap, calls for.
func (cpu *VCPU) InjectException(vector int, errcode *uint32, restartInstruction bool) error {
	const op = "vm_inject_exception"

	if vector < 0 || vector > maxExceptionVector {
		return fmt.Errorf("vmm: exception vector %d is not 0 to %d: %w",
			vector, maxExceptionVector, ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	var code uint32
	valid := errcode != nil
	if valid {
		code = *errcode
	}

	return cpu.err(op, h.InjectException(vector, valid, code,
		restartInstruction))
}

// LAPICIRQ asserts a vector on the vCPU's local APIC, as a device
// signalling through the APIC would.
func (cpu *VCPU) LAPICIRQ(vector int) error {
	const op = "vm_lapic_irq"

	if err := checkInterruptVector(vector); err != nil {
		return err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.LAPICIRQ(vector))
}

// LAPICLocalIRQ asserts a vector on one of the local APIC's own local
// interrupt pins, LINT0 or LINT1, rather than on an external line.
func (cpu *VCPU) LAPICLocalIRQ(vector int) error {
	const op = "vm_lapic_local_irq"

	if err := checkInterruptVector(vector); err != nil {
		return err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.LAPICLocalIRQ(vector))
}

// LAPICMSI delivers a message signalled interrupt to the VM.
//
// The address and message words are the ones a PCI device would write,
// and the local APICs decide from them which vCPU receives what.
func (vm *VM) LAPICMSI(addr, msg uint64) error {
	const op = "vm_lapic_msi"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.LAPICMSI(addr, msg))
}

// IOAPICAssert raises an I/O APIC input line.
func (vm *VM) IOAPICAssert(irq int) error {
	return vm.ioapic("vm_ioapic_assert_irq", irq,
		(*vmmapi.Context).IOAPICAssertIRQ)
}

// IOAPICDeassert lowers an I/O APIC input line.
func (vm *VM) IOAPICDeassert(irq int) error {
	return vm.ioapic("vm_ioapic_deassert_irq", irq,
		(*vmmapi.Context).IOAPICDeassertIRQ)
}

// IOAPICPulse raises and immediately lowers an I/O APIC input line,
// which is what an edge-triggered device does.
func (vm *VM) IOAPICPulse(irq int) error {
	return vm.ioapic("vm_ioapic_pulse_irq", irq,
		(*vmmapi.Context).IOAPICPulseIRQ)
}

// ioapic runs one of the I/O APIC pin operations.
func (vm *VM) ioapic(op string, irq int, fn func(*vmmapi.Context, int) syscall.Errno) error {
	if irq < 0 {
		return fmt.Errorf("vmm: irq %d is negative: %w", irq,
			ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, fn(ctx, irq))
}

// IOAPICPinCount returns how many input pins the VM's I/O APIC has,
// which bounds the irq accepted by IOAPICAssert and its siblings.
func (vm *VM) IOAPICPinCount() (int, error) {
	const op = "vm_ioapic_pincount"

	ctx, release, err := vm.context(op)
	if err != nil {
		return 0, err
	}
	defer release()

	count, errno := ctx.IOAPICPinCount()
	if errno != 0 {
		return 0, vm.err(op, errno)
	}
	return count, nil
}

// ISAAssert raises a legacy 8259 PIC interrupt.
//
// A PC wires the same device to both the PIC and an I/O APIC pin, so
// ioapicIRQ names that pin and both controllers see the interrupt. Pass
// a negative ioapicIRQ to drive the PIC alone.
func (vm *VM) ISAAssert(atpicIRQ, ioapicIRQ int) error {
	return vm.isa("vm_isa_assert_irq", atpicIRQ, ioapicIRQ,
		(*vmmapi.Context).ISAAssertIRQ)
}

// ISADeassert lowers a legacy PIC interrupt.
func (vm *VM) ISADeassert(atpicIRQ, ioapicIRQ int) error {
	return vm.isa("vm_isa_deassert_irq", atpicIRQ, ioapicIRQ,
		(*vmmapi.Context).ISADeassertIRQ)
}

// ISAPulse raises and immediately lowers a legacy PIC interrupt.
func (vm *VM) ISAPulse(atpicIRQ, ioapicIRQ int) error {
	return vm.isa("vm_isa_pulse_irq", atpicIRQ, ioapicIRQ,
		(*vmmapi.Context).ISAPulseIRQ)
}

// isa runs one of the legacy PIC operations.
func (vm *VM) isa(op string, atpicIRQ, ioapicIRQ int, fn func(*vmmapi.Context, int, int) syscall.Errno) error {
	if atpicIRQ < 0 || atpicIRQ > 15 {
		return fmt.Errorf("vmm: atpic irq %d is not 0 to 15: %w",
			atpicIRQ, ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, fn(ctx, atpicIRQ, ioapicIRQ))
}

// ISASetIRQTrigger chooses edge or level triggering for a legacy PIC
// interrupt line.
func (vm *VM) ISASetIRQTrigger(atpicIRQ int, trigger TriggerMode) error {
	const op = "vm_isa_set_irq_trigger"

	if atpicIRQ < 0 || atpicIRQ > 15 {
		return fmt.Errorf("vmm: atpic irq %d is not 0 to 15: %w",
			atpicIRQ, ErrInvalidArgument)
	}
	switch trigger {
	case TriggerEdge, TriggerLevel:
	default:
		return fmt.Errorf("vmm: unknown trigger mode %d: %w",
			int(trigger), ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.ISASetIRQTrigger(atpicIRQ, int(trigger)))
}

// IntInfo returns the two interrupt information words for the vCPU.
//
// The first describes an event staged for delivery; the second describes
// the event that was being delivered when the last VM exit interrupted
// it, which has to be re-staged for the guest to see it. Both are
// decoded with the IntInfo constants and IntInfoVector.
func (cpu *VCPU) IntInfo() (pending, interrupted uint64, err error) {
	const op = "vm_get_intinfo"

	h, release, err := cpu.handle(op)
	if err != nil {
		return 0, 0, err
	}
	defer release()

	i1, i2, errno := h.IntInfo()
	if errno != 0 {
		return 0, 0, cpu.err(op, errno)
	}
	return i1, i2, nil
}

// SetIntInfo stages an event for delivery on the vCPU's next entry into
// the guest.
//
// This is how an interrupted delivery is resumed: pass back the second
// word IntInfo returned.
func (cpu *VCPU) SetIntInfo(info uint64) error {
	const op = "vm_set_intinfo"

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.SetIntInfo(info))
}

// X2APICState returns whether the vCPU's local APIC is in x2APIC mode.
func (cpu *VCPU) X2APICState() (X2APICState, error) {
	const op = "vm_get_x2apic_state"

	h, release, err := cpu.handle(op)
	if err != nil {
		return 0, err
	}
	defer release()

	state, errno := h.X2APICState()
	if errno != 0 {
		return 0, cpu.err(op, errno)
	}
	return X2APICState(state), nil
}

// SetX2APICState puts the vCPU's local APIC into or out of x2APIC mode.
func (cpu *VCPU) SetX2APICState(state X2APICState) error {
	const op = "vm_set_x2apic_state"

	if state < 0 || int(state) >= vmmapi.X2APICLast {
		return fmt.Errorf("vmm: x2apic state %d is out of range: %w",
			int(state), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.SetX2APICState(int(state)))
}

// HPETCapabilities returns the capability word of the VM's high
// precision event timer.
func (vm *VM) HPETCapabilities() (uint32, error) {
	const op = "vm_get_hpet_capabilities"

	ctx, release, err := vm.context(op)
	if err != nil {
		return 0, err
	}
	defer release()

	caps, errno := ctx.HPETCapabilities()
	if errno != 0 {
		return 0, vm.err(op, errno)
	}
	return caps, nil
}

// APICIDToVCPU maps a local APIC id to the vCPU that owns it.
func (vm *VM) APICIDToVCPU(apicid int) (int, error) {
	const op = "vm_apicid2vcpu"

	if apicid < 0 {
		return 0, fmt.Errorf("vmm: apic id %d is negative: %w", apicid,
			ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return 0, err
	}
	defer release()

	id, errno := ctx.APICIDToVCPU(apicid)
	if errno != 0 {
		return 0, vm.err(op, errno)
	}
	return id, nil
}

// KernelEmulatedDevice reads or writes a device the kernel models
// itself, such as the local APIC, the I/O APIC or the HPET, at a guest
// physical address.
//
// This is the companion to ExitInstEmul: when an emulated access lands
// on one of those devices, this performs it without the caller having to
// know how the device works. size is the access width in bytes.
func (cpu *VCPU) KernelEmulatedDevice(gpa GuestAddress, write bool, size int, value uint64) (uint64, error) {
	const op = "vm_readwrite_kernemu_device"

	switch size {
	case 1, 2, 4, 8:
	default:
		return 0, fmt.Errorf("vmm: access size %d is not 1, 2, 4 or 8: %w",
			size, ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return 0, err
	}
	defer release()

	v := value
	if errno := h.ReadWriteKernemuDevice(uint64(gpa), write, size, &v); errno != 0 {
		return 0, cpu.err(op, errno)
	}
	return v, nil
}

// checkInterruptVector rejects a vector outside the IDT.
func checkInterruptVector(vector int) error {
	if vector < 0 || vector > maxInterruptVector {
		return fmt.Errorf("vmm: interrupt vector %d is not 0 to %d: %w",
			vector, maxInterruptVector, ErrInvalidArgument)
	}
	return nil
}
