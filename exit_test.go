//go:build freebsd && amd64

package vmm

import (
	"reflect"
	"testing"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Exit decoding is tested against synthetic bridge output, so it runs
// without a kernel. What it has to get right is picking the one payload
// the exit code names and leaving the rest nil: the C union holds
// whatever the previous exit left in it, so reading the wrong member
// would report stale data as if it were current.

func TestDecodeIOExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:       uint32(ExitInOut),
		RIP:        0x7c00,
		InstLength: 1,
		InOutPort:  0x80,
		InOutBytes: 1,
		InOutIn:    false,
		InOutEAX:   0x42,
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.Code != ExitInOut {
		t.Errorf("Code = %v, want %v", exit.Code, ExitInOut)
	}
	if exit.RIP != 0x7c00 {
		t.Errorf("RIP = %#x, want 0x7c00", exit.RIP)
	}
	if exit.InstLength != 1 {
		t.Errorf("InstLength = %d, want 1", exit.InstLength)
	}
	if exit.IO == nil {
		t.Fatal("IO payload is nil")
	}
	want := IOExit{Port: 0x80, Direction: IOOut, Size: 1, Value: 0x42}
	if *exit.IO != want {
		t.Errorf("IO = %+v, want %+v", *exit.IO, want)
	}
	if got, w := exit.IO.String(), "out 0x80 size=1 value=0x42"; got != w {
		t.Errorf("IO.String() = %q, want %q", got, w)
	}
}

func TestDecodeIODirection(t *testing.T) {
	for _, in := range []bool{true, false} {
		raw := &vmmapi.RawExit{Code: uint32(ExitInOut), InOutIn: in}
		exit := decodeExit(raw, CPUSet{})

		want := IOOut
		if in {
			want = IOIn
		}
		if exit.IO.Direction != want {
			t.Errorf("InOutIn=%v decoded to %v, want %v", in,
				exit.IO.Direction, want)
		}
	}
}

func TestDecodeStringIOExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:          uint32(ExitInOutStr),
		InOutPort:     0x3f8,
		InOutBytes:    1,
		InOutIn:       true,
		InOutString:   true,
		InOutRep:      true,
		StrIndex:      0x2000,
		StrCount:      16,
		StrAddrSize:   4,
		StrSegName:    int32(RegES),
		StrSegBase:    0x1000,
		StrSegLimit:   0xffff,
		StrSegAccess:  0x93,
		StrRFlags:     0x202,
		StrCR0:        0x1,
		PagingCR3:     0x3000,
		PagingCPL:     3,
		PagingCPUMode: int32(CPUModeProtected),
		PagingMode:    int32(PagingMode32),
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.StringIO == nil {
		t.Fatal("StringIO payload is nil")
	}
	if exit.IO != nil {
		t.Error("a string I/O exit also set the plain IO payload")
	}

	s := exit.StringIO
	if s.Port != 0x3f8 || s.Direction != IOIn || s.Size != 1 {
		t.Errorf("embedded IOExit = %+v", s.IOExit)
	}
	if !s.Rep {
		t.Error("Rep = false, want true")
	}
	if s.Index != 0x2000 || s.Count != 16 || s.AddrSize != 4 {
		t.Errorf("Index/Count/AddrSize = %#x/%d/%d", s.Index, s.Count, s.AddrSize)
	}
	if s.SegmentRegister != RegES {
		t.Errorf("SegmentRegister = %v, want %v", s.SegmentRegister, RegES)
	}
	wantSeg := SegmentDescriptor{Base: 0x1000, Limit: 0xffff, Access: 0x93}
	if s.Segment != wantSeg {
		t.Errorf("Segment = %v, want %v", s.Segment, wantSeg)
	}
	wantPaging := PagingState{
		CR3: 0x3000, CPL: 3,
		CPUMode: CPUModeProtected, PagingMode: PagingMode32,
	}
	if s.Paging != wantPaging {
		t.Errorf("Paging = %v, want %v", s.Paging, wantPaging)
	}
}

func TestDecodePagingExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:      uint32(ExitPaging),
		GPA:       0xc0000000,
		FaultType: int32(ProtRead | ProtWrite),
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.Paging == nil {
		t.Fatal("Paging payload is nil")
	}
	if exit.Paging.GuestAddress != 0xc0000000 {
		t.Errorf("GuestAddress = %v, want 0xc0000000", exit.Paging.GuestAddress)
	}
	if exit.Paging.FaultType != ProtRead|ProtWrite {
		t.Errorf("FaultType = %v, want rw-", exit.Paging.FaultType)
	}
	if got, w := exit.Paging.String(), "0xc0000000 rw-"; got != w {
		t.Errorf("String() = %q, want %q", got, w)
	}
}

func TestDecodeInstEmulExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:         uint32(ExitInstEmul),
		GPA:          0xfeb00000,
		GLA:          0xffff8000feb00000,
		CSBase:       0,
		CSD:          1,
		InstNumValid: 3,
	}
	raw.Inst[0] = 0x8b
	raw.Inst[1] = 0x45
	raw.Inst[2] = 0x08
	raw.Inst[3] = 0xcc // beyond num_valid, must not be reported

	exit := decodeExit(raw, CPUSet{})

	if exit.InstEmul == nil {
		t.Fatal("InstEmul payload is nil")
	}
	e := exit.InstEmul
	if e.GuestAddress != 0xfeb00000 {
		t.Errorf("GuestAddress = %v", e.GuestAddress)
	}
	if e.GuestLinearAddress != 0xffff8000feb00000 {
		t.Errorf("GuestLinearAddress = %#x", e.GuestLinearAddress)
	}
	if e.CSDefaultSize != 1 {
		t.Errorf("CSDefaultSize = %d, want 1", e.CSDefaultSize)
	}
	want := []byte{0x8b, 0x45, 0x08}
	if !reflect.DeepEqual(e.Instruction, want) {
		t.Errorf("Instruction = %#v, want %#v", e.Instruction, want)
	}
}

// TestDecodeInstEmulClampsLength is the guard for a guest-influenced
// length being used as a slice bound. num_valid comes from the kernel's
// instruction fetch and must not be trusted past the buffer.
func TestDecodeInstEmulClampsLength(t *testing.T) {
	for _, n := range []uint8{0, 15, 16, 200, 255} {
		raw := &vmmapi.RawExit{
			Code:         uint32(ExitInstEmul),
			InstNumValid: n,
		}

		exit := decodeExit(raw, CPUSet{})
		if exit.InstEmul == nil {
			t.Fatalf("num_valid=%d: InstEmul payload is nil", n)
		}
		if got := len(exit.InstEmul.Instruction); got > vmmapi.InstSize {
			t.Errorf("num_valid=%d: Instruction has %d bytes, buffer holds %d",
				n, got, vmmapi.InstSize)
		}
	}
}

// TestDecodeInstEmulCopiesInstruction checks the bytes are copied rather
// than aliased, since the bridge buffer is reused by RunLoop.
func TestDecodeInstEmulCopiesInstruction(t *testing.T) {
	raw := &vmmapi.RawExit{Code: uint32(ExitInstEmul), InstNumValid: 2}
	raw.Inst[0] = 0xaa
	raw.Inst[1] = 0xbb

	exit := decodeExit(raw, CPUSet{})

	raw.Inst[0] = 0xff
	if exit.InstEmul.Instruction[0] != 0xaa {
		t.Error("Instruction aliases the bridge buffer instead of copying it")
	}
}

func TestDecodeMSRExit(t *testing.T) {
	tests := []struct {
		code      ExitCode
		wantWrite bool
	}{
		{ExitRDMSR, false},
		{ExitWRMSR, true},
	}

	for _, tc := range tests {
		raw := &vmmapi.RawExit{
			Code:    uint32(tc.code),
			MSRCode: 0x1b,
			MSRWVal: 0xfee00900,
		}

		exit := decodeExit(raw, CPUSet{})
		if exit.MSR == nil {
			t.Fatalf("%v: MSR payload is nil", tc.code)
		}
		if exit.MSR.MSR != 0x1b {
			t.Errorf("%v: MSR = %#x, want 0x1b", tc.code, exit.MSR.MSR)
		}
		if exit.MSR.Write != tc.wantWrite {
			t.Errorf("%v: Write = %v, want %v", tc.code,
				exit.MSR.Write, tc.wantWrite)
		}
	}
}

