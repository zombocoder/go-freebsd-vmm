//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// RawExit is one VM exit, flattened out of struct vm_exit.
//
// Which fields are meaningful depends on Code; the rest are zero. The
// public package turns this into typed payloads, so the field soup stops
// here.
type RawExit struct {
	Code       uint32
	InstLength int32
	RIP        uint64

	// INOUT and INOUT_STR.
	InOutEAX    uint32
	InOutPort   uint16
	InOutBytes  uint8
	InOutIn     bool
	InOutString bool
	InOutRep    bool

	// INOUT_STR only.
	StrRFlags    uint64
	StrCR0       uint64
	StrIndex     uint64
	StrCount     uint64
	StrSegBase   uint64
	StrSegLimit  uint32
	StrSegAccess uint32
	StrAddrSize  int32
	StrSegName   int32

	// PAGING and INST_EMUL.
	GPA          uint64
	GLA          uint64
	CSBase       uint64
	FaultType    int32
	CSD          int32
	Inst         [InstSize]byte
	InstNumValid uint8

	// Guest paging state, for INST_EMUL, INOUT_STR, TASK_SWITCH, DB.
	PagingCR3     uint64
	PagingCPL     int32
	PagingCPUMode int32
	PagingMode    int32

	// VMX.
	VMXExitQualification uint64
	VMXExitReason        uint32
	VMXStatus            int32
	VMXInstType          int32
	VMXInstError         int32

	// SVM.
	SVMExitCode  uint64
	SVMExitInfo1 uint64
	SVMExitInfo2 uint64

	// RDMSR and WRMSR.
	MSRWVal uint64
	MSRCode uint32

	// SPINUP_AP.
	SpinupRIP  uint64
	SpinupVCPU int32

	// HLT.
	HLTRFlags     uint64
	HLTIntrStatus uint64

	// IOAPIC_EOI.
	IOAPICVector int32

	// SUSPENDED.
	SuspendHow int32

	// IPI.
	IPIMode   uint32
	IPIVector uint8

	// DB.
	DebugTraceTrap      int32
	DebugPushfIntercept int32
	DebugTFShadowVal    int32

	// BPT.
	BPTInstLength int32

	// TASK_SWITCH.
	TSErrcode      uint32
	TSTSSSel       uint16
	TSExt          int32
	TSErrcodeValid int32
	TSReason       int32
}

// Run enters the guest and returns when it exits.
//
// The call blocks in the kernel for as long as the guest runs. There is
// no way to cancel it from another thread; the vCPU comes back when the
// guest exits, when the VM is suspended, or when a signal arrives on the
// calling thread.
//
// ipiCPUs receives the destination vCPU mask of an IPI exit and must
// have CPUSetWords entries.
func (v *VCPU) Run(out *RawExit, ipiCPUs []uint64) syscall.Errno {
	if !v.valid() || out == nil {
		return syscall.EINVAL
	}
	if len(ipiCPUs) < CPUSetWords() {
		return syscall.EINVAL
	}
	nwords, ok := toCInt(len(ipiCPUs))
	if !ok {
		return syscall.EINVAL
	}

	var exit C.struct_govmm_exit
	r := C.govmm_vm_run(v.ptr, &exit, (*C.uint64_t)(&ipiCPUs[0]), nwords)
	if errno := res(r); errno != 0 {
		return errno
	}

	*out = RawExit{
		Code:       uint32(exit.code),
		InstLength: int32(exit.inst_length),
		RIP:        uint64(exit.rip),

		InOutEAX:    uint32(exit.inout_eax),
		InOutPort:   uint16(exit.inout_port),
		InOutBytes:  uint8(exit.inout_bytes),
		InOutIn:     exit.inout_in != 0,
		InOutString: exit.inout_string != 0,
		InOutRep:    exit.inout_rep != 0,

		StrRFlags:    uint64(exit.str_rflags),
		StrCR0:       uint64(exit.str_cr0),
		StrIndex:     uint64(exit.str_index),
		StrCount:     uint64(exit.str_count),
		StrSegBase:   uint64(exit.str_seg_base),
		StrSegLimit:  uint32(exit.str_seg_limit),
		StrSegAccess: uint32(exit.str_seg_access),
		StrAddrSize:  int32(exit.str_addrsize),
		StrSegName:   int32(exit.str_seg_name),

		GPA:          uint64(exit.gpa),
		GLA:          uint64(exit.gla),
		CSBase:       uint64(exit.cs_base),
		FaultType:    int32(exit.fault_type),
		CSD:          int32(exit.cs_d),
		InstNumValid: uint8(exit.inst_num_valid),

		PagingCR3:     uint64(exit.paging_cr3),
		PagingCPL:     int32(exit.paging_cpl),
		PagingCPUMode: int32(exit.paging_cpu_mode),
		PagingMode:    int32(exit.paging_mode),

		VMXExitQualification: uint64(exit.vmx_exit_qualification),
		VMXExitReason:        uint32(exit.vmx_exit_reason),
		VMXStatus:            int32(exit.vmx_status),
		VMXInstType:          int32(exit.vmx_inst_type),
		VMXInstError:         int32(exit.vmx_inst_error),

		SVMExitCode:  uint64(exit.svm_exitcode),
		SVMExitInfo1: uint64(exit.svm_exitinfo1),
		SVMExitInfo2: uint64(exit.svm_exitinfo2),

		MSRWVal: uint64(exit.msr_wval),
		MSRCode: uint32(exit.msr_code),

		SpinupRIP:  uint64(exit.spinup_rip),
		SpinupVCPU: int32(exit.spinup_vcpu),

		HLTRFlags:     uint64(exit.hlt_rflags),
		HLTIntrStatus: uint64(exit.hlt_intr_status),

		IOAPICVector: int32(exit.ioapic_vector),
		SuspendHow:   int32(exit.suspend_how),

		IPIMode:   uint32(exit.ipi_mode),
		IPIVector: uint8(exit.ipi_vector),

		DebugTraceTrap:      int32(exit.dbg_trace_trap),
		DebugPushfIntercept: int32(exit.dbg_pushf_intercept),
		DebugTFShadowVal:    int32(exit.dbg_tf_shadow_val),

		BPTInstLength: int32(exit.bpt_inst_length),

		TSErrcode:      uint32(exit.ts_errcode),
		TSTSSSel:       uint16(exit.ts_tsssel),
		TSExt:          int32(exit.ts_ext),
		TSErrcodeValid: int32(exit.ts_errcode_valid),
		TSReason:       int32(exit.ts_reason),
	}

	for i := 0; i < InstSize; i++ {
		out.Inst[i] = byte(exit.inst[i])
	}

	return 0
}
