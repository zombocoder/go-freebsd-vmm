//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

// Constants mirrored from the FreeBSD headers.
//
// Every value below is taken from C at compile time rather than written
// out by hand, so a renumbering in a future FreeBSD release follows
// automatically instead of silently shifting meaning.

// Guest register identifiers, from enum vm_reg_name.
const (
	RegRAX             = C.VM_REG_GUEST_RAX
	RegRBX             = C.VM_REG_GUEST_RBX
	RegRCX             = C.VM_REG_GUEST_RCX
	RegRDX             = C.VM_REG_GUEST_RDX
	RegRSI             = C.VM_REG_GUEST_RSI
	RegRDI             = C.VM_REG_GUEST_RDI
	RegRBP             = C.VM_REG_GUEST_RBP
	RegR8              = C.VM_REG_GUEST_R8
	RegR9              = C.VM_REG_GUEST_R9
	RegR10             = C.VM_REG_GUEST_R10
	RegR11             = C.VM_REG_GUEST_R11
	RegR12             = C.VM_REG_GUEST_R12
	RegR13             = C.VM_REG_GUEST_R13
	RegR14             = C.VM_REG_GUEST_R14
	RegR15             = C.VM_REG_GUEST_R15
	RegCR0             = C.VM_REG_GUEST_CR0
	RegCR2             = C.VM_REG_GUEST_CR2
	RegCR3             = C.VM_REG_GUEST_CR3
	RegCR4             = C.VM_REG_GUEST_CR4
	RegDR0             = C.VM_REG_GUEST_DR0
	RegDR1             = C.VM_REG_GUEST_DR1
	RegDR2             = C.VM_REG_GUEST_DR2
	RegDR3             = C.VM_REG_GUEST_DR3
	RegDR6             = C.VM_REG_GUEST_DR6
	RegDR7             = C.VM_REG_GUEST_DR7
	RegRSP             = C.VM_REG_GUEST_RSP
	RegRIP             = C.VM_REG_GUEST_RIP
	RegRFLAGS          = C.VM_REG_GUEST_RFLAGS
	RegES              = C.VM_REG_GUEST_ES
	RegCS              = C.VM_REG_GUEST_CS
	RegSS              = C.VM_REG_GUEST_SS
	RegDS              = C.VM_REG_GUEST_DS
	RegFS              = C.VM_REG_GUEST_FS
	RegGS              = C.VM_REG_GUEST_GS
	RegLDTR            = C.VM_REG_GUEST_LDTR
	RegTR              = C.VM_REG_GUEST_TR
	RegIDTR            = C.VM_REG_GUEST_IDTR
	RegGDTR            = C.VM_REG_GUEST_GDTR
	RegEFER            = C.VM_REG_GUEST_EFER
	RegPDPTE0          = C.VM_REG_GUEST_PDPTE0
	RegPDPTE1          = C.VM_REG_GUEST_PDPTE1
	RegPDPTE2          = C.VM_REG_GUEST_PDPTE2
	RegPDPTE3          = C.VM_REG_GUEST_PDPTE3
	RegIntrShadow      = C.VM_REG_GUEST_INTR_SHADOW
	RegEntryInstLength = C.VM_REG_GUEST_ENTRY_INST_LENGTH
	RegFSBase          = C.VM_REG_GUEST_FS_BASE
	RegGSBase          = C.VM_REG_GUEST_GS_BASE
	RegKGSBase         = C.VM_REG_GUEST_KGS_BASE
	RegTPR             = C.VM_REG_GUEST_TPR

	// RegLast is one past the highest valid register on this
	// architecture and this FreeBSD release.
	RegLast = C.VM_REG_LAST
)

