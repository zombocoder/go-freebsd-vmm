//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// ExitCode is the reason a vCPU left the guest.
//
// Values come from FreeBSD's enum vm_exitcode. A release that adds a
// reason this package does not name still decodes: the code keeps its
// numeric value and String reports it, so an unknown exit is something
// to handle rather than something that breaks.
type ExitCode uint32

const (
	// ExitInOut is a port I/O instruction the guest executed.
	ExitInOut ExitCode = vmmapi.ExitInOut

	// ExitVMX is a VT-x VM exit with no better description. The VMX
	// payload carries the hardware exit reason.
	ExitVMX ExitCode = vmmapi.ExitVMX

	// ExitBogus carries no guest event. The vCPU left the guest
	// because the host thread had work pending, such as a signal, and
	// the caller simply enters again.
	ExitBogus ExitCode = vmmapi.ExitBogus

	// ExitRDMSR is a read from a model specific register the kernel
	// does not handle itself.
	ExitRDMSR ExitCode = vmmapi.ExitRDMSR

	// ExitWRMSR is a write to a model specific register the kernel
	// does not handle itself.
	ExitWRMSR ExitCode = vmmapi.ExitWRMSR

	// ExitHLT is the guest halting with CapHaltExit enabled.
	//
	// A guest that halts with interrupts enabled can still be woken,
	// so this is the exit. A guest that halts with them disabled
	// cannot, and vmm(4) treats that as the vCPU parking itself: once
	// every active vCPU has, the VM is halted and the exit is
	// ExitSuspended carrying SuspendHalt instead. A single-vCPU guest
	// executing HLT with interrupts off therefore never reports
	// ExitHLT.
	ExitHLT ExitCode = vmmapi.ExitHLT

	// ExitMTrap is a monitor trap flag event.
	ExitMTrap ExitCode = vmmapi.ExitMTrap

	// ExitPause is a PAUSE instruction with CapPauseExit enabled.
	ExitPause ExitCode = vmmapi.ExitPause

	// ExitPaging is a nested paging fault on a guest physical address
	// the host has not backed.
	ExitPaging ExitCode = vmmapi.ExitPaging

	// ExitInstEmul is an access the kernel wants userspace to
	// emulate, which is how MMIO to an emulated device arrives.
	ExitInstEmul ExitCode = vmmapi.ExitInstEmul

	// ExitSpinupAP is the guest's boot processor starting another
	// vCPU.
	ExitSpinupAP ExitCode = vmmapi.ExitSpinupAP

	// ExitRendezvous is kernel bookkeeping: the vCPU was pulled out
	// so that all vCPUs could meet. Enter again.
	ExitRendezvous ExitCode = vmmapi.ExitRendezvous

	// ExitIOAPICEOI is an end-of-interrupt the I/O APIC needs to see.
	ExitIOAPICEOI ExitCode = vmmapi.ExitIOAPICEOI

	// ExitSuspended is the VM having been suspended. The payload
	// carries the reason.
	ExitSuspended ExitCode = vmmapi.ExitSuspended

	// ExitInOutStr is a string port I/O instruction, INS or OUTS.
	ExitInOutStr ExitCode = vmmapi.ExitInOutStr

	// ExitTaskSwitch is a hardware task switch, which the guest's
	// 32-bit code can still ask for.
	ExitTaskSwitch ExitCode = vmmapi.ExitTaskSwitch

	// ExitMonitor is a MONITOR instruction.
	ExitMonitor ExitCode = vmmapi.ExitMonitor

	// ExitMWait is an MWAIT instruction.
	ExitMWait ExitCode = vmmapi.ExitMWait

	// ExitSVM is an AMD-V VM exit with no better description. The SVM
	// payload carries the hardware exit code.
	ExitSVM ExitCode = vmmapi.ExitSVM

	// ExitReqIdle is kernel bookkeeping: the vCPU was asked to go
	// idle. Enter again.
	ExitReqIdle ExitCode = vmmapi.ExitReqIdle

	// ExitDebug is the vCPU having been stopped for debugging.
	ExitDebug ExitCode = vmmapi.ExitDebug

	// ExitVMInsn is a virtualization instruction the guest is not
	// allowed to execute.
	ExitVMInsn ExitCode = vmmapi.ExitVMInsn

	// ExitBPT is an INT3 breakpoint with CapBPTExit enabled.
	ExitBPT ExitCode = vmmapi.ExitBPT

	// ExitIPI is an inter-processor interrupt with CapIPIExit
	// enabled. The payload carries the destination vCPUs.
	ExitIPI ExitCode = vmmapi.ExitIPI

	// ExitDB is a debug exception, which is how single stepping
	// reports each instruction.
	ExitDB ExitCode = vmmapi.ExitDB
)

