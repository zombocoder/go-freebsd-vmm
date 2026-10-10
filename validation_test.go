//go:build freebsd && amd64

package vmm

import (
	"errors"
	"testing"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Arguments rejected before any kernel call. These checks run on a
// closed VM on purpose: validation has to happen before the state check,
// so a bad argument reports what is wrong with it rather than
// ErrClosed.

func TestSetupMemoryRejectsZeroSize(t *testing.T) {
	vm := closedVM("vm01")

	err := vm.SetupMemory(0, MemoryMapAll)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SetupMemory(0) = %v, want ErrInvalidArgument", err)
	}
}

// TestSetupMemoryRefusesUnsupportedStyles is the guard for the assertion
// inside vm_setup_memory(3). On FreeBSD 14 it asserts the style is
// MemoryMapAll, so passing anything else down would abort the process
// instead of failing.
func TestSetupMemoryRefusesUnsupportedStyles(t *testing.T) {
	if Supports(FeatureSparseMemory) {
		t.Skip("this libvmmapi supports sparse mappings")
	}

	vm := closedVM("vm01")

	for _, style := range []MemoryMappingStyle{MemoryMapNone, MemoryMapSparse} {
		err := vm.SetupMemory(1<<20, style)
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("SetupMemory(_, %s) = %v, want ErrUnsupported", style, err)
		}
	}

	// MemoryMapAll gets past validation and fails on state instead.
	if err := vm.SetupMemory(1<<20, MemoryMapAll); !errors.Is(err, ErrClosed) {
		t.Errorf("SetupMemory(_, all) = %v, want ErrClosed", err)
	}
}

func TestSetMemoryFlagsRejectsUnknownBits(t *testing.T) {
	vm := closedVM("vm01")

	err := vm.SetMemoryFlags(MemoryFlags(0x8000))
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SetMemoryFlags(0x8000) = %v, want ErrInvalidArgument", err)
	}
}

func TestMemoryMappingRejectsZeroSize(t *testing.T) {
	vm := closedVM("vm01")

	if _, err := vm.MapGuestMemory(0, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("MapGuestMemory(_, 0) = %v, want ErrInvalidArgument", err)
	}
	if err := vm.MapMemorySegment(0, 0, 0, 0, ProtRead); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("MapMemorySegment(_, 0) = %v, want ErrInvalidArgument", err)
	}
	if err := vm.UnmapMemorySegment(0, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("UnmapMemorySegment(_, 0) = %v, want ErrInvalidArgument", err)
	}
	if _, err := vm.CreateDeviceMemory(SegmentBootROM, "rom", 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("CreateDeviceMemory(_, 0) = %v, want ErrInvalidArgument", err)
	}
}

// TestCreateDeviceMemoryRefusesSysMem keeps a caller from overwriting
// the system memory segment, which is not device memory.
func TestCreateDeviceMemoryRefusesSysMem(t *testing.T) {
	vm := closedVM("vm01")

	_, err := vm.CreateDeviceMemory(SegmentSysMem, "oops", 1<<20)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("CreateDeviceMemory(SegmentSysMem, ...) = %v, want ErrInvalidArgument", err)
	}
}

func TestVCPURejectsNegativeID(t *testing.T) {
	vm := closedVM("vm01")

	if _, err := vm.VCPU(-1); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("VCPU(-1) = %v, want ErrInvalidArgument", err)
	}
}

func TestSuspendRejectsUnknownReason(t *testing.T) {
	vm := closedVM("vm01")

	for _, how := range []SuspendHow{-1, SuspendHow(vmmapi.SuspendLast), 99} {
		err := vm.Suspend(how)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Suspend(%d) = %v, want ErrInvalidArgument", int(how), err)
		}
	}
}

func TestOpenWithFlagsRejectsUnknownFlags(t *testing.T) {
	_, err := OpenWithFlags("vm01", OpenFlags(1<<20))
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("OpenWithFlags with an unknown flag = %v, want ErrInvalidArgument", err)
	}
}

func TestOpenFlagsAreDistinctBits(t *testing.T) {
	flags := []OpenFlags{OpenCreate, OpenReinit, OpenDestroyOnClose}

	for i, a := range flags {
		if a == 0 {
			t.Errorf("flag %d is zero", i)
		}
		for j, b := range flags {
			if i != j && a&b != 0 {
				t.Errorf("flags %d and %d overlap (%#x, %#x)", i, j, a, b)
			}
		}
	}
	if want := OpenCreate | OpenReinit | OpenDestroyOnClose; allOpenFlags != want {
		t.Errorf("allOpenFlags = %#x, want %#x", allOpenFlags, want)
	}
}

func TestRegisterValidation(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	bad := []Register{-1, Register(vmmapi.RegLast), Register(1 << 20)}
	for _, reg := range bad {
		if _, err := cpu.Register(reg); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Register(%d) = %v, want ErrInvalidArgument", int(reg), err)
		}
		if err := cpu.SetRegister(reg, 0); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("SetRegister(%d) = %v, want ErrInvalidArgument", int(reg), err)
		}
		if _, err := cpu.SegmentDescriptor(reg); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("SegmentDescriptor(%d) = %v, want ErrInvalidArgument", int(reg), err)
		}
	}

	// A valid register gets past validation and fails on state.
	if _, err := cpu.Register(RegRIP); !errors.Is(err, ErrClosed) {
		t.Errorf("Register(RegRIP) = %v, want ErrClosed", err)
	}
}

// TestBulkRegisterValidationNamesTheIndex matters for debugging: one bad
// entry in a long slice should say which one.
func TestBulkRegisterValidationNamesTheIndex(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	regs := []Register{RegRAX, RegRBX, Register(vmmapi.RegLast), RegRCX}
	_, err := cpu.Registers(regs)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Registers with a bad entry = %v, want ErrInvalidArgument", err)
	}
	if got := err.Error(); !contains(got, "index 2") {
		t.Errorf("error %q does not name the offending index", got)
	}

	values := []RegisterValue{{RegRAX, 1}, {Register(-5), 2}}
	if err := cpu.SetRegisters(values); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SetRegisters with a bad entry = %v, want ErrInvalidArgument", err)
	}
}

// TestEmptyBulkRegisterCallsAreNoOps keeps the bulk calls from reaching
// C with a zero count, where &slice[0] would panic.
func TestEmptyBulkRegisterCallsAreNoOps(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	got, err := cpu.Registers(nil)
	if err != nil || got != nil {
		t.Errorf("Registers(nil) = %v, %v, want nil, nil", got, err)
	}
	if err := cpu.SetRegisters(nil); err != nil {
		t.Errorf("SetRegisters(nil) = %v, want nil", err)
	}
}

func TestCapabilityValidation(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 0}

	bad := []Capability{-1, Capability(vmmapi.CapMax), Capability(1 << 20)}
	for _, c := range bad {
		if _, err := cpu.Capability(c); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Capability(%d) = %v, want ErrInvalidArgument", int(c), err)
		}
		if err := cpu.SetCapability(c, 1); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("SetCapability(%d) = %v, want ErrInvalidArgument", int(c), err)
		}
	}

	if _, err := cpu.Capability(CapHaltExit); !errors.Is(err, ErrClosed) {
		t.Errorf("Capability(CapHaltExit) = %v, want ErrClosed", err)
	}
}

// contains is strings.Contains, spelled out to keep the import list of
// this file to the things being tested.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
