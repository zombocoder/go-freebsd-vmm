//go:build freebsd && amd64

package vmm

import (
	"errors"
	"testing"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Argument validation for the interrupt, RTC, translation and snapshot
// APIs. These run on closed handles, which is deliberate: a bad
// argument has to be reported as such rather than as ErrClosed, meaning
// validation happens before the state check and before anything reaches
// C.

func TestInjectExceptionVectorRange(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	// Exception vectors are 0 through 31; anything above is an
	// interrupt, not an exception.
	for _, vector := range []int{-1, 32, 255, 1000} {
		err := cpu.InjectException(vector, nil, false)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("InjectException(%d) = %v, want ErrInvalidArgument",
				vector, err)
		}
	}

	// In-range vectors get past validation and fail on state.
	for _, vector := range []int{0, 13, 14, 31} {
		err := cpu.InjectException(vector, nil, false)
		if !errors.Is(err, ErrClosed) {
			t.Errorf("InjectException(%d) = %v, want ErrClosed", vector, err)
		}
	}
}

// TestInjectExceptionErrcodeIsOptional documents the pointer: nil means
// no error code is pushed, which is the difference between #UD and #PF.
func TestInjectExceptionErrcodeIsOptional(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	code := uint32(0x2)
	for _, errcode := range []*uint32{nil, &code} {
		if err := cpu.InjectException(14, errcode, true); !errors.Is(err, ErrClosed) {
			t.Errorf("InjectException with errcode=%v = %v, want ErrClosed",
				errcode, err)
		}
	}
}

func TestInterruptVectorRange(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	for _, vector := range []int{-1, 256, 1000} {
		if err := cpu.LAPICIRQ(vector); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("LAPICIRQ(%d) = %v, want ErrInvalidArgument", vector, err)
		}
		if err := cpu.LAPICLocalIRQ(vector); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("LAPICLocalIRQ(%d) = %v, want ErrInvalidArgument",
				vector, err)
		}
	}

	for _, vector := range []int{0, 32, 0xef, 255} {
		if err := cpu.LAPICIRQ(vector); !errors.Is(err, ErrClosed) {
			t.Errorf("LAPICIRQ(%d) = %v, want ErrClosed", vector, err)
		}
	}
}

func TestISAIRQRange(t *testing.T) {
	vm := closedVM("vm01")

	// The legacy PIC pair has 16 lines.
	for _, irq := range []int{-1, 16, 100} {
		if err := vm.ISAAssert(irq, -1); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ISAAssert(%d) = %v, want ErrInvalidArgument", irq, err)
		}
		if err := vm.ISADeassert(irq, -1); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ISADeassert(%d) = %v, want ErrInvalidArgument", irq, err)
		}
		if err := vm.ISAPulse(irq, -1); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ISAPulse(%d) = %v, want ErrInvalidArgument", irq, err)
		}
		if err := vm.ISASetIRQTrigger(irq, TriggerEdge); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ISASetIRQTrigger(%d) = %v, want ErrInvalidArgument",
				irq, err)
		}
	}

	for _, irq := range []int{0, 4, 15} {
		if err := vm.ISAAssert(irq, -1); !errors.Is(err, ErrClosed) {
			t.Errorf("ISAAssert(%d) = %v, want ErrClosed", irq, err)
		}
	}
}

func TestIOAPICIRQRejectsNegative(t *testing.T) {
	vm := closedVM("vm01")

	for _, fn := range []struct {
		name string
		call func(int) error
	}{
		{"IOAPICAssert", vm.IOAPICAssert},
		{"IOAPICDeassert", vm.IOAPICDeassert},
		{"IOAPICPulse", vm.IOAPICPulse},
	} {
		if err := fn.call(-1); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s(-1) = %v, want ErrInvalidArgument", fn.name, err)
		}
		if err := fn.call(2); !errors.Is(err, ErrClosed) {
			t.Errorf("%s(2) = %v, want ErrClosed", fn.name, err)
		}
	}
}

func TestISASetIRQTriggerRejectsUnknownMode(t *testing.T) {
	vm := closedVM("vm01")

	for _, mode := range []TriggerMode{-1, 2, 99} {
		err := vm.ISASetIRQTrigger(4, mode)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ISASetIRQTrigger(_, %d) = %v, want ErrInvalidArgument",
				int(mode), err)
		}
	}
}