// exitNames maps the reasons this package names to a short label.
var exitNames = map[ExitCode]string{
	ExitInOut:      "inout",
	ExitVMX:        "vmx",
	ExitBogus:      "bogus",
	ExitRDMSR:      "rdmsr",
	ExitWRMSR:      "wrmsr",
	ExitHLT:        "hlt",
	ExitMTrap:      "mtrap",
	ExitPause:      "pause",
	ExitPaging:     "paging",
	ExitInstEmul:   "inst_emul",
	ExitSpinupAP:   "spinup_ap",
	ExitRendezvous: "rendezvous",
	ExitIOAPICEOI:  "ioapic_eoi",
	ExitSuspended:  "suspended",
	ExitInOutStr:   "inout_str",
	ExitTaskSwitch: "task_switch",
	ExitMonitor:    "monitor",
	ExitMWait:      "mwait",
	ExitSVM:        "svm",
	ExitReqIdle:    "reqidle",
	ExitDebug:      "debug",
	ExitVMInsn:     "vminsn",
	ExitBPT:        "bpt",
	ExitIPI:        "ipi",
	ExitDB:         "db",
}

// String returns the reason's name, or "unknown(n)" for a reason this
// package does not name.
func (c ExitCode) String() string {
	if name, ok := exitNames[c]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", uint32(c))
}

// Known reports whether this package names the reason. A false answer
// means the running FreeBSD is newer than this package, not that
// anything went wrong.
func (c ExitCode) Known() bool {
	_, ok := exitNames[c]
	return ok
}

// Spurious reports whether an exit is kernel bookkeeping rather than a
// guest event, so that the right response is simply to enter the guest
// again.
//
// This matters more in Go than it does in C. The kernel leaves the guest
// whenever the calling thread has an AST pending, which includes a
// pending signal, and the Go runtime delivers signals to its threads as
// a matter of course: the profiler alone does it a hundred times a
// second. A run loop that treats these as errors will not get far.
func (c ExitCode) Spurious() bool {
	switch c {
	case ExitBogus, ExitReqIdle, ExitRendezvous:
		return true
	default:
		return false
	}
}

// IODirection is which way a port I/O instruction moves data.
type IODirection int

const (
	// IOIn is a read from a port into the guest.
	IOIn IODirection = iota

	// IOOut is a write from the guest to a port.
	IOOut
)

// String renders the direction as "in" or "out".
func (d IODirection) String() string {
	if d == IOIn {
		return "in"
	}
	return "out"
}

// IOExit is a port I/O instruction the guest executed.
//
// For IOOut, Value is what the guest wrote. For IOIn it is not an input:
// the handler supplies the result by writing the low Size bytes of RAX
// before entering the guest again.
type IOExit struct {
	Port      uint16
	Direction IODirection
	Size      uint8
	Value     uint32
}

// String renders the access as "out 0x80 size=1 value=0x42".
func (e IOExit) String() string {
	return fmt.Sprintf("%s %#x size=%d value=%#x",
		e.Direction, e.Port, e.Size, e.Value)
}

