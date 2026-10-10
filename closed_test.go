//go:build freebsd && amd64

package vmm

import (
	"errors"
	"testing"
)

// The tests here cover the state machine without touching the kernel, by
// building handles that are already closed. That is exactly the shape
// the ownership rules have to survive: libvmmapi frees the VM context in
// both vm_close(3) and vm_destroy(3), so a stale pointer reaching C
// would be a double free or a use-after-free rather than an error.

// closedVM returns a VM handle in the state it is left in after Close.
func closedVM(name string) *VM {
	return &VM{name: name, cpus: make(map[int]*VCPU)}
}

func TestClosedVMOperationsReturnErrClosed(t *testing.T) {
	vm := closedVM("vm01")

	ops := map[string]func() error{
		"Reinitialize":       vm.Reinitialize,
		"SuspendAllCPUs":     vm.SuspendAllCPUs,
		"ResumeAllCPUs":      vm.ResumeAllCPUs,
		"Suspend":            func() error { return vm.Suspend(SuspendReset) },
		"SetTopology":        func() error { return vm.SetTopology(Topology{1, 1, 1, 1}) },
		"SetupMemory":        func() error { return vm.SetupMemory(1<<20, MemoryMapAll) },
		"SetMemoryFlags":     func() error { return vm.SetMemoryFlags(MemoryWired) },
		"SetLowMemoryLimit":  func() error { return vm.SetLowMemoryLimit(1 << 30) },
		"MapMemorySegment":   func() error { return vm.MapMemorySegment(0, 0, 0, 1<<20, ProtRead) },
		"UnmapMemorySegment": func() error { return vm.UnmapMemorySegment(0, 1<<20) },
		"Topology":           func() error { _, err := vm.Topology(); return err },
		"ActiveCPUs":         func() error { _, err := vm.ActiveCPUs(); return err },
		"SuspendedCPUs":      func() error { _, err := vm.SuspendedCPUs(); return err },
		"DebugCPUs":          func() error { _, err := vm.DebugCPUs(); return err },
		"VCPU":               func() error { _, err := vm.VCPU(0); return err },
		"MapGuestMemory":     func() error { _, err := vm.MapGuestMemory(0, 1<<20); return err },
		"MemorySegment":      func() error { _, err := vm.MemorySegment(0); return err },
		"CreateDeviceMemory": func() error { _, err := vm.CreateDeviceMemory(1, "rom", 1<<20); return err },
		"MemoryMappings":     func() error { _, err := vm.MemoryMappings(); return err },
	}

	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			err := op()
			if err == nil {
				t.Fatalf("%s on a closed VM = nil, want an error", name)
			}
			if !errors.Is(err, ErrClosed) {
				t.Errorf("%s on a closed VM = %v, want ErrClosed", name, err)
			}
		})
	}
}

// TestClosedVMQueriesDegradeQuietly covers the accessors that report a
// value rather than an error. They answer zero on a closed VM instead of
// reaching into a freed context.
func TestClosedVMQueriesDegradeQuietly(t *testing.T) {
	vm := closedVM("vm01")

	if got := vm.LowMemorySize(); got != 0 {
		t.Errorf("LowMemorySize() = %d, want 0", got)
	}
	if got := vm.HighMemorySize(); got != 0 {
		t.Errorf("HighMemorySize() = %d, want 0", got)
	}
	if got := vm.HighMemoryBase(); got != 0 {
		t.Errorf("HighMemoryBase() = %v, want 0", got)
	}
	if got := vm.LowMemoryLimit(); got != 0 {
		t.Errorf("LowMemoryLimit() = %d, want 0", got)
	}
	if got := vm.MemoryFlags(); got != 0 {
		t.Errorf("MemoryFlags() = %v, want 0", got)
	}
	if got := vm.Name(); got != "vm01" {
		t.Errorf("Name() = %q, want it to survive Close", got)
	}
}

// TestCloseAndDestroyAreIdempotent is the guard for the double-free
// hazard: libvmmapi frees the context in each of them, so the second
// call has to do nothing at all.
func TestCloseAndDestroyAreIdempotent(t *testing.T) {
	sequences := [][]string{
		{"close"},
		{"close", "close"},
		{"destroy"},
		{"destroy", "destroy"},
		{"close", "destroy"},
		{"destroy", "close"},
		{"close", "destroy", "close", "destroy"},
	}

	for _, seq := range sequences {
		vm := closedVM("vm01")
		for i, step := range seq {
			var err error
			switch step {
			case "close":
				err = vm.Close()
			case "destroy":
				err = vm.Destroy()
			}
			if err != nil {
				t.Errorf("%v step %d (%s) = %v, want nil", seq, i, step, err)
			}
		}
	}
}

