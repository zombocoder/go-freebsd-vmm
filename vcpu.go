//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"syscall"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// VCPU is a handle on one virtual CPU of a VM.
//
// A vCPU belongs to the VM it was opened from. Closing or destroying
// that VM closes the vCPU handle too, and every method then returns an
// error wrapping ErrClosed.
//
// Lifetime is explicit, as it is for VM:
//
//	cpu, err := vm.VCPU(0)
//	if err != nil {
//		return err
//	}
//	defer cpu.Close()
//
// A single VCPU must not be driven from several goroutines at once. The
// handle itself is safe to use concurrently with work on other vCPUs of
// the same VM.
type VCPU struct {
	vm  *VM
	id  int
	cpu *vmmapi.VCPU
}

// VCPU opens a handle on virtual CPU id.
//
// Opening the same id twice returns the handle already in use rather
// than a second one, so that closing either does not leave the other
// pointing at freed memory.
//
// The id is checked against the VM's topology first. That check is not
// decoration: vm_vcpu_open(3) issues no ioctl and only allocates, so it
// reports success for any id at all, and an out-of-range one would
// surface much later as an unexplained EINVAL from the first register
// access.
func (vm *VM) VCPU(id int) (*VCPU, error) {
	const op = "vm_vcpu_open"

	if vm == nil {
		return nil, fmt.Errorf("vmm: %s on nil VM: %w", op, ErrClosed)
	}
	if id < 0 {
		return nil, fmt.Errorf("vmm: vcpu id %d is negative: %w",
			id, ErrInvalidArgument)
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.ctx == nil {
		return nil, vm.closedError(op)
	}
	if cpu, ok := vm.cpus[id]; ok {
		return cpu, nil
	}

	_, _, _, maxcpus, errno := vm.ctx.Topology()
	if errno != 0 {
		return nil, newError("vm_get_topology", vm.name, NoVCPU, errno)
	}
	if id >= int(maxcpus) {
		return nil, fmt.Errorf("vmm: vcpu id %d is outside the VM's %d vcpus: %w",
			id, maxcpus, ErrInvalidArgument)
	}

	cpu, errno := vmmapi.VCPUOpen(vm.ctx, id)
	if errno != 0 {
		return nil, newError(op, vm.name, id, errno)
	}

	handle := &VCPU{vm: vm, id: id, cpu: cpu}
	vm.cpus[id] = handle

	return handle, nil
}

// ID returns the virtual CPU number. It stays valid after Close.
func (cpu *VCPU) ID() int {
	if cpu == nil {
		return NoVCPU
	}
	return cpu.id
}

// Close releases the vCPU handle. It is idempotent, and does not affect
// the vCPU's state in the kernel.
func (cpu *VCPU) Close() error {
	if cpu == nil || cpu.vm == nil {
		return nil
	}

	cpu.vm.mu.Lock()
	defer cpu.vm.mu.Unlock()

	if cpu.cpu == nil {
		return nil
	}
	if cpu.vm.cpus[cpu.id] == cpu {
		delete(cpu.vm.cpus, cpu.id)
	}
	cpu.cpu.Close()
	cpu.cpu = nil

	return nil
}

// releaseLocked closes the handle on behalf of the owning VM.
//
// cpu.vm.mu must be held for writing, which is why this cannot simply
// call Close.
func (cpu *VCPU) releaseLocked() {
	if cpu == nil || cpu.cpu == nil {
		return
	}
	cpu.cpu.Close()
	cpu.cpu = nil
}

// handle returns the live vCPU under a read lock on the owning VM.
//
// Callers must call the returned release function when done.
func (cpu *VCPU) handle(op string) (*vmmapi.VCPU, func(), error) {
	if cpu == nil || cpu.vm == nil {
		return nil, nil, fmt.Errorf("vmm: %s on nil vcpu: %w", op, ErrClosed)
	}

	cpu.vm.mu.RLock()
	if cpu.cpu == nil || cpu.vm.ctx == nil {
		cpu.vm.mu.RUnlock()
		return nil, nil, fmt.Errorf("vmm: %s %q vcpu %d: %w",
			op, cpu.vm.name, cpu.id, ErrClosed)
	}
	return cpu.cpu, cpu.vm.mu.RUnlock, nil
}

// err wraps an errno from an operation on this vCPU.
func (cpu *VCPU) err(op string, errno syscall.Errno) error {
	return newError(op, cpu.vm.name, cpu.id, errno)
}

// simple runs a vCPU operation that takes no arguments and returns only
// an errno.
func (cpu *VCPU) simple(op string, fn func(*vmmapi.VCPU) syscall.Errno) error {
	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, fn(h))
}

// Reset resets the vCPU's register state.
func (cpu *VCPU) Reset() error {
	return cpu.simple("vcpu_reset", (*vmmapi.VCPU).Reset)
}

// Activate marks the vCPU active, making it eligible to run.
func (cpu *VCPU) Activate() error {
	return cpu.simple("vm_activate_cpu", (*vmmapi.VCPU).Activate)
}

// Suspend suspends the vCPU.
func (cpu *VCPU) Suspend() error {
	return cpu.simple("vm_suspend_cpu", (*vmmapi.VCPU).Suspend)
}

// Resume resumes a suspended vCPU.
func (cpu *VCPU) Resume() error {
	return cpu.simple("vm_resume_cpu", (*vmmapi.VCPU).Resume)
}

// RestartInstruction arranges for the instruction the vCPU last exited
// on to be executed again when it next enters the guest.
func (cpu *VCPU) RestartInstruction() error {
	return cpu.simple("vm_restart_instruction",
		(*vmmapi.VCPU).RestartInstruction)
}