// StringIOExit is a string port I/O instruction, INS or OUTS.
//
// The guest is moving Count units of Size bytes between the port and
// memory at Index within the segment described by Segment. REP-prefixed
// instructions report the whole remaining count, so a handler may
// service them in one pass.
type StringIOExit struct {
	IOExit

	// Rep reports whether the instruction carried a REP prefix.
	Rep bool

	// Index is the memory operand, RSI for OUTS or RDI for INS.
	Index uint64

	// Count is the number of units left to transfer: RCX for a
	// REP-prefixed instruction, otherwise 1.
	Count uint64

	// AddrSize is the address size in bytes, 2, 4 or 8.
	AddrSize int

	// SegmentRegister names the segment the memory operand is
	// relative to.
	SegmentRegister Register

	// Segment is that segment's descriptor.
	Segment SegmentDescriptor

	RFlags uint64
	CR0    uint64
	Paging PagingState
}

// PagingExit is a nested paging fault: the guest touched a physical
// address the host has not backed with memory.
type PagingExit struct {
	GuestAddress GuestAddress

	// FaultType is the access that faulted, as the ProtRead,
	// ProtWrite and ProtExec bits.
	FaultType MemoryProtection
}

// String renders the fault as "0xc0000000 rw-".
func (e PagingExit) String() string {
	return fmt.Sprintf("%v %v", e.GuestAddress, e.FaultType)
}

// InstEmulExit is an access the kernel wants userspace to emulate.
//
// This is how MMIO to a device modelled outside the kernel arrives. The
// exit says where the access went and hands over the instruction bytes,
// but not what the access was: working that out means decoding the
// instruction.
//
// That decoding is deliberately not here. FreeBSD's instruction
// emulator lives in the kernel and is compiled into bhyve from
// sys/amd64/vmm/vmm_instruction_emul.c; libvmmapi does not export it,
// and reimplementing an x86 decoder is not this library's business. A
// consumer that needs to service MMIO either decodes Instruction itself
// or uses VCPU.KernelEmulatedDevice for the devices the kernel already
// models.
type InstEmulExit struct {
	GuestAddress       GuestAddress
	GuestLinearAddress uint64

	// CSBase is the code segment base the instruction was fetched
	// relative to.
	CSBase uint64

	// CSDefaultSize is the CS.D bit: 1 for a 32-bit default operand
	// size, 0 for 16-bit.
	CSDefaultSize int

	// Instruction holds the bytes the kernel fetched, which may be
	// empty if it could not read them.
	Instruction []byte

	Paging PagingState
}

// String renders the access as "gpa=0xfeb00000 gla=0xffff8000feb00000 inst=4 bytes".
func (e InstEmulExit) String() string {
	return fmt.Sprintf("gpa=%v gla=%#x inst=%d bytes",
		e.GuestAddress, e.GuestLinearAddress, len(e.Instruction))
}

// MSRExit is an access to a model specific register the kernel does not
// service itself.
//
// For a write, Value is what the guest wrote. For a read it is not an
// input: the handler supplies the result by writing RAX and RDX before
// entering the guest again.
type MSRExit struct {
	// MSR is the register number, the ECX the guest used.
	MSR uint32

	// Write distinguishes WRMSR from RDMSR.
	Write bool

	// Value is the value written, for a write.
	Value uint64
}

// String renders the access as "wrmsr 0x1b = 0xfee00900".
func (e MSRExit) String() string {
	if e.Write {
		return fmt.Sprintf("wrmsr %#x = %#x", e.MSR, e.Value)
	}
	return fmt.Sprintf("rdmsr %#x", e.MSR)
}

// HLTExit is the guest halting.
type HLTExit struct {
	RFlags uint64

	// InterruptStatus is the guest's interrupt status at the halt,
	// which tells a handler whether an interrupt could wake it.
	InterruptStatus uint64
}

// InterruptsEnabled reports whether the guest halted with interrupts
// enabled, meaning an interrupt can still wake it. A guest that halts
// with them disabled is not coming back on its own.
func (e HLTExit) InterruptsEnabled() bool {
	const rflagsIF = 1 << 9
	return e.RFlags&rflagsIF != 0
}

