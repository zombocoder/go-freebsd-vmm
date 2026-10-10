//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// InjectNMI delivers a non-maskable interrupt to the vCPU.
func (v *VCPU) InjectNMI() syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_inject_nmi(v.ptr))
}

// InjectException delivers an exception with the given vector.
func (v *VCPU) InjectException(vector int, errcodeValid bool, errcode uint32, restartInstruction bool) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	cvector, ok := toCInt(vector)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_inject_exception(v.ptr, cvector,
		cbool(errcodeValid), C.uint32_t(errcode),
		cbool(restartInstruction)))
}

// LAPICIRQ asserts a vector on the vCPU's local APIC.
func (v *VCPU) LAPICIRQ(vector int) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	cvector, ok := toCInt(vector)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_lapic_irq(v.ptr, cvector))
}

// LAPICLocalIRQ asserts a vector on one of the local APIC's own local
// interrupt pins.
func (v *VCPU) LAPICLocalIRQ(vector int) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	cvector, ok := toCInt(vector)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_lapic_local_irq(v.ptr, cvector))
}

// LAPICMSI delivers a message signalled interrupt through the local
// APICs of the VM.
func (c *Context) LAPICMSI(addr, msg uint64) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_lapic_msi(c.ptr, C.uint64_t(addr),
		C.uint64_t(msg)))
}

// IOAPICAssertIRQ raises a level on an I/O APIC pin.
func (c *Context) IOAPICAssertIRQ(irq int) syscall.Errno {
	return c.ioapicIRQ(irq, ioapicAssert)
}

// IOAPICDeassertIRQ lowers a level on an I/O APIC pin.
func (c *Context) IOAPICDeassertIRQ(irq int) syscall.Errno {
	return c.ioapicIRQ(irq, ioapicDeassert)
}

// IOAPICPulseIRQ raises and lowers an I/O APIC pin.
func (c *Context) IOAPICPulseIRQ(irq int) syscall.Errno {
	return c.ioapicIRQ(irq, ioapicPulse)
}

// ioapicOp selects which of the three I/O APIC pin operations to run.
type ioapicOp int

const (
	ioapicAssert ioapicOp = iota
	ioapicDeassert
	ioapicPulse
)

func (c *Context) ioapicIRQ(irq int, op ioapicOp) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	cirq, ok := toCInt(irq)
	if !ok {
		return syscall.EINVAL
	}

	switch op {
	case ioapicAssert:
		return res(C.govmm_vm_ioapic_assert_irq(c.ptr, cirq))
	case ioapicDeassert:
		return res(C.govmm_vm_ioapic_deassert_irq(c.ptr, cirq))
	case ioapicPulse:
		return res(C.govmm_vm_ioapic_pulse_irq(c.ptr, cirq))
	default:
		return syscall.EINVAL
	}
}

// IOAPICPinCount returns how many pins the VM's I/O APIC has.
func (c *Context) IOAPICPinCount() (int, syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}

	var count C.int
	if errno := res(C.govmm_vm_ioapic_pincount(c.ptr, &count)); errno != 0 {
		return 0, errno
	}
	return int(count), 0
}

// ISAAssertIRQ raises a legacy PIC interrupt, and the I/O APIC pin it is
// wired to when ioapicIRQ is not negative.
func (c *Context) ISAAssertIRQ(atpicIRQ, ioapicIRQ int) syscall.Errno {
	return c.isaIRQ(atpicIRQ, ioapicIRQ, ioapicAssert)
}

// ISADeassertIRQ lowers a legacy PIC interrupt.
func (c *Context) ISADeassertIRQ(atpicIRQ, ioapicIRQ int) syscall.Errno {
	return c.isaIRQ(atpicIRQ, ioapicIRQ, ioapicDeassert)
}

// ISAPulseIRQ raises and lowers a legacy PIC interrupt.
func (c *Context) ISAPulseIRQ(atpicIRQ, ioapicIRQ int) syscall.Errno {
	return c.isaIRQ(atpicIRQ, ioapicIRQ, ioapicPulse)
}

