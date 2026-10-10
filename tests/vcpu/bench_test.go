//go:build freebsd && amd64

package vcpu_test

import (
	"testing"

	vmm "github.com/zombocoder/go-freebsd-vmm"
	"github.com/zombocoder/go-freebsd-vmm/tests/harness"
)

// benchRegs is the set a VM exit handler typically wants: the general
// purpose registers plus RIP and RFLAGS.
var benchRegs = []vmm.Register{
	vmm.RegRAX, vmm.RegRBX, vmm.RegRCX, vmm.RegRDX,
	vmm.RegRSI, vmm.RegRDI, vmm.RegRBP, vmm.RegRSP,
	vmm.RegR8, vmm.RegR9, vmm.RegR10, vmm.RegR11,
	vmm.RegRIP, vmm.RegRFLAGS,
}

// benchVCPU sets up a VM and vCPU for the benchmarks, skipping if the
// host cannot host one.
func benchVCPU(b *testing.B) *vmm.VCPU {
	b.Helper()

	// The harness helpers take *testing.T, so the setup is spelled
	// out here rather than shared. The name still carries the sweep
	// prefix so an interrupted run gets cleaned up like any other.
	vm, err := vmm.CreateAndOpen(harness.NamePrefix + "bench")
	if err != nil {
		b.Skipf("this host cannot create VMs: %v", err)
	}
	b.Cleanup(func() { vm.Destroy() })

	if err := vm.SetTopology(vmm.Topology{
		Sockets: 1, Cores: 1, Threads: 1, MaxCPUs: 1,
	}); err != nil {
		b.Fatalf("SetTopology: %v", err)
	}
	if err := vm.SetupMemory(vmMemory, vmm.MemoryMapAll); err != nil {
		b.Fatalf("SetupMemory: %v", err)
	}

	cpu, err := vm.VCPU(0)
	if err != nil {
		b.Fatalf("VCPU(0): %v", err)
	}
	b.Cleanup(func() { cpu.Close() })

	return cpu
}

// BenchmarkRegister measures one register read, which is one cgo
// transition and one ioctl.
func BenchmarkRegister(b *testing.B) {
	cpu := benchVCPU(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cpu.Register(vmm.RegRIP); err != nil {
			b.Fatalf("Register: %v", err)
		}
	}
}

// BenchmarkRegistersOneByOne reads the whole set through the single
// register call, so the cost of len(benchRegs) transitions is visible.
func BenchmarkRegistersOneByOne(b *testing.B) {
	cpu := benchVCPU(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, reg := range benchRegs {
			if _, err := cpu.Register(reg); err != nil {
				b.Fatalf("Register: %v", err)
			}
		}
	}
}

// BenchmarkRegistersBulk reads the same set in one transition. The gap
// against BenchmarkRegistersOneByOne is the reason the bulk API exists.
func BenchmarkRegistersBulk(b *testing.B) {
	cpu := benchVCPU(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cpu.Registers(benchRegs); err != nil {
			b.Fatalf("Registers: %v", err)
		}
	}
}

func BenchmarkSetRegister(b *testing.B) {
	cpu := benchVCPU(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cpu.SetRegister(vmm.RegRAX, uint64(i)); err != nil {
			b.Fatalf("SetRegister: %v", err)
		}
	}
}

func BenchmarkSetRegistersBulk(b *testing.B) {
	cpu := benchVCPU(b)

	values := make([]vmm.RegisterValue, len(benchRegs))
	for i, reg := range benchRegs {
		values[i] = vmm.RegisterValue{Register: reg}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cpu.SetRegisters(values); err != nil {
			b.Fatalf("SetRegisters: %v", err)
		}
	}
}

// BenchmarkStats measures the statistics path, which gathers every
// counter and its description in a single transition under a
// process-wide lock.
func BenchmarkStats(b *testing.B) {
	cpu := benchVCPU(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cpu.Stats(); err != nil {
			b.Fatalf("Stats: %v", err)
		}
	}
}