// IPIExit is an inter-processor interrupt the guest sent.
type IPIExit struct {
	// Mode is the APIC delivery mode of the interrupt.
	Mode uint32

	// Vector is the interrupt vector, for delivery modes that carry
	// one.
	Vector uint8

	// Destination is the set of vCPUs the interrupt is aimed at.
	Destination CPUSet
}

// String renders the interrupt as "mode=0 vector=0xef -> 1,2,3".
func (e IPIExit) String() string {
	return fmt.Sprintf("mode=%d vector=%#x -> %v",
		e.Mode, e.Vector, e.Destination)
}

// SuspendedExit is the VM having been suspended.
type SuspendedExit struct {
	How SuspendHow
}

// SpinupAPExit is the guest's boot processor bringing another vCPU up.
//
// The handler is expected to set the named vCPU's RIP and activate it.
type SpinupAPExit struct {
	VCPU int
	RIP  uint64
}

// IOAPICEOIExit is an end-of-interrupt that the I/O APIC needs to see so
// that it can lower a level-triggered line.
type IOAPICEOIExit struct {
	Vector int
}

// TaskSwitchReason is what caused a hardware task switch.
type TaskSwitchReason int

const (
	// TaskSwitchCall is a far CALL through a task gate.
	TaskSwitchCall TaskSwitchReason = vmmapi.TaskSwitchCall

	// TaskSwitchIRET is an IRET returning to a different task.
	TaskSwitchIRET TaskSwitchReason = vmmapi.TaskSwitchIRET

	// TaskSwitchJMP is a far JMP to a task.
	TaskSwitchJMP TaskSwitchReason = vmmapi.TaskSwitchJMP

	// TaskSwitchIDTGate is an interrupt delivered through a task gate
	// in the IDT.
	TaskSwitchIDTGate TaskSwitchReason = vmmapi.TaskSwitchIDTGate
)

// String renders the reason.
func (r TaskSwitchReason) String() string {
	switch r {
	case TaskSwitchCall:
		return "call"
	case TaskSwitchIRET:
		return "iret"
	case TaskSwitchJMP:
		return "jmp"
	case TaskSwitchIDTGate:
		return "idt_gate"
	default:
		return fmt.Sprintf("taskswitch(%d)", int(r))
	}
}

// TaskSwitchExit is a hardware task switch, which the kernel leaves to
// userspace to carry out.
type TaskSwitchExit struct {
	// TSSSelector is the selector of the task being switched to.
	TSSSelector uint16

	// External reports whether an external event caused the switch
	// rather than an instruction.
	External bool

	// ErrorCode is pushed on the new task's stack when
	// ErrorCodeValid is set.
	ErrorCode      uint32
	ErrorCodeValid bool

	Reason TaskSwitchReason
	Paging PagingState
}

// DebugExit is a debug exception, which is how single stepping reports
// each instruction executed.
type DebugExit struct {
	// TraceTrap reports whether the trap flag caused the exception.
	TraceTrap bool

	// PushfIntercept reports whether the exception came from
	// intercepting PUSHF, which the VMM does to hide the trap flag
	// from the guest.
	PushfIntercept bool

	// TFShadowValue is the trap flag value the guest should see,
	// which is not the one actually in RFLAGS while single stepping.
	TFShadowValue int

	Paging PagingState
}

// BreakpointExit is an INT3 the guest executed with CapBPTExit enabled.
type BreakpointExit struct {
	InstLength int
}

// VMXExit is a VT-x VM exit the kernel had no better description for.
//
// When Status is zero the hardware exit reason and qualification are
// meaningful; otherwise the instruction error fields describe why the
// VMX instruction itself failed.
type VMXExit struct {
	Status            int
	ExitReason        uint32
	ExitQualification uint64
	InstType          int
	InstError         int
}

// String renders the exit as "reason=0x30 qualification=0x182".
func (e VMXExit) String() string {
	if e.Status != 0 {
		return fmt.Sprintf("status=%d inst_type=%d inst_error=%d",
			e.Status, e.InstType, e.InstError)
	}
	return fmt.Sprintf("reason=%#x qualification=%#x",
		e.ExitReason, e.ExitQualification)
}

