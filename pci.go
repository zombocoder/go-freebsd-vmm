//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"strconv"
	"strings"
)

// Limits of a PCI address, from the bus geometry: 256 buses, 32 slots
// per bus, 8 functions per slot.
const (
	MaxPCIBus      = 255
	MaxPCISlot     = 31
	MaxPCIFunction = 7
)

// PCIDevice is the bus address of a PCI device on the host.
//
// It identifies a device to assign to a guest, not a slot inside the
// guest: where a passed-through device appears in the guest's PCI
// topology is a question for whatever builds that topology.
type PCIDevice struct {
	Bus      uint8
	Slot     uint8
	Function uint8
}

// String renders the address as "bus/slot/function", for example
// "3/0/0".
//
// This is the form bhyve's own -s argument uses, and it is stable: it
// will not change to a colon-separated or hexadecimal spelling.
func (d PCIDevice) String() string {
	return fmt.Sprintf("%d/%d/%d", d.Bus, d.Slot, d.Function)
}

// validate rejects an address outside the PCI geometry.
//
// The fields are uint8, so Bus is always in range; Slot and Function are
// not, and a caller that computed them can get them wrong.
func (d PCIDevice) validate() error {
	if d.Slot > MaxPCISlot {
		return fmt.Errorf("vmm: pci slot %d is not 0 to %d: %w",
			d.Slot, MaxPCISlot, ErrInvalidArgument)
	}
	if d.Function > MaxPCIFunction {
		return fmt.Errorf("vmm: pci function %d is not 0 to %d: %w",
			d.Function, MaxPCIFunction, ErrInvalidArgument)
	}
	return nil
}

// ParsePCIDevice parses a "bus/slot/function" address, the form
// PCIDevice.String produces.
func ParsePCIDevice(s string) (PCIDevice, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return PCIDevice{}, fmt.Errorf("vmm: pci address %q is not bus/slot/function: %w",
			s, ErrInvalidArgument)
	}

	limits := [3]int{MaxPCIBus, MaxPCISlot, MaxPCIFunction}
	var fields [3]uint8

	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > limits[i] {
			return PCIDevice{}, fmt.Errorf("vmm: pci address %q has a bad field %q: %w",
				s, part, ErrInvalidArgument)
		}
		fields[i] = uint8(n)
	}

	return PCIDevice{Bus: fields[0], Slot: fields[1], Function: fields[2]}, nil
}
