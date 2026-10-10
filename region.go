//go:build freebsd && amd64

package vmm

import (
	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// MemoryRegion is the controlling process's mapping of a range of guest
// memory.
//
// It is unsafe in the semantic sense rather than the Go sense: the bytes
// it exposes are the guest's memory, so writing to them changes what the
// guest sees, immediately and without any synchronisation against the
// vCPUs that may be reading the same addresses. Treat anything read out
// of it as untrusted input, since the guest can change it at any moment.
//
// A region belongs to the VM it came from and stays valid only while
// that VM is open. The memory is owned by C and is not garbage
// collected, so a region must not be retained past its VM.
type MemoryRegion struct {
	vm     *VM
	addr   GuestAddress
	region *vmmapi.Region
}

// Address returns the guest physical address the region starts at.
//
// For a region returned by CreateDeviceMemory this is 0, because device
// memory has no guest address until MapMemorySegment gives it one.
func (r *MemoryRegion) Address() GuestAddress {
	if r == nil {
		return 0
	}
	return r.addr
}

// Size returns the length of the region in bytes.
func (r *MemoryRegion) Size() uint64 {
	if r == nil {
		return 0
	}
	return r.region.Size()
}

// Valid reports whether the region still holds a mapping and its VM is
// still open.
func (r *MemoryRegion) Valid() bool {
	if r == nil || !r.region.Valid() || r.vm == nil {
		return false
	}

	r.vm.mu.RLock()
	defer r.vm.mu.RUnlock()

	return r.vm.ctx != nil
}

// Bytes exposes the region as a byte slice aliasing guest memory.
//
// Reads see whatever the guest has written and writes are visible to the
// guest at once. The backing array is C memory: appending to the slice
// copies it rather than extending the mapping, and keeping the slice
// alive does not keep the mapping alive.
//
// Bytes returns nil once the region or its VM is no longer valid.
func (r *MemoryRegion) Bytes() []byte {
	if !r.Valid() {
		return nil
	}
	return r.region.Bytes()
}