func TestDecodeHLTExit(t *testing.T) {
	const rflagsIF = 1 << 9

	tests := []struct {
		rflags      uint64
		wantEnabled bool
	}{
		{rflagsIF, true},
		{0x202, true},
		{0, false},
		{0x2, false},
	}

	for _, tc := range tests {
		raw := &vmmapi.RawExit{
			Code:          uint32(ExitHLT),
			HLTRFlags:     tc.rflags,
			HLTIntrStatus: 0,
		}

		exit := decodeExit(raw, CPUSet{})
		if exit.HLT == nil {
			t.Fatal("HLT payload is nil")
		}
		if got := exit.HLT.InterruptsEnabled(); got != tc.wantEnabled {
			t.Errorf("rflags=%#x: InterruptsEnabled() = %v, want %v",
				tc.rflags, got, tc.wantEnabled)
		}
	}
}

func TestDecodeIPIExit(t *testing.T) {
	dest := CPUSet{words: []uint64{0b1110}}
	raw := &vmmapi.RawExit{
		Code:      uint32(ExitIPI),
		IPIMode:   0,
		IPIVector: 0xef,
	}

	exit := decodeExit(raw, dest)

	if exit.IPI == nil {
		t.Fatal("IPI payload is nil")
	}
	if exit.IPI.Vector != 0xef {
		t.Errorf("Vector = %#x, want 0xef", exit.IPI.Vector)
	}
	if got := exit.IPI.Destination.IDs(); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("Destination = %v, want 1,2,3", got)
	}
	if got, w := exit.IPI.String(), "mode=0 vector=0xef -> 1,2,3"; got != w {
		t.Errorf("String() = %q, want %q", got, w)
	}
}

func TestDecodeSuspendedExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:       uint32(ExitSuspended),
		SuspendHow: int32(SuspendPowerOff),
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.Suspended == nil {
		t.Fatal("Suspended payload is nil")
	}
	if exit.Suspended.How != SuspendPowerOff {
		t.Errorf("How = %v, want %v", exit.Suspended.How, SuspendPowerOff)
	}
}

func TestDecodeSpinupAPExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:       uint32(ExitSpinupAP),
		SpinupVCPU: 2,
		SpinupRIP:  0x8000,
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.SpinupAP == nil {
		t.Fatal("SpinupAP payload is nil")
	}
	if exit.SpinupAP.VCPU != 2 || exit.SpinupAP.RIP != 0x8000 {
		t.Errorf("SpinupAP = %+v", *exit.SpinupAP)
	}
}

func TestDecodeTaskSwitchExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:           uint32(ExitTaskSwitch),
		TSTSSSel:       0x28,
		TSExt:          1,
		TSErrcode:      0xd,
		TSErrcodeValid: 1,
		TSReason:       int32(TaskSwitchIDTGate),
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.TaskSwitch == nil {
		t.Fatal("TaskSwitch payload is nil")
	}
	ts := exit.TaskSwitch
	if ts.TSSSelector != 0x28 || !ts.External || ts.ErrorCode != 0xd ||
		!ts.ErrorCodeValid || ts.Reason != TaskSwitchIDTGate {
		t.Errorf("TaskSwitch = %+v", *ts)
	}
	if got, w := ts.Reason.String(), "idt_gate"; got != w {
		t.Errorf("Reason.String() = %q, want %q", got, w)
	}
}

func TestDecodeDebugExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:                uint32(ExitDB),
		DebugTraceTrap:      1,
		DebugPushfIntercept: 0,
		DebugTFShadowVal:    1,
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.Debug == nil {
		t.Fatal("Debug payload is nil")
	}
	if !exit.Debug.TraceTrap || exit.Debug.PushfIntercept ||
		exit.Debug.TFShadowValue != 1 {
		t.Errorf("Debug = %+v", *exit.Debug)
	}
}

func TestDecodeBreakpointExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:          uint32(ExitBPT),
		BPTInstLength: 1,
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.Breakpoint == nil {
		t.Fatal("Breakpoint payload is nil")
	}
	if exit.Breakpoint.InstLength != 1 {
		t.Errorf("InstLength = %d, want 1", exit.Breakpoint.InstLength)
	}
}

func TestDecodeVMXExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:                 uint32(ExitVMX),
		VMXStatus:            0,
		VMXExitReason:        0x30,
		VMXExitQualification: 0x182,
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.VMX == nil {
		t.Fatal("VMX payload is nil")
	}
	if got, w := exit.VMX.String(), "reason=0x30 qualification=0x182"; got != w {
		t.Errorf("String() = %q, want %q", got, w)
	}

	// A non-zero status means the instruction fields are what matter.
	raw.VMXStatus = 1
	raw.VMXInstType = 2
	raw.VMXInstError = 7
	exit = decodeExit(raw, CPUSet{})
	if got, w := exit.VMX.String(), "status=1 inst_type=2 inst_error=7"; got != w {
		t.Errorf("String() with status = %q, want %q", got, w)
	}
}

func TestDecodeSVMExit(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:        uint32(ExitSVM),
		SVMExitCode: 0x7b,
	}

	exit := decodeExit(raw, CPUSet{})

	if exit.SVM == nil {
		t.Fatal("SVM payload is nil")
	}
	if exit.SVM.ExitCode != 0x7b {
		t.Errorf("ExitCode = %#x, want 0x7b", exit.SVM.ExitCode)
	}
}

// TestDecodePayloadlessExits covers the reasons that carry no detail.
// They must leave every payload nil rather than reporting whatever the
// union happened to hold.
func TestDecodePayloadlessExits(t *testing.T) {
	codes := []ExitCode{
		ExitBogus, ExitMTrap, ExitPause, ExitRendezvous,
		ExitReqIdle, ExitMonitor, ExitMWait, ExitVMInsn, ExitDebug,
	}

	for _, code := range codes {
		// Fill every raw field, so a decoder reading the wrong
		// member would be caught.
		raw := &vmmapi.RawExit{
			Code:        uint32(code),
			RIP:         0x1000,
			InOutPort:   0xffff,
			GPA:         0xdeadbeef,
			MSRCode:     0xffff,
			HLTRFlags:   0xffff,
			IPIVector:   0xff,
			SuspendHow:  3,
			SVMExitCode: 0xffff,
		}

		exit := decodeExit(raw, CPUSet{})

		if exit.Code != code {
			t.Errorf("%v: Code = %v", code, exit.Code)
		}
		if name, ok := nonNilPayload(exit); ok {
			t.Errorf("%v: payload %s is set, want all nil", code, name)
		}
	}
}

// TestDecodeSetsExactlyOnePayload is the structural invariant the Exit
// documentation promises.
func TestDecodeSetsExactlyOnePayload(t *testing.T) {
	withPayload := map[ExitCode]string{
		ExitInOut:      "IO",
		ExitInOutStr:   "StringIO",
		ExitPaging:     "Paging",
		ExitInstEmul:   "InstEmul",
		ExitRDMSR:      "MSR",
		ExitWRMSR:      "MSR",
		ExitHLT:        "HLT",
		ExitIPI:        "IPI",
		ExitSuspended:  "Suspended",
		ExitSpinupAP:   "SpinupAP",
		ExitIOAPICEOI:  "IOAPICEOI",
		ExitTaskSwitch: "TaskSwitch",
		ExitDB:         "Debug",
		ExitBPT:        "Breakpoint",
		ExitVMX:        "VMX",
		ExitSVM:        "SVM",
	}

	for code, want := range withPayload {
		raw := &vmmapi.RawExit{Code: uint32(code)}
		exit := decodeExit(raw, CPUSet{})

		got, ok := nonNilPayload(exit)
		if !ok {
			t.Errorf("%v: no payload set, want %s", code, want)
			continue
		}
		if got != want {
			t.Errorf("%v: payload %s is set, want %s", code, got, want)
		}
	}
}

// TestDecodeUnknownExitCodes is the forward-compatibility contract: a
// reason a future FreeBSD adds must decode to a usable value rather than
// panicking or being mistaken for a known one.
func TestDecodeUnknownExitCodes(t *testing.T) {
	for code := uint32(0); code < 512; code++ {
		raw := &vmmapi.RawExit{Code: code, RIP: 0x1000}

		exit := decodeExit(raw, CPUSet{})
		if uint32(exit.Code) != code {
			t.Fatalf("code %d decoded to %d", code, uint32(exit.Code))
		}
		// String must always produce something.
		if exit.Code.String() == "" {
			t.Fatalf("code %d has an empty String()", code)
		}
		// And so must the exit's own rendering.
		if exit.String() == "" {
			t.Fatalf("code %d has an empty Exit.String()", code)
		}
	}
}

