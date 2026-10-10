//go:build freebsd && amd64

// Package vcpu_test exercises vCPU handles, registers, capabilities and
// statistics against a real kernel.
package vcpu_test

import (
	"errors"
	"os"
	"testing"

	vmm "github.com/zombocoder/go-freebsd-vmm"
	"github.com/zombocoder/go-freebsd-vmm/tests/harness"
)

// vmMemory is enough guest memory for the kernel to accept vCPU
// operations without being large enough to slow the tests down.
const vmMemory = 64 << 20

func TestMain(m *testing.M) {
	os.Exit(harness.Run(m))
}

func TestOpenAndCloseVCPU(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 2, vmMemory)

	cpu, err := vm.VCPU(1)
	if err != nil {
		t.Fatalf("VCPU(1) = %v", err)
	}
	if got := cpu.ID(); got != 1 {
		t.Errorf("ID() = %d, want 1", got)
	}

	if err := cpu.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if err := cpu.Close(); err != nil {
		t.Errorf("Close() twice = %v, want nil", err)
	}
	if err := cpu.Reset(); !errors.Is(err, vmm.ErrClosed) {
		t.Errorf("Reset() after Close = %v, want ErrClosed", err)
	}
}

// TestVCPUIDIsRangeChecked is the guard for vm_vcpu_open(3) performing
// no ioctl: without a check here an out-of-range id would look like a
// success and fail later with a bare EINVAL.
func TestVCPUIDIsRangeChecked(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)

	// The bound is the VM's vCPU count, which the kernel fixes at
	// creation; a topology asking for fewer does not lower it.
	cpus := int(harness.MaxCPUs(t, vm))

	for _, id := range []int{cpus, cpus + 1, 1024} {
		_, err := vm.VCPU(id)
		if !errors.Is(err, vmm.ErrInvalidArgument) {
			t.Errorf("VCPU(%d) on a %d-vcpu VM = %v, want ErrInvalidArgument",
				id, cpus, err)
		}
	}

	// Every id inside the bound works.
	for id := 0; id < cpus; id++ {
		if _, err := vm.VCPU(id); err != nil {
			t.Errorf("VCPU(%d) = %v, want success", id, err)
		}
	}
}

// TestVCPUReturnsTheSameHandle matters for safety rather than
// convenience: two handles on one id would both free the same
// allocation.
func TestVCPUReturnsTheSameHandle(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 2, vmMemory)

	first, err := vm.VCPU(0)
	if err != nil {
		t.Fatalf("VCPU(0) = %v", err)
	}
	second, err := vm.VCPU(0)
	if err != nil {
		t.Fatalf("VCPU(0) again = %v", err)
	}

	if first != second {
		t.Error("VCPU(0) returned two different handles for one vcpu")
	}
}

// TestClosingVMInvalidatesVCPUs is the cascade from the ownership rules.
// libvmmapi frees the context that each struct vcpu points into, so a
// handle left live afterwards would be a use-after-free.
func TestClosingVMInvalidatesVCPUs(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 2, vmMemory)

	cpu0, err := vm.VCPU(0)
	if err != nil {
		t.Fatalf("VCPU(0) = %v", err)
	}
	cpu1, err := vm.VCPU(1)
	if err != nil {
		t.Fatalf("VCPU(1) = %v", err)
	}

	if err := vm.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	for _, cpu := range []*vmm.VCPU{cpu0, cpu1} {
		if err := cpu.Reset(); !errors.Is(err, vmm.ErrClosed) {
			t.Errorf("vcpu %d Reset() after the VM closed = %v, want ErrClosed",
				cpu.ID(), err)
		}
		if _, err := cpu.Register(vmm.RegRIP); !errors.Is(err, vmm.ErrClosed) {
			t.Errorf("vcpu %d Register() after the VM closed = %v, want ErrClosed",
				cpu.ID(), err)
		}
		// Closing an already invalidated handle must not double free.
		if err := cpu.Close(); err != nil {
			t.Errorf("vcpu %d Close() after the VM closed = %v, want nil",
				cpu.ID(), err)
		}
	}
}

func TestActivateSuspendResume(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	if err := cpu.Activate(); err != nil {
		t.Fatalf("Activate() = %v", err)
	}

	active, err := vm.ActiveCPUs()
	if err != nil {
		t.Fatalf("ActiveCPUs() = %v", err)
	}
	if !active.Contains(0) {
		t.Errorf("ActiveCPUs() = %v, want it to contain vcpu 0", active)
	}

	if err := cpu.Suspend(); err != nil {
		t.Errorf("Suspend() = %v", err)
	}
	if err := cpu.Resume(); err != nil {
		t.Errorf("Resume() = %v", err)
	}
}

