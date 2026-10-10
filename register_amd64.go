//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// amd64 guest registers.
//
// Every value comes from FreeBSD's enum vm_reg_name at compile time, so
// the numbering is the kernel's and stays correct if it is ever changed.
const (
	RegRAX Register = vmmapi.RegRAX
	RegRBX Register = vmmapi.RegRBX
	RegRCX Register = vmmapi.RegRCX
	RegRDX Register = vmmapi.RegRDX
	RegRSI Register = vmmapi.RegRSI
	RegRDI Register = vmmapi.RegRDI
	RegRBP Register = vmmapi.RegRBP
	RegRSP Register = vmmapi.RegRSP
	RegR8  Register = vmmapi.RegR8
	RegR9  Register = vmmapi.RegR9
	RegR10 Register = vmmapi.RegR10
	RegR11 Register = vmmapi.RegR11
	RegR12 Register = vmmapi.RegR12
	RegR13 Register = vmmapi.RegR13
	RegR14 Register = vmmapi.RegR14
	RegR15 Register = vmmapi.RegR15

	RegRIP    Register = vmmapi.RegRIP
	RegRFLAGS Register = vmmapi.RegRFLAGS

	RegCR0 Register = vmmapi.RegCR0
	RegCR2 Register = vmmapi.RegCR2
	RegCR3 Register = vmmapi.RegCR3
	RegCR4 Register = vmmapi.RegCR4

	RegDR0 Register = vmmapi.RegDR0
	RegDR1 Register = vmmapi.RegDR1
	RegDR2 Register = vmmapi.RegDR2
	RegDR3 Register = vmmapi.RegDR3
	RegDR6 Register = vmmapi.RegDR6
	RegDR7 Register = vmmapi.RegDR7

	// Segment and descriptor table registers. The descriptor behind
	// these is read and written with SegmentDescriptor, which carries
	// the base, limit and access bits that a plain register value
	// cannot.
	RegES   Register = vmmapi.RegES
	RegCS   Register = vmmapi.RegCS
	RegSS   Register = vmmapi.RegSS
	RegDS   Register = vmmapi.RegDS
	RegFS   Register = vmmapi.RegFS
	RegGS   Register = vmmapi.RegGS
	RegLDTR Register = vmmapi.RegLDTR
	RegTR   Register = vmmapi.RegTR
	RegIDTR Register = vmmapi.RegIDTR
	RegGDTR Register = vmmapi.RegGDTR

	RegEFER    Register = vmmapi.RegEFER
	RegPDPTE0  Register = vmmapi.RegPDPTE0
	RegPDPTE1  Register = vmmapi.RegPDPTE1
	RegPDPTE2  Register = vmmapi.RegPDPTE2
	RegPDPTE3  Register = vmmapi.RegPDPTE3
	RegFSBase  Register = vmmapi.RegFSBase
	RegGSBase  Register = vmmapi.RegGSBase
	RegKGSBase Register = vmmapi.RegKGSBase
	RegTPR     Register = vmmapi.RegTPR

	// RegIntrShadow is non-zero while the guest is in an interrupt
	// shadow, the window after an instruction such as STI or MOV SS
	// in which interrupts stay masked.
	RegIntrShadow Register = vmmapi.RegIntrShadow

	// RegEntryInstLength tells the VMM how long the instruction at
	// RIP is when re-entering the guest.
	RegEntryInstLength Register = vmmapi.RegEntryInstLength
)

// registerNames maps the registers this package names to their
// conventional spelling.
var registerNames = map[Register]string{
	RegRAX: "rax", RegRBX: "rbx", RegRCX: "rcx", RegRDX: "rdx",
	RegRSI: "rsi", RegRDI: "rdi", RegRBP: "rbp", RegRSP: "rsp",
	RegR8: "r8", RegR9: "r9", RegR10: "r10", RegR11: "r11",
	RegR12: "r12", RegR13: "r13", RegR14: "r14", RegR15: "r15",
	RegRIP: "rip", RegRFLAGS: "rflags",
	RegCR0: "cr0", RegCR2: "cr2", RegCR3: "cr3", RegCR4: "cr4",
	RegDR0: "dr0", RegDR1: "dr1", RegDR2: "dr2", RegDR3: "dr3",
	RegDR6: "dr6", RegDR7: "dr7",
	RegES: "es", RegCS: "cs", RegSS: "ss", RegDS: "ds",
	RegFS: "fs", RegGS: "gs", RegLDTR: "ldtr", RegTR: "tr",
	RegIDTR: "idtr", RegGDTR: "gdtr",
	RegEFER:   "efer",
	RegPDPTE0: "pdpte0", RegPDPTE1: "pdpte1",
	RegPDPTE2: "pdpte2", RegPDPTE3: "pdpte3",
	RegFSBase: "fs_base", RegGSBase: "gs_base",
	RegKGSBase: "kgs_base", RegTPR: "tpr",
	RegIntrShadow:      "intr_shadow",
	RegEntryInstLength: "entry_inst_length",
}

// String returns the register's name, or its number if this FreeBSD
// release has a register this package does not name.
func (r Register) String() string {
	if name, ok := registerNames[r]; ok {
		return name
	}
	return fmt.Sprintf("reg(%d)", int(r))
}

// SegmentDescriptor is the hidden part of a segment or descriptor table
// register: the base address, limit and access rights the processor
// actually uses, which the visible selector value does not carry.
type SegmentDescriptor struct {
	Base   uint64
	Limit  uint32
	Access uint32
}

// String renders the descriptor as
// "base=0x0 limit=0xffffffff access=0xc09b".
func (d SegmentDescriptor) String() string {
	return fmt.Sprintf("base=%#x limit=%#x access=%#x",
		d.Base, d.Limit, d.Access)
}

// SegmentDescriptor reads the descriptor behind a segment or descriptor
// table register.
func (cpu *VCPU) SegmentDescriptor(reg Register) (SegmentDescriptor, error) {
	const op = "vm_get_desc"

	if !reg.valid() {
		return SegmentDescriptor{}, fmt.Errorf("vmm: register %d is out of range: %w",
			int(reg), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return SegmentDescriptor{}, err
	}
	defer release()

	base, limit, access, errno := h.GetDesc(int(reg))
	if errno != 0 {
		return SegmentDescriptor{}, cpu.err(op, errno)
	}
	return SegmentDescriptor{Base: base, Limit: limit, Access: access}, nil
}

// SetSegmentDescriptor writes the descriptor behind a segment or
// descriptor table register.
func (cpu *VCPU) SetSegmentDescriptor(reg Register, desc SegmentDescriptor) error {
	const op = "vm_set_desc"

	if !reg.valid() {
		return fmt.Errorf("vmm: register %d is out of range: %w",
			int(reg), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.SetDesc(int(reg), desc.Base, desc.Limit,
		desc.Access))
}
