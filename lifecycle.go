//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// SuspendHow is the reason a VM is being suspended, which is what the
// guest's own view of the event is derived from.
type SuspendHow int

const (
	// SuspendNone is the absence of a suspend request.
	SuspendNone SuspendHow = vmmapi.SuspendNone

	// SuspendReset suspends the VM for a reset.
	SuspendReset SuspendHow = vmmapi.SuspendReset

	// SuspendPowerOff suspends the VM for a power off.
	SuspendPowerOff SuspendHow = vmmapi.SuspendPowerOff

	// SuspendHalt suspends the VM because the guest halted.
	SuspendHalt SuspendHow = vmmapi.SuspendHalt

	// SuspendTripleFault suspends the VM because the guest triple
	// faulted.
	SuspendTripleFault SuspendHow = vmmapi.SuspendTripleFault
)

// suspendNames maps the reasons this release knows about to their names.
var suspendNames = map[SuspendHow]string{
	SuspendNone:        "none",
	SuspendReset:       "reset",
	SuspendPowerOff:    "poweroff",
	SuspendHalt:        "halt",
	SuspendTripleFault: "triplefault",
}

// String renders the reason, falling back to its number if this release
// of FreeBSD has a reason this package does not name.
func (h SuspendHow) String() string {
	if name, ok := suspendNames[h]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", int(h))
}

// valid reports whether the reason is one the kernel accepts.
func (h SuspendHow) valid() bool {
	return h >= SuspendNone && int(h) < vmmapi.SuspendLast
}

// Suspend suspends the VM.
//
// The vCPUs stop at their next exit and report the reason, which lets
// the controlling program decide what to do about it.
func (vm *VM) Suspend(how SuspendHow) error {
	const op = "vm_suspend"

	if !how.valid() {
		return fmt.Errorf("vmm: suspend reason %d is out of range: %w",
			int(how), ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.Suspend(int(how)))
}

// Reinitialize resets the VM to the state it had just after creation.
//
// Guest memory is kept; vCPU state, interrupt controllers and device
// state are reset. This is how a guest-initiated reboot is serviced
// without destroying and recreating the VM.
func (vm *VM) Reinitialize() error {
	const op = "vm_reinit"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.Reinit())
}

// SuspendAllCPUs suspends every vCPU in the VM.
func (vm *VM) SuspendAllCPUs() error {
	const op = "vm_suspend_all_cpus"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.SuspendAllCPUs())
}

// ResumeAllCPUs resumes every suspended vCPU in the VM.
func (vm *VM) ResumeAllCPUs() error {
	const op = "vm_resume_all_cpus"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.ResumeAllCPUs())
}