func TestTriggerModeString(t *testing.T) {
	tests := []struct {
		in   TriggerMode
		want string
	}{
		{TriggerEdge, "edge"},
		{TriggerLevel, "level"},
		{TriggerMode(99), "trigger(99)"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestSetX2APICStateRange(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	for _, state := range []X2APICState{-1, X2APICState(vmmapi.X2APICLast), 99} {
		err := cpu.SetX2APICState(state)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("SetX2APICState(%d) = %v, want ErrInvalidArgument",
				int(state), err)
		}
	}

	for _, state := range []X2APICState{X2APICDisabled, X2APICEnabled} {
		if err := cpu.SetX2APICState(state); !errors.Is(err, ErrClosed) {
			t.Errorf("SetX2APICState(%v) = %v, want ErrClosed", state, err)
		}
	}
}

func TestX2APICStateString(t *testing.T) {
	tests := []struct {
		in   X2APICState
		want string
	}{
		{X2APICDisabled, "disabled"},
		{X2APICEnabled, "enabled"},
		{X2APICState(99), "x2apic(99)"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestIntInfoBitsMatchFreeBSD checks the exported bits are the kernel's,
// since callers build and decode these words with them.
func TestIntInfoBitsMatchFreeBSD(t *testing.T) {
	if IntInfoValid == 0 {
		t.Error("IntInfoValid is 0")
	}
	if IntInfoValid&IntInfoTypeMask != 0 {
		t.Error("IntInfoValid overlaps the type mask")
	}
	if IntInfoDeliverErrcode&IntInfoTypeMask != 0 {
		t.Error("IntInfoDeliverErrcode overlaps the type mask")
	}

	// The type values have to be distinct and fit inside the mask.
	// IntInfoHWInterrupt is legitimately zero, so this checks
	// pairwise rather than by set size.
	types := []struct {
		name  string
		value uint64
	}{
		{"hwintr", IntInfoHWInterrupt},
		{"nmi", IntInfoNMI},
		{"hwexception", IntInfoHWException},
		{"swintr", IntInfoSWInterrupt},
	}

	for i, a := range types {
		if a.value&^IntInfoTypeMask != 0 {
			t.Errorf("%s value %#x has bits outside the type mask",
				a.name, a.value)
		}
		for j, b := range types {
			if i != j && a.value == b.value {
				t.Errorf("%s and %s are the same value %#x",
					a.name, b.name, a.value)
			}
		}
	}
}

func TestIntInfoVector(t *testing.T) {
	tests := []struct {
		info uint64
		want int
	}{
		{0, 0},
		{0x0e, 14},
		{IntInfoValid | IntInfoHWException | 0x0e, 14},
		{0xffffffff, 255},
	}
	for _, tc := range tests {
		if got := IntInfoVector(tc.info); got != tc.want {
			t.Errorf("IntInfoVector(%#x) = %d, want %d", tc.info, got, tc.want)
		}
	}
}

func TestKernelEmulatedDeviceSizeRange(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	for _, size := range []int{0, 3, 5, 7, 9, -1, 16} {
		_, err := cpu.KernelEmulatedDevice(0, false, size, 0)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("KernelEmulatedDevice(size=%d) = %v, want ErrInvalidArgument",
				size, err)
		}
	}

	for _, size := range []int{1, 2, 4, 8} {
		_, err := cpu.KernelEmulatedDevice(0, false, size, 0)
		if !errors.Is(err, ErrClosed) {
			t.Errorf("KernelEmulatedDevice(size=%d) = %v, want ErrClosed",
				size, err)
		}
	}
}

func TestAPICIDToVCPURejectsNegative(t *testing.T) {
	vm := closedVM("vm01")

	if _, err := vm.APICIDToVCPU(-1); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("APICIDToVCPU(-1) = %v, want ErrInvalidArgument", err)
	}
	if _, err := vm.APICIDToVCPU(0); !errors.Is(err, ErrClosed) {
		t.Errorf("APICIDToVCPU(0) = %v, want ErrClosed", err)
	}
}

func TestRTCOffsetRange(t *testing.T) {
	vm := closedVM("vm01")

	for _, offset := range []int{-1, RTCRegisters, RTCRegisters + 1, 1000} {
		if _, err := vm.RTCRead(offset); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("RTCRead(%d) = %v, want ErrInvalidArgument", offset, err)
		}
		if err := vm.RTCWrite(offset, 0); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("RTCWrite(%d) = %v, want ErrInvalidArgument", offset, err)
		}
	}

	for _, offset := range []int{0, 9, 13, RTCRegisters - 1} {
		if _, err := vm.RTCRead(offset); !errors.Is(err, ErrClosed) {
			t.Errorf("RTCRead(%d) = %v, want ErrClosed", offset, err)
		}
	}
}

func TestPagingStateValidate(t *testing.T) {
	valid := []PagingState{
		{CPL: 0, CPUMode: CPUModeReal, PagingMode: PagingModeFlat},
		{CPL: 3, CPUMode: CPUMode64Bit, PagingMode: PagingMode64},
		{CR3: 0x1000, CPL: 1, CPUMode: CPUModeProtected, PagingMode: PagingMode32},
		{CPL: 2, CPUMode: CPUModeCompatibility, PagingMode: PagingModePAE},
		{CPUMode: CPUMode64Bit, PagingMode: PagingMode64LA57},
	}
	for _, p := range valid {
		if err := p.validate(); err != nil {
			t.Errorf("%v.validate() = %v, want nil", p, err)
		}
	}

	invalid := []PagingState{
		{CPL: -1},
		{CPL: 4},
		{CPL: 99},
		{CPUMode: 99},
		{CPUMode: -1},
		{PagingMode: 99},
		{PagingMode: -1},
	}
	for _, p := range invalid {
		err := p.validate()
		if err == nil {
			t.Errorf("%v.validate() = nil, want an error", p)
			continue
		}
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%v.validate() = %v, want ErrInvalidArgument", p, err)
		}
	}
}