func TestReset(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	if err := cpu.Reset(); err != nil {
		t.Fatalf("Reset() = %v", err)
	}

	// A reset vCPU starts at the architectural power-on vector.
	rip, err := cpu.Register(vmm.RegRIP)
	if err != nil {
		t.Fatalf("Register(RegRIP) = %v", err)
	}
	t.Logf("RIP after reset: %#x", rip)
}

func TestRegisterRoundTrip(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	tests := []struct {
		reg  vmm.Register
		want uint64
	}{
		{vmm.RegRAX, 0xdeadbeef},
		{vmm.RegRBX, 0},
		{vmm.RegRCX, 0xffffffffffffffff},
		{vmm.RegRDX, 1},
		{vmm.RegRSI, 0x1000},
		{vmm.RegRDI, 0x5555555555555555},
	}

	for _, tc := range tests {
		t.Run(tc.reg.String(), func(t *testing.T) {
			if err := cpu.SetRegister(tc.reg, tc.want); err != nil {
				t.Fatalf("SetRegister(%v, %#x) = %v", tc.reg, tc.want, err)
			}
			got, err := cpu.Register(tc.reg)
			if err != nil {
				t.Fatalf("Register(%v) = %v", tc.reg, err)
			}
			if got != tc.want {
				t.Errorf("Register(%v) = %#x, want %#x", tc.reg, got, tc.want)
			}
		})
	}
}

// TestBulkRegistersMatchSingleReads checks the bulk path against the
// single-register path, which is the thing that could silently go wrong
// about it: the values have to come back paired with the right register.
func TestBulkRegistersMatchSingleReads(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	want := []vmm.RegisterValue{
		{Register: vmm.RegRAX, Value: 0x1111111111111111},
		{Register: vmm.RegRBX, Value: 0x2222222222222222},
		{Register: vmm.RegRCX, Value: 0x3333333333333333},
		{Register: vmm.RegRDX, Value: 0x4444444444444444},
		{Register: vmm.RegRSI, Value: 0x5555555555555555},
		{Register: vmm.RegRDI, Value: 0x6666666666666666},
		{Register: vmm.RegRBP, Value: 0x7777777777777777},
		{Register: vmm.RegR8, Value: 0x8888888888888888},
	}

	if err := cpu.SetRegisters(want); err != nil {
		t.Fatalf("SetRegisters = %v", err)
	}

	regs := make([]vmm.Register, len(want))
	for i, rv := range want {
		regs[i] = rv.Register
	}

	got, err := cpu.Registers(regs)
	if err != nil {
		t.Fatalf("Registers = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Registers returned %d values, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Registers[%d] = %v, want %v", i, got[i], want[i])
		}
		// And the same value through the single-register path.
		single, err := cpu.Register(want[i].Register)
		if err != nil {
			t.Fatalf("Register(%v) = %v", want[i].Register, err)
		}
		if single != want[i].Value {
			t.Errorf("Register(%v) = %#x, bulk said %#x",
				want[i].Register, single, want[i].Value)
		}
	}
}

func TestSegmentDescriptorRoundTrip(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	want := vmm.SegmentDescriptor{
		Base:   0x1000,
		Limit:  0xffff,
		Access: 0x93,
	}

	if err := cpu.SetSegmentDescriptor(vmm.RegDS, want); err != nil {
		t.Fatalf("SetSegmentDescriptor(RegDS, %v) = %v", want, err)
	}

	got, err := cpu.SegmentDescriptor(vmm.RegDS)
	if err != nil {
		t.Fatalf("SegmentDescriptor(RegDS) = %v", err)
	}
	if got.Base != want.Base || got.Limit != want.Limit {
		t.Errorf("SegmentDescriptor(RegDS) = %v, want base and limit from %v",
			got, want)
	}
}

