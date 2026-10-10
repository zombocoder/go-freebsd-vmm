//go:build freebsd && amd64

// Package lifecycle_test exercises the VM lifecycle against a real
// kernel: creating, opening, closing, reopening and destroying.
package lifecycle_test

import (
	"errors"
	"os"
	"syscall"
	"testing"

	vmm "github.com/zombocoder/go-freebsd-vmm"
	"github.com/zombocoder/go-freebsd-vmm/tests/harness"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Run(m))
}

// TestCreateOpenCloseReopenDestroy is the acceptance test for the
// lifecycle: every transition in order, on one VM.
func TestCreateOpenCloseReopenDestroy(t *testing.T) {
	name := harness.Name(t)

	if err := vmm.Create(name); err != nil {
		if skipIfUnavailable(t, err) {
			return
		}
		t.Fatalf("Create(%q) = %v", name, err)
	}

	// Destroy it however the test ends, including on a failure part
	// way through.
	t.Cleanup(func() {
		vm, err := vmm.Open(name)
		if err != nil {
			return
		}
		if err := vm.Destroy(); err != nil {
			t.Errorf("cleanup: destroying %q: %v", name, err)
		}
	})

	vm, err := vmm.Open(name)
	if err != nil {
		t.Fatalf("Open(%q) = %v", name, err)
	}

	if got := vm.Name(); got != name {
		t.Errorf("Name() = %q, want %q", got, name)
	}

	if err := vm.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	// The VM is still in the kernel, so it opens again.
	vm, err = vmm.Open(name)
	if err != nil {
		t.Fatalf("reopening %q after Close: %v", name, err)
	}

	if got := vm.Name(); got != name {
		t.Errorf("Name() after reopen = %q, want %q", got, name)
	}

	if err := vm.Destroy(); err != nil {
		t.Fatalf("Destroy() = %v", err)
	}

	// And now it is gone.
	if _, err := vmm.Open(name); err == nil {
		t.Error("Open after Destroy succeeded, want an error")
	} else if !errors.Is(err, syscall.ENOENT) {
		t.Errorf("Open after Destroy = %v, want ENOENT", err)
	}
}

// TestCloseAfterDestroy is the double-free guard against a live context.
// Both calls free the libvmmapi context, so the second has to do
// nothing.
func TestCloseAfterDestroy(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.Destroy(); err != nil {
		t.Fatalf("Destroy() = %v", err)
	}
	if err := vm.Close(); err != nil {
		t.Errorf("Close() after Destroy = %v, want nil", err)
	}
	if err := vm.Destroy(); err != nil {
		t.Errorf("Destroy() twice = %v, want nil", err)
	}

	// Operations afterwards report the handle is closed.
	if err := vm.Reinitialize(); !errors.Is(err, vmm.ErrClosed) {
		t.Errorf("Reinitialize() after Destroy = %v, want ErrClosed", err)
	}
}

// TestDeferredCloseWithExplicitDestroy is the idiom from the package
// documentation. It has to be safe, since it is what callers will write.
func TestDeferredCloseWithExplicitDestroy(t *testing.T) {
	name := harness.Name(t)

	vm, err := vmm.CreateAndOpen(name)
	if err != nil {
		if skipIfUnavailable(t, err) {
			return
		}
		t.Fatalf("CreateAndOpen(%q) = %v", name, err)
	}

	func() {
		defer func() {
			if err := vm.Close(); err != nil {
				t.Errorf("deferred Close() = %v", err)
			}
		}()

		if err := vm.Destroy(); err != nil {
			t.Errorf("Destroy() = %v", err)
		}
	}()
}

func TestCreateTwiceReportsEEXIST(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vmm.Create(vm.Name()); err == nil {
		t.Error("Create on an existing VM succeeded, want an error")
	} else if !errors.Is(err, syscall.EEXIST) {
		t.Errorf("Create on an existing VM = %v, want EEXIST", err)
	}
}

// TestCreateAndOpenToleratesExisting is the O_CREAT-without-O_EXCL
// behaviour OpenCreate documents.
func TestCreateAndOpenToleratesExisting(t *testing.T) {
	first := harness.RequireVM(t)
	name := first.Name()

	second, err := vmm.CreateAndOpen(name)
	if err != nil {
		t.Fatalf("CreateAndOpen on an existing VM = %v, want success", err)
	}
	defer second.Close()

	if got := second.Name(); got != name {
		t.Errorf("Name() = %q, want %q", got, name)
	}
}

func TestOpenMissingVM(t *testing.T) {
	name := harness.Name(t)

	_, err := vmm.Open(name)
	if err == nil {
		t.Fatalf("Open(%q) on a missing VM succeeded", name)
	}
	if !errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.EACCES) {
		t.Errorf("Open on a missing VM = %v, want ENOENT", err)
	}

	// The error carries the operation and the VM it happened on.
	var e *vmm.Error
	if errors.As(err, &e) {
		if e.Op != "vm_open" {
			t.Errorf("Error.Op = %q, want %q", e.Op, "vm_open")
		}
		if e.VM != name {
			t.Errorf("Error.VM = %q, want %q", e.VM, name)
		}
		if e.VCPU != vmm.NoVCPU {
			t.Errorf("Error.VCPU = %d, want NoVCPU", e.VCPU)
		}
	}
}

func TestOpenDestroyOnClose(t *testing.T) {
	name := harness.Name(t)

	vm, err := vmm.OpenWithFlags(name, vmm.OpenCreate|vmm.OpenDestroyOnClose)
	if err != nil {
		if skipIfUnavailable(t, err) {
			return
		}
		t.Fatalf("OpenWithFlags(%q) = %v", name, err)
	}

	if err := vm.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	// Close destroyed it, so it cannot be reopened.
	if reopened, err := vmm.Open(name); err == nil {
		reopened.Destroy()
		t.Error("the VM survived a Close with OpenDestroyOnClose")
	}
}

func TestOpenReinit(t *testing.T) {
	vm := harness.RequireVM(t)
	name := vm.Name()

	if err := vm.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	reopened, err := vmm.OpenWithFlags(name, vmm.OpenReinit)
	if err != nil {
		t.Fatalf("OpenWithFlags(%q, OpenReinit) = %v", name, err)
	}
	defer reopened.Close()

	if got := reopened.Name(); got != name {
		t.Errorf("Name() = %q, want %q", got, name)
	}
}

func TestReinitialize(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.Reinitialize(); err != nil {
		t.Errorf("Reinitialize() = %v", err)
	}
}

func TestSuspend(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, 64<<20)

	// A VM with no running vCPU still accepts the request; what
	// matters here is that the reason reaches the kernel intact.
	if err := vm.Suspend(vmm.SuspendPowerOff); err != nil {
		t.Errorf("Suspend(SuspendPowerOff) = %v", err)
	}
}

// skipIfUnavailable mirrors the harness skip logic for the tests that
// create VMs by hand rather than through RequireVM.
func skipIfUnavailable(t *testing.T, err error) bool {
	t.Helper()

	for _, errno := range []syscall.Errno{
		syscall.EPERM, syscall.EACCES, syscall.ENOENT,
		syscall.ENXIO, syscall.ENODEV, syscall.EOPNOTSUPP,
	} {
		if errors.Is(err, errno) {
			t.Skipf("this host cannot create VMs: %v", err)
			return true
		}
	}
	return false
}
