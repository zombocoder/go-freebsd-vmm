//go:build freebsd && amd64

// Package harness supports the integration tests, which need a FreeBSD
// host with vmm(4) and enough privilege to create virtual machines.
//
// Two things matter here. Tests skip with a reason rather than fail when
// the host cannot host a VM, so that the suite is still useful on a
// developer machine or in a container. And every VM a test creates is
// destroyed afterwards, including when the test fails, so that a CI
// worker does not accumulate objects under /dev/vmm.
package harness

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

// NamePrefix marks the VMs these tests create, so that leftovers can be
// recognised and swept up.
const NamePrefix = "govmm-test-"

// vmmDir is where vmm(4) publishes its devices.
const vmmDir = "/dev/vmm"

// Name returns a VM name unique to this process and test.
//
// The pid keeps concurrent test binaries apart and the random suffix
// keeps repeated runs apart, so a leftover from a killed run never
// collides with a live test.
func Name(t *testing.T) string {
	t.Helper()

	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		t.Fatalf("harness: reading random bytes: %v", err)
	}
	return fmt.Sprintf("%s%d-%08x", NamePrefix, os.Getpid(),
		binary.BigEndian.Uint32(buf[:]))
}

// RequireVM creates a VM, opens it, and arranges for it to be destroyed
// when the test ends.
//
// The test is skipped, with the reason, if the host cannot create one.
func RequireVM(t *testing.T) *vmm.VM {
	t.Helper()

	name := Name(t)

	vm, err := vmm.CreateAndOpen(name)
	if err != nil {
		if reason, ok := skipReason(err); ok {
			t.Skipf("harness: %s", reason)
		}
		t.Fatalf("harness: creating VM %q: %v", name, err)
	}

	t.Cleanup(func() {
		if err := vm.Destroy(); err != nil {
			t.Errorf("harness: destroying VM %q: %v", name, err)
		}
	})

	return vm
}

// MaxCPUs returns how many vCPUs a VM has.
//
// This is not something a test can choose. The kernel fixes it when the
// VM is created, from the hw.vmm.maxcpu tunable, which defaults to the
// number of host CPUs; vm_set_topology(9) ignores the maxcpus it is
// passed. A test that needs a particular number has to ask the VM.
func MaxCPUs(t *testing.T, vm *vmm.VM) uint16 {
	t.Helper()

	topology, err := vm.Topology()
	if err != nil {
		t.Fatalf("harness: reading topology of %q: %v", vm.Name(), err)
	}
	return topology.MaxCPUs
}

// RequireVMWithCPUs creates a VM presenting the given number of vCPUs
// and the given amount of memory, which is what most tests need before
// they can touch a vCPU.
//
// The test is skipped if the host cannot give the VM that many vCPUs.
// Memory is skipped when size is 0.
func RequireVMWithCPUs(t *testing.T, cpus uint16, size uint64) *vmm.VM {
	t.Helper()

	vm := RequireVM(t)

	if maxcpus := MaxCPUs(t, vm); cpus > maxcpus {
		t.Skipf("harness: this test needs %d vcpus but the host allows %d (hw.vmm.maxcpu)",
			cpus, maxcpus)
	}

	topology := vmm.Topology{
		Sockets: 1,
		Cores:   cpus,
		Threads: 1,
	}
	if err := vm.SetTopology(topology); err != nil {
		t.Fatalf("harness: setting topology %v: %v", topology, err)
	}
	if size > 0 {
		if err := vm.SetupMemory(size, vmm.MemoryMapAll); err != nil {
			t.Fatalf("harness: setting up %d bytes of memory: %v", size, err)
		}
	}

	return vm
}

// RequireVCPU opens a vCPU and arranges for it to be closed when the
// test ends.
func RequireVCPU(t *testing.T, vm *vmm.VM, id int) *vmm.VCPU {
	t.Helper()

	cpu, err := vm.VCPU(id)
	if err != nil {
		t.Fatalf("harness: opening vcpu %d of %q: %v", id, vm.Name(), err)
	}
	t.Cleanup(func() {
		if err := cpu.Close(); err != nil {
			t.Errorf("harness: closing vcpu %d: %v", id, err)
		}
	})

	return cpu
}

// skipReason turns an environment problem into something worth printing,
// and reports whether the error was one.
//
// Anything else is a real failure and must not be hidden behind a skip.
func skipReason(err error) (string, bool) {
	switch {
	case errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EACCES):
		return fmt.Sprintf("creating a VM needs privilege and this process runs as uid %d",
			os.Geteuid()), true

	case errors.Is(err, syscall.ENOENT):
		return "vmm(4) is not available: the module could not be loaded", true

	case errors.Is(err, syscall.ENXIO), errors.Is(err, syscall.ENODEV):
		return "the host has no usable hardware virtualization", true

	case errors.Is(err, syscall.EOPNOTSUPP), errors.Is(err, syscall.ENOTSUP):
		return "vmm(4) reports the operation is unsupported on this host", true
	}

	return "", false
}

// Sweep destroys every VM left behind by these tests.
//
// It runs from TestMain so that a run interrupted part way through does
// not leave objects under /dev/vmm for the next one to trip over.
func Sweep() {
	entries, err := os.ReadDir(vmmDir)
	if err != nil {
		// No /dev/vmm means vmm(4) has never had a VM, so there is
		// nothing to clean up.
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, NamePrefix) {
			continue
		}
		vm, err := vmm.Open(name)
		if err != nil {
			continue
		}
		if err := vm.Destroy(); err != nil {
			fmt.Fprintf(os.Stderr, "harness: sweeping %s: %v\n",
				filepath.Join(vmmDir, name), err)
		}
	}
}

// Run is the body of TestMain for every integration test package. It
// sweeps leftovers before and after the run.
func Run(m *testing.M) int {
	Sweep()
	code := m.Run()
	Sweep()
	return code
}