func TestExitCodeString(t *testing.T) {
	tests := []struct {
		in   ExitCode
		want string
	}{
		{ExitInOut, "inout"},
		{ExitHLT, "hlt"},
		{ExitInstEmul, "inst_emul"},
		{ExitBogus, "bogus"},
		{ExitDB, "db"},
		{ExitCode(9999), "unknown(9999)"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("ExitCode(%d).String() = %q, want %q",
				uint32(tc.in), got, tc.want)
		}
	}
}

func TestExitCodeKnown(t *testing.T) {
	for _, code := range []ExitCode{ExitInOut, ExitHLT, ExitDB, ExitIPI} {
		if !code.Known() {
			t.Errorf("%v.Known() = false, want true", code)
		}
	}
	for _, code := range []ExitCode{ExitCode(9999), ExitCode(100)} {
		if code.Known() {
			t.Errorf("ExitCode(%d).Known() = true, want false", uint32(code))
		}
	}
}

// TestExitCodeSpurious pins down which exits a run loop can simply
// re-enter on. Getting this wrong either loses guest events or spins.
func TestExitCodeSpurious(t *testing.T) {
	spurious := []ExitCode{ExitBogus, ExitReqIdle, ExitRendezvous}
	real := []ExitCode{
		ExitInOut, ExitHLT, ExitPaging, ExitInstEmul, ExitSuspended,
		ExitIPI, ExitDB, ExitBPT, ExitVMX, ExitSVM, ExitCode(9999),
	}

	for _, code := range spurious {
		if !code.Spurious() {
			t.Errorf("%v.Spurious() = false, want true", code)
		}
	}
	for _, code := range real {
		if code.Spurious() {
			t.Errorf("%v.Spurious() = true, want false", code)
		}
	}
}

func TestExitString(t *testing.T) {
	raw := &vmmapi.RawExit{
		Code:       uint32(ExitInOut),
		RIP:        0x7c00,
		InOutPort:  0x80,
		InOutBytes: 1,
		InOutEAX:   0x42,
	}

	exit := decodeExit(raw, CPUSet{})
	want := "inout at rip=0x7c00: out 0x80 size=1 value=0x42"
	if got := exit.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	bare := decodeExit(&vmmapi.RawExit{Code: uint32(ExitBogus), RIP: 0x10}, CPUSet{})
	if got, w := bare.String(), "bogus at rip=0x10"; got != w {
		t.Errorf("String() = %q, want %q", got, w)
	}
}

// nonNilPayload reports which payload an exit carries, if any.
func nonNilPayload(e Exit) (string, bool) {
	switch {
	case e.IO != nil:
		return "IO", true
	case e.StringIO != nil:
		return "StringIO", true
	case e.Paging != nil:
		return "Paging", true
	case e.InstEmul != nil:
		return "InstEmul", true
	case e.MSR != nil:
		return "MSR", true
	case e.HLT != nil:
		return "HLT", true
	case e.IPI != nil:
		return "IPI", true
	case e.Suspended != nil:
		return "Suspended", true
	case e.SpinupAP != nil:
		return "SpinupAP", true
	case e.IOAPICEOI != nil:
		return "IOAPICEOI", true
	case e.TaskSwitch != nil:
		return "TaskSwitch", true
	case e.Debug != nil:
		return "Debug", true
	case e.Breakpoint != nil:
		return "Breakpoint", true
	case e.VMX != nil:
		return "VMX", true
	case e.SVM != nil:
		return "SVM", true
	default:
		return "", false
	}
}

func BenchmarkExitDecode(b *testing.B) {
	raw := &vmmapi.RawExit{
		Code:       uint32(ExitInOut),
		RIP:        0x7c00,
		InOutPort:  0x80,
		InOutBytes: 1,
		InOutEAX:   0x42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decodeExit(raw, CPUSet{})
	}
}

func BenchmarkExitDecodeInstEmul(b *testing.B) {
	raw := &vmmapi.RawExit{
		Code:         uint32(ExitInstEmul),
		GPA:          0xfeb00000,
		InstNumValid: 4,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decodeExit(raw, CPUSet{})
	}
}