func TestCapabilityRoundTrip(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	// HLT exit is implemented by both VT-x and AMD-V, so it is the
	// safe one to toggle.
	before, err := cpu.Capability(vmm.CapHaltExit)
	if err != nil {
		t.Fatalf("Capability(CapHaltExit) = %v", err)
	}

	if err := cpu.SetCapability(vmm.CapHaltExit, 1); err != nil {
		t.Fatalf("SetCapability(CapHaltExit, 1) = %v", err)
	}
	after, err := cpu.Capability(vmm.CapHaltExit)
	if err != nil {
		t.Fatalf("Capability(CapHaltExit) after set = %v", err)
	}
	if after != 1 {
		t.Errorf("Capability(CapHaltExit) = %d after setting it to 1", after)
	}

	if err := cpu.SetCapability(vmm.CapHaltExit, before); err != nil {
		t.Errorf("restoring CapHaltExit to %d = %v", before, err)
	}
}

// TestCapabilityNamesComeFromLibvmmapi checks the name mapping is the
// system's rather than a table in this package.
func TestCapabilityNamesComeFromLibvmmapi(t *testing.T) {
	name := vmm.CapHaltExit.String()
	if name == "" {
		t.Fatal("CapHaltExit.String() is empty")
	}
	t.Logf("CapHaltExit is named %q", name)

	got, err := vmm.CapabilityByName(name)
	if err != nil {
		t.Fatalf("CapabilityByName(%q) = %v", name, err)
	}
	if got != vmm.CapHaltExit {
		t.Errorf("CapabilityByName(%q) = %v, want CapHaltExit", name, got)
	}

	if _, err := vmm.CapabilityByName("no-such-capability"); !errors.Is(err, vmm.ErrInvalidArgument) {
		t.Errorf("CapabilityByName on an unknown name = %v, want ErrInvalidArgument", err)
	}
}

func TestStats(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	stats, err := cpu.Stats()
	if err != nil {
		t.Fatalf("Stats() = %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("Stats() returned nothing")
	}

	for i, s := range stats {
		if s.Index != i {
			t.Errorf("stats[%d].Index = %d, want %d", i, s.Index, i)
		}
		if s.Description == "" {
			t.Errorf("stats[%d] has no description", i)
		}
	}
	t.Logf("%d counters, first is %q", len(stats), stats[0].Description)
}

func TestStatsSnapshotHasKernelTime(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	snapshot, err := cpu.StatsSnapshot()
	if err != nil {
		t.Fatalf("StatsSnapshot() = %v", err)
	}
	if snapshot.Time.IsZero() {
		t.Error("StatsSnapshot().Time is zero, want the kernel's sample time")
	}
	if len(snapshot.Stats) == 0 {
		t.Error("StatsSnapshot().Stats is empty")
	}
	t.Logf("sampled at %v", snapshot.Time)
}

// TestConcurrentStatsAreSerialised drives the path that reads
// libvmmapi's static storage from several goroutines at once. Run under
// -race, this is what would catch the descriptions being read from a
// buffer another goroutine has already overwritten.
func TestConcurrentStatsAreSerialised(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpu := harness.RequireVCPU(t, vm, 0)

	reference, err := cpu.Stats()
	if err != nil {
		t.Fatalf("Stats() = %v", err)
	}

	const goroutines = 16
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			stats, err := cpu.Stats()
			if err != nil {
				errs <- err
				return
			}
			if len(stats) != len(reference) {
				errs <- errors.New("stats count changed between calls")
				return
			}
			for j := range stats {
				if stats[j].Description != reference[j].Description {
					errs <- errors.New("stat description does not match the reference")
					return
				}
			}
			errs <- nil
		}()
	}

	for i := 0; i < goroutines; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent Stats(): %v", err)
		}
	}
}

// TestConcurrentVCPUsDoNotSerialise checks the locking lets work on
// independent vCPUs proceed together, which is the point of the shared
// lock on VM state.
func TestConcurrentVCPUsDoNotSerialise(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, vmMemory)
	cpus := int(harness.MaxCPUs(t, vm))

	handles := make([]*vmm.VCPU, cpus)
	for i := range handles {
		handles[i] = harness.RequireVCPU(t, vm, i)
	}

	errs := make(chan error, cpus)
	for i, cpu := range handles {
		go func(id int, cpu *vmm.VCPU) {
			want := uint64(0x1000 + id)
			if err := cpu.SetRegister(vmm.RegRAX, want); err != nil {
				errs <- err
				return
			}
			got, err := cpu.Register(vmm.RegRAX)
			if err != nil {
				errs <- err
				return
			}
			if got != want {
				errs <- errors.New("register value crossed between vcpus")
				return
			}
			errs <- nil
		}(i, cpu)
	}

	for i := 0; i < cpus; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent vcpu work: %v", err)
		}
	}
}