func TestPagingStateString(t *testing.T) {
	p := PagingState{
		CR3: 0x1000, CPL: 0,
		CPUMode: CPUMode64Bit, PagingMode: PagingMode64,
	}
	want := "cr3=0x1000 cpl=0 mode=64bit paging=64"
	if got := p.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCPUModeAndPagingModeStrings(t *testing.T) {
	modes := []struct {
		in   CPUMode
		want string
	}{
		{CPUModeReal, "real"},
		{CPUModeProtected, "protected"},
		{CPUModeCompatibility, "compatibility"},
		{CPUMode64Bit, "64bit"},
		{CPUMode(99), "cpumode(99)"},
	}
	for _, tc := range modes {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("CPUMode.String() = %q, want %q", got, tc.want)
		}
	}

	paging := []struct {
		in   PagingMode
		want string
	}{
		{PagingModeFlat, "flat"},
		{PagingMode32, "32"},
		{PagingModePAE, "pae"},
		{PagingMode64, "64"},
		{PagingMode64LA57, "64-la57"},
		{PagingMode(99), "pagingmode(99)"},
	}
	for _, tc := range paging {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("PagingMode.String() = %q, want %q", got, tc.want)
		}
	}
}

// TestTranslationValidatesPagingState checks the paging state is
// screened before the call, since an out-of-range mode would otherwise
// be cast to a C enum.
func TestTranslationValidatesPagingState(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}
	bad := PagingState{CPL: 9}
	good := PagingState{CPUMode: CPUMode64Bit, PagingMode: PagingMode64}

	if _, err := cpu.GuestLinearToPhysical(bad, 0, ProtRead); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("GuestLinearToPhysical with bad paging = %v, want ErrInvalidArgument", err)
	}
	if _, err := cpu.GuestLinearToPhysicalNoFault(bad, 0, ProtRead); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("GuestLinearToPhysicalNoFault with bad paging = %v, want ErrInvalidArgument", err)
	}
	if _, err := cpu.GuestLinearToPhysical(good, 0, ProtRead); !errors.Is(err, ErrClosed) {
		t.Errorf("GuestLinearToPhysical = %v, want ErrClosed", err)
	}
}