// SVMExit is an AMD-V VM exit the kernel had no better description for.
type SVMExit struct {
	ExitCode  uint64
	ExitInfo1 uint64
	ExitInfo2 uint64
}

// String renders the exit as "exitcode=0x7b info1=0x0 info2=0x0".
func (e SVMExit) String() string {
	return fmt.Sprintf("exitcode=%#x info1=%#x info2=%#x",
		e.ExitCode, e.ExitInfo1, e.ExitInfo2)
}

// Exit describes why a vCPU left the guest and what it was doing.
//
// Code is always set. At most one payload pointer is non-nil, the one
// matching Code; an exit that carries no detail, such as ExitBogus or
// ExitPause, leaves all of them nil. Checking the payload you care about
// for nil is therefore equivalent to checking Code, and switching on
// Code is the way to find out which that is.
//
// Guest-controlled values reach this struct unfiltered. Nothing in it
// may be used as a slice index, an allocation size or a pointer offset
// without being checked first.
type Exit struct {
	// Code is the reason the vCPU exited.
	Code ExitCode

	// RIP is the guest instruction pointer at the exit.
	RIP uint64

	// InstLength is the length of the instruction at RIP, or 0 when
	// the kernel does not know it.
	InstLength int

	IO         *IOExit
	StringIO   *StringIOExit
	Paging     *PagingExit
	InstEmul   *InstEmulExit
	MSR        *MSRExit
	HLT        *HLTExit
	IPI        *IPIExit
	Suspended  *SuspendedExit
	SpinupAP   *SpinupAPExit
	IOAPICEOI  *IOAPICEOIExit
	TaskSwitch *TaskSwitchExit
	Debug      *DebugExit
	Breakpoint *BreakpointExit
	VMX        *VMXExit
	SVM        *SVMExit
}

// String renders the exit as "inout at rip=0x7c00: out 0x80 size=1 value=0x42".
func (e Exit) String() string {
	head := fmt.Sprintf("%s at rip=%#x", e.Code, e.RIP)

	switch {
	case e.IO != nil:
		return head + ": " + e.IO.String()
	case e.StringIO != nil:
		return head + ": " + e.StringIO.String()
	case e.Paging != nil:
		return head + ": " + e.Paging.String()
	case e.InstEmul != nil:
		return head + ": " + e.InstEmul.String()
	case e.MSR != nil:
		return head + ": " + e.MSR.String()
	case e.IPI != nil:
		return head + ": " + e.IPI.String()
	case e.Suspended != nil:
		return head + ": " + e.Suspended.How.String()
	case e.VMX != nil:
		return head + ": " + e.VMX.String()
	case e.SVM != nil:
		return head + ": " + e.SVM.String()
	default:
		return head
	}
}

