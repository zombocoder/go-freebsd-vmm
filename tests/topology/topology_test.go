//go:build freebsd && amd64

// Package topology_test exercises CPU topology and the CPU state
// queries against a real kernel.
package topology_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	vmm "github.com/zombocoder/go-freebsd-vmm"
	"github.com/zombocoder/go-freebsd-vmm/tests/harness"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Run(m))
}

func TestSetAndGetTopology(t *testing.T) {
	vm := harness.RequireVM(t)

	// The VM's vCPU count is fixed at creation, so a topology that
	// fits has to be derived from it rather than chosen.
	maxcpus := harness.MaxCPUs(t, vm)
	want := vmm.Topology{
		Sockets: 1,
		Cores:   maxcpus,
		Threads: 1,
		MaxCPUs: maxcpus,
	}

	if err := vm.SetTopology(want); err != nil {
		t.Fatalf("SetTopology(%v) = %v", want, err)
	}

	got, err := vm.Topology()
	if err != nil {
		t.Fatalf("Topology() = %v", err)
	}
	if got != want {
		t.Errorf("Topology() = %v, want %v", got, want)
	}
}

// TestTopologyVariants covers the arrangements that fit within the VM's
// vCPU count. Which arrangements those are depends on the host, so they
// are generated rather than listed.
func TestTopologyVariants(t *testing.T) {
	vm := harness.RequireVM(t)
	maxcpus := harness.MaxCPUs(t, vm)

	var tests []vmm.Topology
	for _, threads := range []uint16{1, 2} {
		for _, sockets := range []uint16{1, 2} {
			cores := maxcpus / (threads * sockets)
			if cores == 0 {
				continue
			}
			tests = append(tests, vmm.Topology{
				Sockets: sockets,
				Cores:   cores,
				Threads: threads,
				MaxCPUs: maxcpus,
			})
		}
	}
	if len(tests) == 0 {
		t.Skipf("a VM with %d vcpus admits no arrangement to test", maxcpus)
	}

	for _, want := range tests {
		t.Run(want.String(), func(t *testing.T) {
			vm := harness.RequireVM(t)

			if err := vm.SetTopology(want); err != nil {
				t.Fatalf("SetTopology(%v) = %v", want, err)
			}
			got, err := vm.Topology()
			if err != nil {
				t.Fatalf("Topology() = %v", err)
			}
			if got != want {
				t.Errorf("Topology() = %v, want %v", got, want)
			}
		})
	}
}

// TestSetTopologyIgnoresMaxCPUs pins down the behaviour that is easy to
// mistake for a bug: the vCPU count is fixed when the VM is created and
// SetTopology cannot change it, because vm_set_topology(9) declares its
// maxcpus parameter __unused.
func TestSetTopologyIgnoresMaxCPUs(t *testing.T) {
	vm := harness.RequireVM(t)
	maxcpus := harness.MaxCPUs(t, vm)

	// Ask for one vCPU, and for the count to become one too.
	if err := vm.SetTopology(vmm.Topology{
		Sockets: 1, Cores: 1, Threads: 1, MaxCPUs: 1,
	}); err != nil {
		t.Fatalf("SetTopology = %v", err)
	}

	got, err := vm.Topology()
	if err != nil {
		t.Fatalf("Topology() = %v", err)
	}
	if got.MaxCPUs != maxcpus {
		t.Errorf("MaxCPUs = %d after asking for 1, want it unchanged at %d",
			got.MaxCPUs, maxcpus)
	}
	if got.Cores != 1 {
		t.Errorf("Cores = %d, want the arrangement to have changed to 1",
			got.Cores)
	}

	// Leaving MaxCPUs zero is therefore not an error.
	if err := vm.SetTopology(vmm.Topology{
		Sockets: 1, Cores: 1, Threads: 1,
	}); err != nil {
		t.Errorf("SetTopology with MaxCPUs unset = %v, want nil", err)
	}
}