func TestNilVMIsSafe(t *testing.T) {
	var vm *VM

	if err := vm.Close(); err != nil {
		t.Errorf("Close() on nil = %v, want nil", err)
	}
	if err := vm.Destroy(); err != nil {
		t.Errorf("Destroy() on nil = %v, want nil", err)
	}
	if got := vm.Name(); got != "" {
		t.Errorf("Name() on nil = %q, want empty", got)
	}
	if _, err := vm.VCPU(0); !errors.Is(err, ErrClosed) {
		t.Errorf("VCPU(0) on nil = %v, want ErrClosed", err)
	}
	if err := vm.Reinitialize(); !errors.Is(err, ErrClosed) {
		t.Errorf("Reinitialize() on nil = %v, want ErrClosed", err)
	}
}

func TestClosedVCPUOperationsReturnErrClosed(t *testing.T) {
	vm := closedVM("vm01")
	cpu := &VCPU{vm: vm, id: 0}

	ops := map[string]func() error{
		"Reset":              cpu.Reset,
		"Activate":           cpu.Activate,
		"Suspend":            cpu.Suspend,
		"Resume":             cpu.Resume,
		"RestartInstruction": cpu.RestartInstruction,
		"Register":           func() error { _, err := cpu.Register(RegRIP); return err },
		"SetRegister":        func() error { return cpu.SetRegister(RegRIP, 0) },
		"Registers":          func() error { _, err := cpu.Registers([]Register{RegRIP}); return err },
		"SetRegisters": func() error {
			return cpu.SetRegisters([]RegisterValue{{RegRIP, 0}})
		},
		"SegmentDescriptor": func() error { _, err := cpu.SegmentDescriptor(RegCS); return err },
		"SetSegmentDescriptor": func() error {
			return cpu.SetSegmentDescriptor(RegCS, SegmentDescriptor{})
		},
		"Capability":    func() error { _, err := cpu.Capability(CapHaltExit); return err },
		"SetCapability": func() error { return cpu.SetCapability(CapHaltExit, 1) },
		"Stats":         func() error { _, err := cpu.Stats(); return err },
		"StatsSnapshot": func() error { _, err := cpu.StatsSnapshot(); return err },
	}

	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			err := op()
			if err == nil {
				t.Fatalf("%s on a closed vcpu = nil, want an error", name)
			}
			if !errors.Is(err, ErrClosed) {
				t.Errorf("%s on a closed vcpu = %v, want ErrClosed", name, err)
			}
		})
	}
}

func TestNilVCPUIsSafe(t *testing.T) {
	var cpu *VCPU

	if err := cpu.Close(); err != nil {
		t.Errorf("Close() on nil = %v, want nil", err)
	}
	if got := cpu.ID(); got != NoVCPU {
		t.Errorf("ID() on nil = %d, want NoVCPU", got)
	}
	if err := cpu.Reset(); !errors.Is(err, ErrClosed) {
		t.Errorf("Reset() on nil = %v, want ErrClosed", err)
	}
	if _, err := cpu.Register(RegRIP); !errors.Is(err, ErrClosed) {
		t.Errorf("Register() on nil = %v, want ErrClosed", err)
	}
}

// TestClosedVCPUIDSurvives documents that the id stays readable after
// Close, so an error message about a dead handle can still name it.
func TestClosedVCPUIDSurvives(t *testing.T) {
	cpu := &VCPU{vm: closedVM("vm01"), id: 3}

	if err := cpu.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if got := cpu.ID(); got != 3 {
		t.Errorf("ID() after Close = %d, want 3", got)
	}
}

func TestInvalidRegionIsSafe(t *testing.T) {
	var r *MemoryRegion

	if r.Valid() {
		t.Error("Valid() on nil = true, want false")
	}
	if got := r.Size(); got != 0 {
		t.Errorf("Size() on nil = %d, want 0", got)
	}
	if got := r.Address(); got != 0 {
		t.Errorf("Address() on nil = %v, want 0", got)
	}
	if got := r.Bytes(); got != nil {
		t.Errorf("Bytes() on nil = %v, want nil", got)
	}

	// A region whose VM has been closed reports itself invalid rather
	// than handing out a slice over memory the VM no longer owns.
	orphan := &MemoryRegion{vm: closedVM("vm01")}
	if orphan.Valid() {
		t.Error("Valid() on a region of a closed VM = true, want false")
	}
	if orphan.Bytes() != nil {
		t.Error("Bytes() on a region of a closed VM returned a slice")
	}
}