// decodeExit turns a flattened exit from the bridge into typed form.
//
// Only the payload the exit code names is built; the bridge leaves the
// rest zero, and reading them would mean reporting whatever the previous
// exit left in the union.
func decodeExit(raw *vmmapi.RawExit, ipi CPUSet) Exit {
	exit := Exit{
		Code:       ExitCode(raw.Code),
		RIP:        raw.RIP,
		InstLength: int(raw.InstLength),
	}

	switch exit.Code {
	case ExitInOut:
		io := decodeIO(raw)
		exit.IO = &io

	case ExitInOutStr:
		exit.StringIO = &StringIOExit{
			IOExit:          decodeIO(raw),
			Rep:             raw.InOutRep,
			Index:           raw.StrIndex,
			Count:           raw.StrCount,
			AddrSize:        int(raw.StrAddrSize),
			SegmentRegister: Register(raw.StrSegName),
			Segment: SegmentDescriptor{
				Base:   raw.StrSegBase,
				Limit:  raw.StrSegLimit,
				Access: raw.StrSegAccess,
			},
			RFlags: raw.StrRFlags,
			CR0:    raw.StrCR0,
			Paging: rawPaging(raw),
		}

	case ExitPaging:
		exit.Paging = &PagingExit{
			GuestAddress: GuestAddress(raw.GPA),
			FaultType:    MemoryProtection(raw.FaultType),
		}

	case ExitInstEmul:
		// num_valid is guest-influenced, so clamp it to the buffer
		// rather than trusting it as a length.
		n := int(raw.InstNumValid)
		if n < 0 || n > len(raw.Inst) {
			n = len(raw.Inst)
		}
		inst := make([]byte, n)
		copy(inst, raw.Inst[:n])

		exit.InstEmul = &InstEmulExit{
			GuestAddress:       GuestAddress(raw.GPA),
			GuestLinearAddress: raw.GLA,
			CSBase:             raw.CSBase,
			CSDefaultSize:      int(raw.CSD),
			Instruction:        inst,
			Paging:             rawPaging(raw),
		}

	case ExitRDMSR, ExitWRMSR:
		exit.MSR = &MSRExit{
			MSR:   raw.MSRCode,
			Write: exit.Code == ExitWRMSR,
			Value: raw.MSRWVal,
		}

	case ExitHLT:
		exit.HLT = &HLTExit{
			RFlags:          raw.HLTRFlags,
			InterruptStatus: raw.HLTIntrStatus,
		}

	case ExitIPI:
		exit.IPI = &IPIExit{
			Mode:        raw.IPIMode,
			Vector:      raw.IPIVector,
			Destination: ipi,
		}

	case ExitSuspended:
		exit.Suspended = &SuspendedExit{How: SuspendHow(raw.SuspendHow)}

	case ExitSpinupAP:
		exit.SpinupAP = &SpinupAPExit{
			VCPU: int(raw.SpinupVCPU),
			RIP:  raw.SpinupRIP,
		}

	case ExitIOAPICEOI:
		exit.IOAPICEOI = &IOAPICEOIExit{Vector: int(raw.IOAPICVector)}

	case ExitTaskSwitch:
		exit.TaskSwitch = &TaskSwitchExit{
			TSSSelector:    raw.TSTSSSel,
			External:       raw.TSExt != 0,
			ErrorCode:      raw.TSErrcode,
			ErrorCodeValid: raw.TSErrcodeValid != 0,
			Reason:         TaskSwitchReason(raw.TSReason),
			Paging:         rawPaging(raw),
		}

	case ExitDB:
		exit.Debug = &DebugExit{
			TraceTrap:      raw.DebugTraceTrap != 0,
			PushfIntercept: raw.DebugPushfIntercept != 0,
			TFShadowValue:  int(raw.DebugTFShadowVal),
			Paging:         rawPaging(raw),
		}

	case ExitBPT:
		exit.Breakpoint = &BreakpointExit{
			InstLength: int(raw.BPTInstLength),
		}

	case ExitVMX:
		exit.VMX = &VMXExit{
			Status:            int(raw.VMXStatus),
			ExitReason:        raw.VMXExitReason,
			ExitQualification: raw.VMXExitQualification,
			InstType:          int(raw.VMXInstType),
			InstError:         int(raw.VMXInstError),
		}

	case ExitSVM:
		exit.SVM = &SVMExit{
			ExitCode:  raw.SVMExitCode,
			ExitInfo1: raw.SVMExitInfo1,
			ExitInfo2: raw.SVMExitInfo2,
		}
	}

	return exit
}

// decodeIO builds the part shared by plain and string port I/O.
func decodeIO(raw *vmmapi.RawExit) IOExit {
	direction := IOOut
	if raw.InOutIn {
		direction = IOIn
	}
	return IOExit{
		Port:      raw.InOutPort,
		Direction: direction,
		Size:      raw.InOutBytes,
		Value:     raw.InOutEAX,
	}
}

// rawPaging extracts the paging state an exit reports.
func rawPaging(raw *vmmapi.RawExit) PagingState {
	return pagingFromRaw(raw.PagingCR3, raw.PagingCPL, raw.PagingCPUMode,
		raw.PagingMode)
}