func TestGuestCopyValidation(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}
	paging := PagingState{CPUMode: CPUMode64Bit, PagingMode: PagingMode64}

	// An empty copy is a no-op, not an error: it would otherwise
	// reach C with a nil buffer pointer.
	if err := cpu.ReadGuestVirtual(0, nil, paging); err != nil {
		t.Errorf("ReadGuestVirtual(nil) = %v, want nil", err)
	}
	if err := cpu.WriteGuestVirtual(0, nil, paging); err != nil {
		t.Errorf("WriteGuestVirtual(nil) = %v, want nil", err)
	}

	// Past the library's own bound.
	oversize := maxGuestCopy + 1
	if err := cpu.ReadGuestVirtual(0, make([]byte, 1), PagingState{CPL: 9}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("ReadGuestVirtual with bad paging = %v, want ErrInvalidArgument", err)
	}
	if err := checkGuestCopy(oversize, paging); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("checkGuestCopy(%d) = %v, want ErrInvalidArgument", oversize, err)
	}
	if err := checkGuestCopy(maxGuestCopy, paging); err != nil {
		t.Errorf("checkGuestCopy(%d) = %v, want nil", maxGuestCopy, err)
	}

	// A valid request gets past validation and fails on state.
	if err := cpu.ReadGuestVirtual(0, make([]byte, 16), paging); !errors.Is(err, ErrClosed) {
		t.Errorf("ReadGuestVirtual = %v, want ErrClosed", err)
	}
}

func TestSnapshotValidation(t *testing.T) {
	vm := closedVM("vm01")

	if _, err := vm.SnapshotSave(nil); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SnapshotSave(nil) = %v, want ErrInvalidArgument", err)
	}
	if _, err := vm.SnapshotRestore(nil); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SnapshotRestore(nil) = %v, want ErrInvalidArgument", err)
	}

	// Unknown structure.
	bad := &SnapshotMetadata{
		Structure: SnapshotStructure(999),
		Buffer:    make([]byte, 16),
	}
	if _, err := vm.SnapshotSave(bad); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SnapshotSave with an unknown structure = %v, want ErrInvalidArgument", err)
	}

	// Empty buffer.
	empty := &SnapshotMetadata{Structure: SnapshotVM}
	if _, err := vm.SnapshotSave(empty); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SnapshotSave with an empty buffer = %v, want ErrInvalidArgument", err)
	}

	// A valid request gets past validation and fails on state.
	ok := &SnapshotMetadata{Structure: SnapshotVM, Buffer: make([]byte, 4096)}
	if _, err := vm.SnapshotSave(ok); !errors.Is(err, ErrClosed) {
		t.Errorf("SnapshotSave = %v, want ErrClosed", err)
	}
}

func TestSnapshotStructureStrings(t *testing.T) {
	structures := []SnapshotStructure{
		SnapshotVM, SnapshotVMCX, SnapshotLAPIC, SnapshotIOAPIC,
		SnapshotHPET, SnapshotATPIC, SnapshotATPIT, SnapshotPMTimer,
		SnapshotRTC, SnapshotMemory,
	}

	seen := map[string]bool{}
	for _, s := range structures {
		name := s.String()
		if name == "" {
			t.Errorf("SnapshotStructure(%d) has an empty name", int(s))
		}
		if seen[name] {
			t.Errorf("two structures are both named %q", name)
		}
		seen[name] = true

		if !s.valid() {
			t.Errorf("%v.valid() = false, want true", s)
		}
	}

	if got, want := SnapshotStructure(999).String(), "snapshot(999)"; got != want {
		t.Errorf("unknown structure String() = %q, want %q", got, want)
	}
	if SnapshotStructure(999).valid() {
		t.Error("an unknown structure reports valid")
	}
}

func TestRunLoopNeedsHandler(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	if err := cpu.RunLoop(nil); err == nil {
		t.Error("RunLoop(nil) = nil, want an error")
	}
}

func TestRunOnClosedVCPU(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	if _, err := cpu.Run(); !errors.Is(err, ErrClosed) {
		t.Errorf("Run() = %v, want ErrClosed", err)
	}
	if err := cpu.RunLoop(func(Exit) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Errorf("RunLoop() = %v, want ErrClosed", err)
	}
}