// VMM capabilities, from enum vm_cap_type.
const (
	CapHaltExit          = C.VM_CAP_HALT_EXIT
	CapMTrapExit         = C.VM_CAP_MTRAP_EXIT
	CapPauseExit         = C.VM_CAP_PAUSE_EXIT
	CapUnrestrictedGuest = C.VM_CAP_UNRESTRICTED_GUEST
	CapEnableINVPCID     = C.VM_CAP_ENABLE_INVPCID
	CapBPTExit           = C.VM_CAP_BPT_EXIT
	CapRDPID             = C.VM_CAP_RDPID
	CapRDTSCP            = C.VM_CAP_RDTSCP
	CapIPIExit           = C.VM_CAP_IPI_EXIT
	CapMaskHWIntr        = C.VM_CAP_MASK_HWINTR
	CapRFlagsTF          = C.VM_CAP_RFLAGS_TF

	// CapMax is one past the highest capability this FreeBSD release
	// knows about.
	CapMax = C.VM_CAP_MAX
)

// VM suspend reasons, from enum vm_suspend_how.
const (
	SuspendNone        = C.VM_SUSPEND_NONE
	SuspendReset       = C.VM_SUSPEND_RESET
	SuspendPowerOff    = C.VM_SUSPEND_POWEROFF
	SuspendHalt        = C.VM_SUSPEND_HALT
	SuspendTripleFault = C.VM_SUSPEND_TRIPLEFAULT

	// SuspendLast is one past the highest valid reason.
	SuspendLast = C.VM_SUSPEND_LAST
)

// Flags accepted by SetMemFlags.
const (
	MemFlagInCore = C.VM_MEM_F_INCORE
	MemFlagWired  = C.VM_MEM_F_WIRED
)

// Memory segment identifiers. SegSysMem is the segment that
// SetupMemory populates; the rest are available for device memory.
const (
	SegSysMem      = C.VM_SYSMEM
	SegBootROM     = C.VM_BOOTROM
	SegFramebuffer = C.VM_FRAMEBUFFER
	SegPCIROM      = C.VM_PCIROM
)

// Styles of mapping guest memory into the controlling process, from
// enum vm_mmap_style.
const (
	MMapNone   = C.VM_MMAP_NONE
	MMapAll    = C.VM_MMAP_ALL
	MMapSparse = C.VM_MMAP_SPARSE
)

// Memory protection bits, from <sys/mman.h>.
const (
	ProtNone  = C.PROT_NONE
	ProtRead  = C.PROT_READ
	ProtWrite = C.PROT_WRITE
	ProtExec  = C.PROT_EXEC
)

// VM name length limits enforced by devfs, from <machine/vmm.h>.
const (
	MaxNameLen = C.VM_MAX_NAMELEN
	MinNameLen = C.VM_MIN_NAMELEN
)

// SegNameLen is the size of the memory segment name buffer, including
// the terminating NUL.
const SegNameLen = C.VM_MAX_SUFFIXLEN + 1

// HighMemBase is the guest physical address at which libvmmapi places
// the high memory segment when a VM is given more memory than the low
// memory limit.
//
// FreeBSD 14's libvmmapi has no vm_get_highmem_base(); the layout is a
// 4 GiB literal inside vm_setup_memory() and vm_map_gpa(). The value is
// restated here rather than guessed at from a newer header, and
// MemoryMappings reports the authoritative layout from the kernel.
const HighMemBase = 4 << 30

// VM exit reasons, from enum vm_exitcode.
const (
	ExitInOut      = C.VM_EXITCODE_INOUT
	ExitVMX        = C.VM_EXITCODE_VMX
	ExitBogus      = C.VM_EXITCODE_BOGUS
	ExitRDMSR      = C.VM_EXITCODE_RDMSR
	ExitWRMSR      = C.VM_EXITCODE_WRMSR
	ExitHLT        = C.VM_EXITCODE_HLT
	ExitMTrap      = C.VM_EXITCODE_MTRAP
	ExitPause      = C.VM_EXITCODE_PAUSE
	ExitPaging     = C.VM_EXITCODE_PAGING
	ExitInstEmul   = C.VM_EXITCODE_INST_EMUL
	ExitSpinupAP   = C.VM_EXITCODE_SPINUP_AP
	ExitRendezvous = C.VM_EXITCODE_RENDEZVOUS
	ExitIOAPICEOI  = C.VM_EXITCODE_IOAPIC_EOI
	ExitSuspended  = C.VM_EXITCODE_SUSPENDED
	ExitInOutStr   = C.VM_EXITCODE_INOUT_STR
	ExitTaskSwitch = C.VM_EXITCODE_TASK_SWITCH
	ExitMonitor    = C.VM_EXITCODE_MONITOR
	ExitMWait      = C.VM_EXITCODE_MWAIT
	ExitSVM        = C.VM_EXITCODE_SVM
	ExitReqIdle    = C.VM_EXITCODE_REQIDLE
	ExitDebug      = C.VM_EXITCODE_DEBUG
	ExitVMInsn     = C.VM_EXITCODE_VMINSN
	ExitBPT        = C.VM_EXITCODE_BPT
	ExitIPI        = C.VM_EXITCODE_IPI
	ExitDB         = C.VM_EXITCODE_DB

	// ExitMax is one past the highest exit reason this FreeBSD
	// release defines.
	ExitMax = C.VM_EXITCODE_MAX
)

