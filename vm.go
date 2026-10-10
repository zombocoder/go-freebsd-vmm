//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// OpenFlags selects behaviour for OpenWithFlags.
type OpenFlags uint32

const (
	// OpenCreate creates the VM if it does not exist yet. An existing
	// VM is opened as it stands, the way O_CREAT behaves without
	// O_EXCL.
	OpenCreate OpenFlags = 1 << iota

	// OpenReinit resets the VM to its freshly created state after
	// opening it.
	OpenReinit

	// OpenDestroyOnClose makes Close destroy the VM instead of merely
	// releasing the context.
	OpenDestroyOnClose
)

// allOpenFlags is every flag OpenWithFlags understands.
const allOpenFlags = OpenCreate | OpenReinit | OpenDestroyOnClose

// VM is an open handle on a FreeBSD virtual machine, owning one
// libvmmapi context.
//
// # Lifecycle
//
// A VM moves from absent to created to opened, and leaves either by
// Close, which keeps the VM in the kernel, or by Destroy, which removes
// it. Both release the context; whichever runs first wins, and the
// other then does nothing. Every other method returns an error wrapping
// ErrClosed once that has happened.
//
// Cleanup is explicit. The library installs no finalizers, so a handle
// that is never closed leaks a file descriptor for the lifetime of the
// process:
//
//	vm, err := vmm.Open("vm01")
//	if err != nil {
//		return err
//	}
//	defer vm.Close()
//
// # Ownership
//
// A VM owns the vCPU handles opened from it. Closing or destroying the
// VM closes them too, because libvmmapi frees the VM context that each
// vCPU holds a pointer to, and using one afterwards would be a
// use-after-free rather than an error.
//
// # Concurrency
//
// Methods are safe to call concurrently. Reads of VM state and all vCPU
// operations take a shared lock, so work on independent vCPUs does not
// serialise; opening and closing handles take it exclusively.
type VM struct {
	name  string
	flags OpenFlags

	// mu guards ctx and cpus together. Holding it for reading is
	// enough for any operation that merely needs the VM to still be
	// open, which is the common case.
	mu   sync.RWMutex
	ctx  *vmmapi.Context
	cpus map[int]*VCPU
}

// Create creates a VM in the kernel without opening it, loading the
// vmm(4) module first if it is not already loaded.
//
// Creating a VM requires privilege; an unprivileged caller gets EPERM.
func Create(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	return newError("vm_create", name, NoVCPU, vmmapi.Create(name))
}

// Open opens an existing VM.
func Open(name string) (*VM, error) {
	return OpenWithFlags(name, 0)
}

// CreateAndOpen creates the VM if needed and opens it.
func CreateAndOpen(name string) (*VM, error) {
	return OpenWithFlags(name, OpenCreate)
}

// OpenWithFlags opens a VM, applying flags.
//
// The flags are implemented here rather than passed to libvmmapi.
// FreeBSD 14 has no vm_openf(), so OpenCreate, OpenReinit and
// OpenDestroyOnClose are built from vm_create(3), vm_open(3),
// vm_reinit(3) and vm_destroy(3). The observable behaviour matches, with
// one caveat worth knowing: the steps are separate calls, so
// OpenCreate|OpenReinit is not atomic against another process acting on
// the same VM in between.
func OpenWithFlags(name string, flags OpenFlags) (*VM, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if flags&^allOpenFlags != 0 {
		return nil, fmt.Errorf("vmm: unknown open flags %#x: %w",
			uint32(flags&^allOpenFlags), ErrInvalidArgument)
	}

	if flags&OpenCreate != 0 {
		// An already existing VM is not an error here, matching
		// O_CREAT without O_EXCL.
		if errno := vmmapi.Create(name); errno != 0 && !isExist(errno) {
			return nil, newError("vm_create", name, NoVCPU, errno)
		}
	}

	ctx, errno := vmmapi.Open(name)
	if errno != 0 {
		return nil, newError("vm_open", name, NoVCPU, errno)
	}

	vm := &VM{
		name:  name,
		flags: flags,
		ctx:   ctx,
		cpus:  make(map[int]*VCPU),
	}

	if flags&OpenReinit != 0 {
		if errno := ctx.Reinit(); errno != 0 {
			// Leave nothing behind on a failed open.
			_ = vm.Close()
			return nil, newError("vm_reinit", name, NoVCPU, errno)
		}
	}

	return vm, nil
}

// Name returns the VM name. It stays valid after the VM is closed.
func (vm *VM) Name() string {
	if vm == nil {
		return ""
	}
	return vm.name
}

// Close releases the VM context and every vCPU handle opened from it.
//
// The VM itself stays in the kernel unless the VM was opened with
// OpenDestroyOnClose, in which case Close destroys it.
//
// Close is idempotent, and safe to defer alongside an explicit Destroy:
// whichever happens first releases the context and the other becomes a
// no-op. That matters because libvmmapi frees the context in both calls,
// so a second release would be a double free.
func (vm *VM) Close() error {
	if vm == nil {
		return nil
	}
	if vm.flags&OpenDestroyOnClose != 0 {
		return vm.Destroy()
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.ctx == nil {
		return nil
	}
	vm.releaseCPUsLocked()
	vm.ctx.Close()
	vm.ctx = nil

	return nil
}

// Destroy destroys the VM and releases the context and every vCPU handle
// opened from it. Destroy is idempotent.
//
// A kernel-side failure is not reported: vm_destroy(3) returns void on
// the supported libvmmapi versions, so there is nothing to propagate.
// Destroy still returns an error for the one failure it can see, which is
// being called on a handle that was never open.
func (vm *VM) Destroy() error {
	if vm == nil {
		return nil
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.ctx == nil {
		return nil
	}
	vm.releaseCPUsLocked()
	vm.ctx.Destroy()
	vm.ctx = nil

	return nil
}

// releaseCPUsLocked closes every vCPU handle still registered.
//
// This runs before the context goes away: struct vcpu points at the
// struct vmctx that vm_close(3) and vm_destroy(3) free, so the handles
// have to be released first.
//
// vm.mu must be held for writing.
func (vm *VM) releaseCPUsLocked() {
	for id, cpu := range vm.cpus {
		cpu.releaseLocked()
		delete(vm.cpus, id)
	}
}

// context returns the live context under a read lock, or an error
// wrapping ErrClosed.
//
// Callers must call the returned release function when done, and must
// not retain the context past it.
func (vm *VM) context(op string) (*vmmapi.Context, func(), error) {
	if vm == nil {
		return nil, nil, fmt.Errorf("vmm: %s on nil VM: %w", op, ErrClosed)
	}

	vm.mu.RLock()
	if vm.ctx == nil {
		vm.mu.RUnlock()
		return nil, nil, vm.closedError(op)
	}
	return vm.ctx, vm.mu.RUnlock, nil
}

// closedError builds the error for an operation on a closed VM.
func (vm *VM) closedError(op string) error {
	return fmt.Errorf("vmm: %s %q: %w", op, vm.name, ErrClosed)
}

// err wraps an errno from an operation that was not tied to a vCPU.
func (vm *VM) err(op string, errno syscall.Errno) error {
	return newError(op, vm.name, NoVCPU, errno)
}

// isExist reports whether an errno means the VM was already there.
func isExist(errno syscall.Errno) bool {
	return errno == syscall.EEXIST
}