func (c *Context) isaIRQ(atpicIRQ, ioapicIRQ int, op ioapicOp) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	catpic, ok := toCInt(atpicIRQ)
	if !ok {
		return syscall.EINVAL
	}
	cioapic, ok := toCInt(ioapicIRQ)
	if !ok {
		return syscall.EINVAL
	}

	switch op {
	case ioapicAssert:
		return res(C.govmm_vm_isa_assert_irq(c.ptr, catpic, cioapic))
	case ioapicDeassert:
		return res(C.govmm_vm_isa_deassert_irq(c.ptr, catpic, cioapic))
	case ioapicPulse:
		return res(C.govmm_vm_isa_pulse_irq(c.ptr, catpic, cioapic))
	default:
		return syscall.EINVAL
	}
}

// ISASetIRQTrigger sets edge or level triggering for a legacy PIC
// interrupt.
func (c *Context) ISASetIRQTrigger(atpicIRQ, trigger int) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	catpic, ok := toCInt(atpicIRQ)
	if !ok {
		return syscall.EINVAL
	}
	ctrigger, ok := toCInt(trigger)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_isa_set_irq_trigger(c.ptr, catpic, ctrigger))
}

// IntInfo returns the two words describing the events pending on the
// vCPU: the one being delivered and the one the last exit interrupted.
func (v *VCPU) IntInfo() (info1, info2 uint64, errno syscall.Errno) {
	if !v.valid() {
		return 0, 0, syscall.EINVAL
	}

	var i1, i2 C.uint64_t
	if errno := res(C.govmm_vm_get_intinfo(v.ptr, &i1, &i2)); errno != 0 {
		return 0, 0, errno
	}
	return uint64(i1), uint64(i2), 0
}

// SetIntInfo stages an event for delivery on the next guest entry.
func (v *VCPU) SetIntInfo(info uint64) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_set_intinfo(v.ptr, C.uint64_t(info)))
}

// X2APICState returns whether the vCPU's local APIC is in x2APIC mode.
func (v *VCPU) X2APICState() (int, syscall.Errno) {
	if !v.valid() {
		return 0, syscall.EINVAL
	}

	var state C.int
	if errno := res(C.govmm_vm_get_x2apic_state(v.ptr, &state)); errno != 0 {
		return 0, errno
	}
	return int(state), 0
}

// SetX2APICState puts the vCPU's local APIC into or out of x2APIC mode.
func (v *VCPU) SetX2APICState(state int) syscall.Errno {
	if !v.valid() {
		return syscall.EINVAL
	}
	cstate, ok := toCInt(state)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_set_x2apic_state(v.ptr, cstate))
}

// HPETCapabilities returns the capability word of the VM's HPET.
func (c *Context) HPETCapabilities() (uint32, syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}

	var caps C.uint32_t
	r := C.govmm_vm_get_hpet_capabilities(c.ptr, &caps)
	if errno := res(r); errno != 0 {
		return 0, errno
	}
	return uint32(caps), 0
}

// APICIDToVCPU maps a local APIC id to the vCPU id that owns it.
func (c *Context) APICIDToVCPU(apicid int) (int, syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}
	capicid, ok := toCInt(apicid)
	if !ok {
		return 0, syscall.EINVAL
	}

	r := C.govmm_vm_apicid2vcpu(c.ptr, capicid)
	if r.rc < 0 {
		return 0, errnoOf(r.err)
	}
	return int(r.rc), 0
}

// ReadWriteKernemuDevice reads or writes a device the kernel emulates,
// such as the local APIC or the HPET, at a guest physical address.
func (v *VCPU) ReadWriteKernemuDevice(gpa uint64, write bool, size int, value *uint64) syscall.Errno {
	if !v.valid() || value == nil {
		return syscall.EINVAL
	}
	csize, ok := toCInt(size)
	if !ok {
		return syscall.EINVAL
	}

	cvalue := C.uint64_t(*value)
	r := C.govmm_vm_readwrite_kernemu_device(v.ptr, C.uint64_t(gpa),
		cbool(write), csize, &cvalue)
	if errno := res(r); errno != 0 {
		return errno
	}
	*value = uint64(cvalue)
	return 0
}

// cbool converts a Go bool to the int C uses for one.
func cbool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