// Guest CPU modes, from enum vm_cpu_mode.
const (
	CPUModeReal          = C.CPU_MODE_REAL
	CPUModeProtected     = C.CPU_MODE_PROTECTED
	CPUModeCompatibility = C.CPU_MODE_COMPATIBILITY
	CPUMode64Bit         = C.CPU_MODE_64BIT
)

// Guest paging modes, from enum vm_paging_mode.
const (
	PagingModeFlat   = C.PAGING_MODE_FLAT
	PagingMode32     = C.PAGING_MODE_32
	PagingModePAE    = C.PAGING_MODE_PAE
	PagingMode64     = C.PAGING_MODE_64
	PagingMode64LA57 = C.PAGING_MODE_64_LA57
)

// Task switch reasons, from enum task_switch_reason.
const (
	TaskSwitchCall    = C.TSR_CALL
	TaskSwitchIRET    = C.TSR_IRET
	TaskSwitchJMP     = C.TSR_JMP
	TaskSwitchIDTGate = C.TSR_IDT_GATE
)

// x2APIC states, from enum x2apic_state.
const (
	X2APICDisabled = C.X2APIC_DISABLED
	X2APICEnabled  = C.X2APIC_ENABLED

	// X2APICLast is one past the highest valid state.
	X2APICLast = C.X2APIC_STATE_LAST
)

// Interrupt trigger modes, from enum vm_intr_trigger.
const (
	TriggerEdge  = C.EDGE_TRIGGER
	TriggerLevel = C.LEVEL_TRIGGER
)

// Interrupt information bits, from the VM_INTINFO_* macros. These
// describe the pending event that Get/SetIntInfo carries.
const (
	IntInfoValid       = C.VM_INTINFO_VALID
	IntInfoDelErrcode  = C.VM_INTINFO_DEL_ERRCODE
	IntInfoTypeMask    = C.VM_INTINFO_TYPE
	IntInfoHWIntr      = C.VM_INTINFO_HWINTR
	IntInfoNMI         = C.VM_INTINFO_NMI
	IntInfoHWException = C.VM_INTINFO_HWEXCEPTION
	IntInfoSWIntr      = C.VM_INTINFO_SWINTR
)

// Kernel structures a snapshot request can target, from
// enum snapshot_req.
const (
	SnapshotIOAPIC  = C.STRUCT_VIOAPIC
	SnapshotVM      = C.STRUCT_VM
	SnapshotLAPIC   = C.STRUCT_VLAPIC
	SnapshotMem     = C.VM_MEM
	SnapshotHPET    = C.STRUCT_VHPET
	SnapshotVMCX    = C.STRUCT_VMCX
	SnapshotATPIC   = C.STRUCT_VATPIC
	SnapshotATPIT   = C.STRUCT_VATPIT
	SnapshotPMTimer = C.STRUCT_VPMTMR
	SnapshotRTC     = C.STRUCT_VRTC
)

// Snapshot directions, from enum vm_snapshot_op.
const (
	SnapshotSave    = C.VM_SNAPSHOT_SAVE
	SnapshotRestore = C.VM_SNAPSHOT_RESTORE
)

// InstSize is the number of instruction bytes an instruction emulation
// exit carries.
const InstSize = C.GOVMM_INST_SIZE
