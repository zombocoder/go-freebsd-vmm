//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Register identifies a guest register. The architecture-specific
// constants are in register_GOARCH.go, so a register that does not exist
// on the target architecture is a compile error rather than a runtime
// one.
type Register int

// RegisterValue pairs a register with its value, for the bulk calls.
type RegisterValue struct {
	Register Register
	Value    uint64
}

// valid reports whether the register is one this FreeBSD release knows.
func (r Register) valid() bool {
	return r >= 0 && int(r) < vmmapi.RegLast
}

// Register reads one guest register.
func (cpu *VCPU) Register(reg Register) (uint64, error) {
	const op = "vm_get_register"

	if !reg.valid() {
		return 0, fmt.Errorf("vmm: register %d is out of range: %w",
			int(reg), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return 0, err
	}
	defer release()

	val, errno := h.GetRegister(int(reg))
	if errno != 0 {
		return 0, cpu.err(op, errno)
	}
	return val, nil
}

// SetRegister writes one guest register.
func (cpu *VCPU) SetRegister(reg Register, value uint64) error {
	const op = "vm_set_register"

	if !reg.valid() {
		return fmt.Errorf("vmm: register %d is out of range: %w",
			int(reg), ErrInvalidArgument)
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.SetRegister(int(reg), value))
}

// Registers reads several registers at once and returns them paired
// with the register they came from, in the order asked for.
//
// This costs one kernel call and one cgo transition for the whole set,
// where a loop over Register would cost one of each per register.
// Crossing the cgo boundary is expensive enough that the difference is
// worth caring about on any hot path.
func (cpu *VCPU) Registers(regs []Register) ([]RegisterValue, error) {
	const op = "vm_get_register_set"

	if len(regs) == 0 {
		return nil, nil
	}

	nums, err := registerNumbers(regs)
	if err != nil {
		return nil, err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return nil, err
	}
	defer release()

	vals := make([]uint64, len(regs))
	if errno := h.GetRegisterSet(nums, vals); errno != 0 {
		return nil, cpu.err(op, errno)
	}

	out := make([]RegisterValue, len(regs))
	for i, reg := range regs {
		out[i] = RegisterValue{Register: reg, Value: vals[i]}
	}
	return out, nil
}

// SetRegisters writes several registers at once, in one kernel call.
//
// The kernel applies them in the order given, which matters when the
// registers interact: a mode change written before the registers it
// reinterprets will be seen by them.
func (cpu *VCPU) SetRegisters(values []RegisterValue) error {
	const op = "vm_set_register_set"

	if len(values) == 0 {
		return nil
	}

	regs := make([]Register, len(values))
	vals := make([]uint64, len(values))
	for i, rv := range values {
		regs[i] = rv.Register
		vals[i] = rv.Value
	}

	nums, err := registerNumbers(regs)
	if err != nil {
		return err
	}

	h, release, err := cpu.handle(op)
	if err != nil {
		return err
	}
	defer release()

	return cpu.err(op, h.SetRegisterSet(nums, vals))
}

// registerNumbers validates registers and converts them for the bridge.
func registerNumbers(regs []Register) ([]int, error) {
	nums := make([]int, len(regs))
	for i, reg := range regs {
		if !reg.valid() {
			return nil, fmt.Errorf("vmm: register %d at index %d is out of range: %w",
				int(reg), i, ErrInvalidArgument)
		}
		nums[i] = int(reg)
	}
	return nums, nil
}
