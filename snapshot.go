//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// SnapshotStructure names a piece of kernel state a snapshot request can
// save or restore.
//
// These are the structures vmm(4) owns. A guest's full state is more
// than this: the device models outside the kernel have their own, and
// nothing here collects it.
type SnapshotStructure int

const (
	// SnapshotVM is the virtual machine's own state.
	SnapshotVM SnapshotStructure = vmmapi.SnapshotVM

	// SnapshotVMCX is the per-vCPU processor state, the VMCS on
	// Intel or the VMCB on AMD.
	SnapshotVMCX SnapshotStructure = vmmapi.SnapshotVMCX

	// SnapshotLAPIC is the local APIC state of the vCPUs.
	SnapshotLAPIC SnapshotStructure = vmmapi.SnapshotLAPIC

	// SnapshotIOAPIC is the I/O APIC state.
	SnapshotIOAPIC SnapshotStructure = vmmapi.SnapshotIOAPIC

	// SnapshotHPET is the high precision event timer state.
	SnapshotHPET SnapshotStructure = vmmapi.SnapshotHPET

	// SnapshotATPIC is the legacy 8259 interrupt controller state.
	SnapshotATPIC SnapshotStructure = vmmapi.SnapshotATPIC

	// SnapshotATPIT is the legacy 8254 timer state.
	SnapshotATPIT SnapshotStructure = vmmapi.SnapshotATPIT

	// SnapshotPMTimer is the ACPI power management timer state.
	SnapshotPMTimer SnapshotStructure = vmmapi.SnapshotPMTimer

	// SnapshotRTC is the real time clock state.
	SnapshotRTC SnapshotStructure = vmmapi.SnapshotRTC

	// SnapshotMemory is the guest memory map.
	SnapshotMemory SnapshotStructure = vmmapi.SnapshotMem
)

// snapshotNames maps the structures this package names to a label.
var snapshotNames = map[SnapshotStructure]string{
	SnapshotVM:      "vm",
	SnapshotVMCX:    "vmcx",
	SnapshotLAPIC:   "lapic",
	SnapshotIOAPIC:  "ioapic",
	SnapshotHPET:    "hpet",
	SnapshotATPIC:   "atpic",
	SnapshotATPIT:   "atpit",
	SnapshotPMTimer: "pmtimer",
	SnapshotRTC:     "rtc",
	SnapshotMemory:  "memory",
}

// String renders the structure's name.
func (s SnapshotStructure) String() string {
	if name, ok := snapshotNames[s]; ok {
		return name
	}
	return fmt.Sprintf("snapshot(%d)", int(s))
}

// valid reports whether this package names the structure.
func (s SnapshotStructure) valid() bool {
	_, ok := snapshotNames[s]
	return ok
}

// SnapshotMetadata describes one snapshot request.
type SnapshotMetadata struct {
	// Structure names the kernel state to save or restore.
	Structure SnapshotStructure

	// Device names a userspace device, for requests that carry one.
	// Leave it empty for the kernel structures above.
	Device string

	// Buffer holds the state. For a save it receives it and must be
	// large enough; for a restore it supplies what a previous save
	// produced for the same structure.
	Buffer []byte
}

// SnapshotSave writes one piece of kernel state into meta.Buffer and
// returns how many bytes it used.
//
// # Experimental
//
// This is a low-level primitive and is expected to change. Two
// warnings, both load-bearing:
//
// A snapshot taken with these calls is not a restorable virtual
// machine. It covers what vmm(4) owns and nothing else: the state of
// every device modelled outside the kernel is absent, and so is any
// agreement about file format or ordering. Assembling a snapshot that
// can actually be restored is the job of the layer that owns the device
// models, which is bhyve's, not this library's.
//
// The format is the kernel's internal representation, with no version
// negotiation. State saved on one kernel is not portable to another.
//
// A buffer too small reports ENOSPC; the size needed depends on the
// structure and on the VM's topology, so the practical approach is to
// start generously.
//
// Snapshot support is a kernel build option. A kernel built without it
// has no ioctl behind this call and reports ENOTTY, which is worth
// distinguishing from a real failure:
//
//	if errors.Is(err, syscall.ENOTTY) {
//		// this kernel has no snapshot support
//	}
func (vm *VM) SnapshotSave(meta *SnapshotMetadata) (int, error) {
	return vm.snapshot(meta, vmmapi.SnapshotSave)
}

// SnapshotRestore reads one piece of kernel state back from
// meta.Buffer.
//
// The same warnings as SnapshotSave apply. The buffer must hold exactly
// what a save of the same structure on the same kernel produced.
func (vm *VM) SnapshotRestore(meta *SnapshotMetadata) (int, error) {
	return vm.snapshot(meta, vmmapi.SnapshotRestore)
}

func (vm *VM) snapshot(meta *SnapshotMetadata, op int) (int, error) {
	const name = "vm_snapshot_req"

	if meta == nil {
		return 0, fmt.Errorf("vmm: %s needs metadata: %w", name,
			ErrInvalidArgument)
	}
	if !meta.Structure.valid() {
		return 0, fmt.Errorf("vmm: unknown snapshot structure %d: %w",
			int(meta.Structure), ErrInvalidArgument)
	}
	if len(meta.Buffer) == 0 {
		return 0, fmt.Errorf("vmm: snapshot buffer is empty: %w",
			ErrInvalidArgument)
	}

	ctx, release, err := vm.context(name)
	if err != nil {
		return 0, err
	}
	defer release()

	used, errno := ctx.SnapshotReq(meta.Device, int(meta.Structure), op,
		meta.Buffer)
	if errno != 0 {
		return int(used), vm.err(name, errno)
	}
	return int(used), nil
}

// RestoreTime tells the kernel to re-derive the VM's notion of time
// after a restore, so that the guest's timers resume from the host clock
// rather than from where they were when the snapshot was taken.
func (vm *VM) RestoreTime() error {
	const op = "vm_restore_time"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.RestoreTime())
}

// LimitRights drops the Capsicum rights on the VM's descriptor to the
// set vmm(4) actually needs.
//
// A process that has done this for every VM it holds can then enter
// capability mode and keep working with them. It is one way, and the
// rights cannot be restored.
func (vm *VM) LimitRights() error {
	const op = "vm_limit_rights"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.LimitRights())
}
