//go:build freebsd && amd64

package vmm

import "fmt"

// MSIConfig describes a message signalled interrupt for a passthrough
// device.
//
// Address and Message are the words the device will write to raise the
// interrupt; the local APICs derive the target vCPU and vector from
// them. Vectors is how many consecutive vectors the device is allowed,
// and must be a power of two for MSI.
type MSIConfig struct {
	Address uint64
	Message uint64
	Vectors int
}

// MSIXConfig describes one MSI-X table entry for a passthrough device.
type MSIXConfig struct {
	// Index is the entry's position in the device's MSI-X table.
	Index int

	Address uint64
	Message uint64

	// VectorControl is the entry's control word; bit 0 masks the
	// vector.
	VectorControl uint32
}

// AssignPassthrough hands a host PCI device to the guest.
//
// The device must already be attached to the ppt(4) driver on the host
// rather than to its normal driver, which is a host configuration
// matter and not something this library does. A device still bound to
// another driver reports ENOENT or EBUSY.
//
// Assignment is a privileged operation and takes the device away from
// the host for as long as the guest holds it.
func (vm *VM) AssignPassthrough(dev PCIDevice) error {
	const op = "vm_assign_pptdev"

	if err := dev.validate(); err != nil {
		return err
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.AssignPPTDev(int(dev.Bus), int(dev.Slot),
		int(dev.Function)))
}

// UnassignPassthrough takes a PCI device back from the guest.
func (vm *VM) UnassignPassthrough(dev PCIDevice) error {
	const op = "vm_unassign_pptdev"

	if err := dev.validate(); err != nil {
		return err
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.UnassignPPTDev(int(dev.Bus), int(dev.Slot),
		int(dev.Function)))
}

// ResetPassthrough resets a passthrough device.
//
// Not available on the supported libvmmapi versions, which have no
// entry point for it; this always reports ErrUnsupported.
// Supports(FeaturePPTReset) answers the same question without the call.
func (vm *VM) ResetPassthrough(dev PCIDevice) error {
	if err := dev.validate(); err != nil {
		return err
	}
	return fmt.Errorf("vmm: resetting passthrough device %v: %w",
		dev, ErrUnsupported)
}

// MapPassthroughMMIO maps a region of the device's host physical memory
// into the guest address space.
//
// This is what makes a passed-through device's registers reachable from
// the guest: hostPhysical is where the BAR lives on the host, guest is
// where the guest should see it. Both must be page aligned.
func (vm *VM) MapPassthroughMMIO(dev PCIDevice, guest GuestAddress, size uint64, hostPhysical uint64) error {
	const op = "vm_map_pptdev_mmio"

	if err := dev.validate(); err != nil {
		return err
	}
	if size == 0 {
		return fmt.Errorf("vmm: mmio size is 0: %w", ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.MapPPTDevMMIO(int(dev.Bus), int(dev.Slot),
		int(dev.Function), uint64(guest), size, hostPhysical))
}

// UnmapPassthroughMMIO removes a passthrough MMIO mapping.
func (vm *VM) UnmapPassthroughMMIO(dev PCIDevice, guest GuestAddress, size uint64) error {
	const op = "vm_unmap_pptdev_mmio"

	if err := dev.validate(); err != nil {
		return err
	}
	if size == 0 {
		return fmt.Errorf("vmm: mmio size is 0: %w", ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.UnmapPPTDevMMIO(int(dev.Bus), int(dev.Slot),
		int(dev.Function), uint64(guest), size))
}

// SetupPassthroughMSI configures MSI delivery for a passthrough device.
//
// The host programs the device so that its interrupts arrive as the
// configured message, which the VM's local APICs then deliver to the
// guest.
func (vm *VM) SetupPassthroughMSI(dev PCIDevice, cfg MSIConfig) error {
	const op = "vm_setup_pptdev_msi"

	if err := dev.validate(); err != nil {
		return err
	}
	if cfg.Vectors < 0 {
		return fmt.Errorf("vmm: msi vector count %d is negative: %w",
			cfg.Vectors, ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.SetupPPTDevMSI(int(dev.Bus), int(dev.Slot),
		int(dev.Function), cfg.Address, cfg.Message, cfg.Vectors))
}

// SetupPassthroughMSIX configures one MSI-X table entry for a
// passthrough device.
//
// MSI-X entries are programmed one at a time, each with its own address,
// message and mask bit, which is the difference from MSI.
func (vm *VM) SetupPassthroughMSIX(dev PCIDevice, cfg MSIXConfig) error {
	const op = "vm_setup_pptdev_msix"

	if err := dev.validate(); err != nil {
		return err
	}
	if cfg.Index < 0 {
		return fmt.Errorf("vmm: msi-x index %d is negative: %w",
			cfg.Index, ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.SetupPPTDevMSIX(int(dev.Bus), int(dev.Slot),
		int(dev.Function), cfg.Index, cfg.Address, cfg.Message,
		cfg.VectorControl))
}

// DisablePassthroughMSIX turns MSI-X off for a passthrough device,
// releasing every entry at once.
func (vm *VM) DisablePassthroughMSIX(dev PCIDevice) error {
	const op = "vm_disable_pptdev_msix"

	if err := dev.validate(); err != nil {
		return err
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.DisablePPTDevMSIX(int(dev.Bus), int(dev.Slot),
		int(dev.Function)))
}