// TestSetTopologyRejectsTooManyVCPUs checks the bound is reported in Go,
// where it can name the limit, rather than arriving as a bare EINVAL.
func TestSetTopologyRejectsTooManyVCPUs(t *testing.T) {
	vm := harness.RequireVM(t)
	maxcpus := harness.MaxCPUs(t, vm)

	tooMany := vmm.Topology{
		Sockets: 1,
		Cores:   maxcpus + 1,
		Threads: 1,
	}

	err := vm.SetTopology(tooMany)
	if !errors.Is(err, vmm.ErrInvalidArgument) {
		t.Fatalf("SetTopology(%v) on a %d-vcpu VM = %v, want ErrInvalidArgument",
			tooMany, maxcpus, err)
	}
	if got := err.Error(); !strings.Contains(got, "vcpus") {
		t.Errorf("error %q does not mention the vcpu limit", got)
	}
	t.Logf("rejected with: %v", err)
}

// TestDefaultTopology records what a fresh VM reports, which is what
// bounds VM.VCPU before any SetTopology call.
func TestDefaultTopology(t *testing.T) {
	vm := harness.RequireVM(t)

	got, err := vm.Topology()
	if err != nil {
		t.Fatalf("Topology() = %v", err)
	}
	if got.MaxCPUs == 0 {
		t.Error("a fresh VM reports 0 maxcpus, so no vCPU could be opened")
	}
	t.Logf("default topology: %v", got)
}

// TestInvalidTopologyIsRejectedBeforeTheKernel checks the zero checks
// run on the Go side, where they can say which field is wrong.
func TestInvalidTopologyIsRejectedBeforeTheKernel(t *testing.T) {
	vm := harness.RequireVM(t)

	bad := []vmm.Topology{
		{Sockets: 0, Cores: 1, Threads: 1, MaxCPUs: 1},
		{Sockets: 1, Cores: 0, Threads: 1, MaxCPUs: 1},
		{Sockets: 1, Cores: 1, Threads: 0, MaxCPUs: 1},
	}

	for _, topology := range bad {
		err := vm.SetTopology(topology)
		if !errors.Is(err, vmm.ErrInvalidArgument) {
			t.Errorf("SetTopology(%v) = %v, want ErrInvalidArgument",
				topology, err)
		}
	}
}

func TestCPUSetsOnAFreshVM(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 2, 64<<20)

	active, err := vm.ActiveCPUs()
	if err != nil {
		t.Fatalf("ActiveCPUs() = %v", err)
	}
	if active.Len() != 0 {
		t.Errorf("ActiveCPUs() = %v on a VM with no activated vCPU, want empty",
			active)
	}

	suspended, err := vm.SuspendedCPUs()
	if err != nil {
		t.Fatalf("SuspendedCPUs() = %v", err)
	}
	t.Logf("suspended: %v", suspended)

	debug, err := vm.DebugCPUs()
	if err != nil {
		t.Fatalf("DebugCPUs() = %v", err)
	}
	t.Logf("debug: %v", debug)
}

// TestActiveCPUsTracksActivation is the end-to-end check that a cpuset_t
// survives the trip through C as the right bits.
func TestActiveCPUsTracksActivation(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, 64<<20)
	cpus := int(harness.MaxCPUs(t, vm))
	if cpus < 3 {
		t.Skipf("this test activates vcpu 2 and the host allows only %d vcpus", cpus)
	}

	// vCPU 0 is activated by the kernel as the boot processor on some
	// paths, so compare against the set before activating rather than
	// against an assumed empty one.
	before, err := vm.ActiveCPUs()
	if err != nil {
		t.Fatalf("ActiveCPUs() = %v", err)
	}

	cpu := harness.RequireVCPU(t, vm, 2)
	if err := cpu.Activate(); err != nil {
		t.Fatalf("Activate() on vcpu 2 = %v", err)
	}

	after, err := vm.ActiveCPUs()
	if err != nil {
		t.Fatalf("ActiveCPUs() after Activate = %v", err)
	}

	if !after.Contains(2) {
		t.Errorf("ActiveCPUs() = %v, want it to contain vcpu 2", after)
	}
	if after.Len() <= before.Len() {
		t.Errorf("ActiveCPUs() went from %v to %v, want it to grow", before, after)
	}
	for _, id := range after.IDs() {
		if id >= cpus {
			t.Errorf("ActiveCPUs() reports vcpu %d, outside the VM's %d",
				id, cpus)
		}
	}
}
