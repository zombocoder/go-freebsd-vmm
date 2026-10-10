//go:build freebsd && amd64

package vmm

import (
	"errors"
	"testing"
)

func TestPCIDeviceString(t *testing.T) {
	tests := []struct {
		in   PCIDevice
		want string
	}{
		{PCIDevice{}, "0/0/0"},
		{PCIDevice{Bus: 3}, "3/0/0"},
		{PCIDevice{Bus: 3, Slot: 0, Function: 0}, "3/0/0"},
		{PCIDevice{Bus: 0, Slot: 31, Function: 7}, "0/31/7"},
		{PCIDevice{Bus: 255, Slot: 31, Function: 7}, "255/31/7"},
		{PCIDevice{Bus: 1, Slot: 2, Function: 3}, "1/2/3"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestPCIDeviceStringFormatIsStable guards the documented output format.
// Callers put these strings in configuration and logs, so a change to
// colons or hexadecimal would break them.
func TestPCIDeviceStringFormatIsStable(t *testing.T) {
	got := PCIDevice{Bus: 10, Slot: 16, Function: 2}.String()
	if got != "10/16/2" {
		t.Errorf("String() = %q, want decimal bus/slot/function", got)
	}
}

func TestPCIDeviceValidate(t *testing.T) {
	valid := []PCIDevice{
		{},
		{Bus: 255, Slot: 31, Function: 7},
		{Bus: 3, Slot: 0, Function: 0},
	}
	for _, dev := range valid {
		if err := dev.validate(); err != nil {
			t.Errorf("%v.validate() = %v, want nil", dev, err)
		}
	}

	invalid := []PCIDevice{
		{Slot: 32},
		{Slot: 255},
		{Function: 8},
		{Function: 255},
		{Slot: 32, Function: 8},
	}
	for _, dev := range invalid {
		err := dev.validate()
		if err == nil {
			t.Errorf("%v.validate() = nil, want an error", dev)
			continue
		}
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%v.validate() = %v, want ErrInvalidArgument", dev, err)
		}
	}
}

func TestParsePCIDevice(t *testing.T) {
	tests := []struct {
		in   string
		want PCIDevice
	}{
		{"0/0/0", PCIDevice{}},
		{"3/0/0", PCIDevice{Bus: 3}},
		{"255/31/7", PCIDevice{Bus: 255, Slot: 31, Function: 7}},
		{"1/2/3", PCIDevice{Bus: 1, Slot: 2, Function: 3}},
	}
	for _, tc := range tests {
		got, err := ParsePCIDevice(tc.in)
		if err != nil {
			t.Errorf("ParsePCIDevice(%q) = %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParsePCIDevice(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParsePCIDeviceErrors(t *testing.T) {
	bad := []string{
		"", "3", "3/0", "3/0/0/0",
		"256/0/0", // bus out of range
		"0/32/0",  // slot out of range
		"0/0/8",   // function out of range
		"-1/0/0",  // negative
		"3:0:0",   // wrong separator
		"a/b/c",   // not numbers
		"3/0/0 ",  // trailing space
		"0x3/0/0", // hexadecimal
	}
	for _, in := range bad {
		if _, err := ParsePCIDevice(in); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ParsePCIDevice(%q) = %v, want ErrInvalidArgument", in, err)
		}
	}
}

// TestParsePCIDeviceRoundTrip checks the two halves agree, since String
// is the documented input format for Parse.
func TestParsePCIDeviceRoundTrip(t *testing.T) {
	devices := []PCIDevice{
		{}, {Bus: 3}, {Bus: 255, Slot: 31, Function: 7},
		{Bus: 7, Slot: 16, Function: 4},
	}
	for _, want := range devices {
		got, err := ParsePCIDevice(want.String())
		if err != nil {
			t.Errorf("ParsePCIDevice(%q) = %v", want.String(), err)
			continue
		}
		if got != want {
			t.Errorf("round trip of %v gave %v", want, got)
		}
	}
}

// TestPassthroughValidatesBeforeState checks a bad PCI address is
// reported as such rather than as ErrClosed, which means validation runs
// before the state check.
func TestPassthroughValidatesBeforeState(t *testing.T) {
	vm := closedVM("vm01")
	bad := PCIDevice{Slot: 99}

	ops := map[string]func() error{
		"AssignPassthrough":   func() error { return vm.AssignPassthrough(bad) },
		"UnassignPassthrough": func() error { return vm.UnassignPassthrough(bad) },
		"ResetPassthrough":    func() error { return vm.ResetPassthrough(bad) },
		"MapPassthroughMMIO": func() error {
			return vm.MapPassthroughMMIO(bad, 0, 4096, 0)
		},
		"UnmapPassthroughMMIO": func() error {
			return vm.UnmapPassthroughMMIO(bad, 0, 4096)
		},
		"SetupPassthroughMSI": func() error {
			return vm.SetupPassthroughMSI(bad, MSIConfig{})
		},
		"SetupPassthroughMSIX": func() error {
			return vm.SetupPassthroughMSIX(bad, MSIXConfig{})
		},
		"DisablePassthroughMSIX": func() error {
			return vm.DisablePassthroughMSIX(bad)
		},
	}

	for name, op := range ops {
		if err := op(); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s with a bad address = %v, want ErrInvalidArgument",
				name, err)
		}
	}
}

// TestResetPassthroughIsUnsupported pins the honest answer: no supported
// libvmmapi has an entry point for it.
func TestResetPassthroughIsUnsupported(t *testing.T) {
	vm := closedVM("vm01")

	err := vm.ResetPassthrough(PCIDevice{Bus: 3})
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("ResetPassthrough = %v, want ErrUnsupported", err)
	}
	if Supports(FeaturePPTReset) {
		t.Error("Supports(FeaturePPTReset) = true but the call reports unsupported")
	}
}

// TestPassthroughRejectsZeroSize keeps a zero-length MMIO mapping from
// reaching the kernel.
func TestPassthroughRejectsZeroSize(t *testing.T) {
	vm := closedVM("vm01")
	dev := PCIDevice{Bus: 3}

	if err := vm.MapPassthroughMMIO(dev, 0, 0, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("MapPassthroughMMIO with size 0 = %v, want ErrInvalidArgument", err)
	}
	if err := vm.UnmapPassthroughMMIO(dev, 0, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("UnmapPassthroughMMIO with size 0 = %v, want ErrInvalidArgument", err)
	}

	// A valid request gets past validation and fails on state.
	if err := vm.MapPassthroughMMIO(dev, 0, 4096, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("MapPassthroughMMIO = %v, want ErrClosed", err)
	}
}
