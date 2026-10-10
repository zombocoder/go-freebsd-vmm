//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// GuestAddress is a guest physical address.
type GuestAddress uint64

// String renders the address in hexadecimal.
func (a GuestAddress) String() string { return fmt.Sprintf("%#x", uint64(a)) }

// MemoryMappingStyle selects how guest memory is mapped into the
// controlling process.
type MemoryMappingStyle int

const (
	// MemoryMapNone allocates guest memory without mapping it into
	// the controlling process.
	MemoryMapNone MemoryMappingStyle = vmmapi.MMapNone

	// MemoryMapAll maps all of guest memory up front.
	MemoryMapAll MemoryMappingStyle = vmmapi.MMapAll

	// MemoryMapSparse creates the mappings on demand.
	MemoryMapSparse MemoryMappingStyle = vmmapi.MMapSparse
)

// String renders the style.
func (s MemoryMappingStyle) String() string {
	switch s {
	case MemoryMapNone:
		return "none"
	case MemoryMapAll:
		return "all"
	case MemoryMapSparse:
		return "sparse"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// MemoryFlags controls how guest memory is treated by the host.
type MemoryFlags uint32

const (
	// MemoryInCore includes guest memory in a core dump of the
	// controlling process.
	MemoryInCore MemoryFlags = vmmapi.MemFlagInCore

	// MemoryWired keeps guest memory resident, never paged out.
	MemoryWired MemoryFlags = vmmapi.MemFlagWired
)

// String renders the flags as a "|"-separated list, or "0" when none
// are set.
func (f MemoryFlags) String() string {
	switch {
	case f == 0:
		return "0"
	case f == MemoryInCore:
		return "incore"
	case f == MemoryWired:
		return "wired"
	case f == MemoryInCore|MemoryWired:
		return "incore|wired"
	default:
		return fmt.Sprintf("%#x", uint32(f))
	}
}

// MemoryProtection is a set of access rights on a guest memory mapping,
// built from the ProtRead, ProtWrite and ProtExec bits.
type MemoryProtection int

const (
	// ProtNone permits no access.
	ProtNone MemoryProtection = vmmapi.ProtNone

	// ProtRead permits reads.
	ProtRead MemoryProtection = vmmapi.ProtRead

	// ProtWrite permits writes.
	ProtWrite MemoryProtection = vmmapi.ProtWrite

	// ProtExec permits instruction fetch.
	ProtExec MemoryProtection = vmmapi.ProtExec
)

// String renders the protection as "rwx", with a dash for each absent
// right.
func (p MemoryProtection) String() string {
	out := []byte("---")
	if p&ProtRead != 0 {
		out[0] = 'r'
	}
	if p&ProtWrite != 0 {
		out[1] = 'w'
	}
	if p&ProtExec != 0 {
		out[2] = 'x'
	}
	return string(out)
}

// Memory segment identifiers. SegmentSysMem holds the guest's system
// memory and is populated by SetupMemory; the others are available for
// device memory.
const (
	SegmentSysMem      int = vmmapi.SegSysMem
	SegmentBootROM     int = vmmapi.SegBootROM
	SegmentFramebuffer int = vmmapi.SegFramebuffer
	SegmentPCIROM      int = vmmapi.SegPCIROM
)

// MemorySegment describes one memory segment of a VM. System memory
// segments have an empty name.
type MemorySegment struct {
	ID     int
	Name   string
	Length uint64
}

// MemoryMapEntry is one range of the guest address space.
type MemoryMapEntry struct {
	GuestAddress  GuestAddress
	SegmentID     int
	SegmentOffset uint64
	Length        uint64
	Protection    MemoryProtection
	Flags         uint32
}

// String renders the entry as
// "0x0-0x10000000 seg=0 off=0x0 rw-".
func (e MemoryMapEntry) String() string {
	return fmt.Sprintf("%#x-%#x seg=%d off=%#x %s",
		uint64(e.GuestAddress), uint64(e.GuestAddress)+e.Length,
		e.SegmentID, e.SegmentOffset, e.Protection)
}

// SetupMemory allocates size bytes of system memory for the VM and maps
// it according to style.
//
// Memory above the low memory limit, 3 GiB by default, is placed in a
// second region starting at HighMemoryBase so that the guest's 32-bit
// MMIO hole stays clear.
//
// Only MemoryMapAll is accepted on FreeBSD 14. The other styles are
// refused here with ErrUnsupported rather than passed down, because
// vm_setup_memory(3) asserts on them and would abort the process instead
// of returning an error. Supports(FeatureSparseMemory) reports whether
// that has changed.
func (vm *VM) SetupMemory(size uint64, style MemoryMappingStyle) error {
	const op = "vm_setup_memory"

	if size == 0 {
		return fmt.Errorf("vmm: memory size is 0: %w", ErrInvalidArgument)
	}
	if style != MemoryMapAll && !Supports(FeatureSparseMemory) {
		return fmt.Errorf("vmm: memory mapping style %s needs a newer libvmmapi: %w",
			style, ErrUnsupported)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.SetupMemory(size, int(style)))
}

// LowMemorySize returns the size of the guest's low memory region, the
// part below LowMemoryLimit. It is 0 before SetupMemory has run.
func (vm *VM) LowMemorySize() uint64 {
	ctx, release, err := vm.context("vm_get_lowmem_size")
	if err != nil {
		return 0
	}
	defer release()

	return ctx.LowMemSize()
}

// HighMemorySize returns the size of the guest's high memory region,
// which is 0 unless the VM was given more memory than LowMemoryLimit.
func (vm *VM) HighMemorySize() uint64 {
	ctx, release, err := vm.context("vm_get_highmem_size")
	if err != nil {
		return 0
	}
	defer release()

	return ctx.HighMemSize()
}

// HighMemoryBase returns the guest physical address where the high
// memory region starts, or 0 when the VM has none.
//
// FreeBSD 14's libvmmapi has no accessor for this; the address is a
// 4 GiB constant inside its memory layout. MemoryMappings reports what
// the kernel actually has mapped, which is the authoritative answer.
func (vm *VM) HighMemoryBase() GuestAddress {
	if vm.HighMemorySize() == 0 {
		return 0
	}
	return GuestAddress(vmmapi.HighMemBase)
}

// LowMemoryLimit returns the guest physical address at which low memory
// stops. Memory beyond it goes to the high memory region.
func (vm *VM) LowMemoryLimit() uint32 {
	ctx, release, err := vm.context("vm_get_lowmem_limit")
	if err != nil {
		return 0
	}
	defer release()

	return ctx.LowMemLimit()
}

// SetLowMemoryLimit changes where low memory stops. It only affects
// later SetupMemory calls, not memory already allocated.
func (vm *VM) SetLowMemoryLimit(limit uint32) error {
	const op = "vm_set_lowmem_limit"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	ctx.SetLowMemLimit(limit)
	return nil
}

// MemoryFlags returns the current guest memory flags.
func (vm *VM) MemoryFlags() MemoryFlags {
	ctx, release, err := vm.context("vm_get_memflags")
	if err != nil {
		return 0
	}
	defer release()

	return MemoryFlags(ctx.MemFlags())
}

// SetMemoryFlags sets the guest memory flags. They only affect later
// mapping operations, so this belongs before SetupMemory.
func (vm *VM) SetMemoryFlags(flags MemoryFlags) error {
	const op = "vm_set_memflags"

	known := MemoryInCore | MemoryWired
	if flags&^known != 0 {
		return fmt.Errorf("vmm: unknown memory flags %#x: %w",
			uint32(flags&^known), ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	ctx.SetMemFlags(int(flags))
	return nil
}

// MapGuestMemory returns the controlling process's mapping of
// [addr, addr+size) in guest physical memory.
//
// The returned region aliases the guest's memory: writing through it
// writes to the guest, with no synchronisation against the vCPUs that
// may be reading the same bytes. The region stays valid only while the
// VM is open.
func (vm *VM) MapGuestMemory(addr GuestAddress, size uint64) (*MemoryRegion, error) {
	const op = "vm_map_gpa"

	if size == 0 {
		return nil, fmt.Errorf("vmm: mapping size is 0: %w", ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return nil, err
	}
	defer release()

	region, errno := ctx.MapGPA(uint64(addr), size)
	if errno != 0 {
		return nil, vm.err(op, errno)
	}
	return &MemoryRegion{vm: vm, addr: addr, region: region}, nil
}

// ReverseMap returns the guest physical address that offset off within
// region corresponds to.
func (vm *VM) ReverseMap(region *MemoryRegion, off uint64) (GuestAddress, error) {
	const op = "vm_rev_map_gpa"

	if region == nil || !region.region.Valid() {
		return 0, fmt.Errorf("vmm: %s on invalid region: %w", op, ErrInvalidArgument)
	}
	// Checked here, not left to the bridge, so that an offset outside
	// the region reports what is wrong with it rather than arriving as
	// an errno from pointer arithmetic.
	if off >= region.Size() {
		return 0, fmt.Errorf("vmm: offset %d is outside the %d byte region: %w",
			off, region.Size(), ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return 0, err
	}
	defer release()

	gpa, errno := ctx.RevMapGPA(region.region, off)
	if errno != 0 {
		return 0, vm.err(op, errno)
	}
	return GuestAddress(gpa), nil
}

// MemorySegment returns the length and name of a memory segment.
func (vm *VM) MemorySegment(id int) (MemorySegment, error) {
	const op = "vm_get_memseg"

	ctx, release, err := vm.context(op)
	if err != nil {
		return MemorySegment{}, err
	}
	defer release()

	length, name, errno := ctx.MemSeg(id)
	if errno != 0 {
		return MemorySegment{}, vm.err(op, errno)
	}
	return MemorySegment{ID: id, Name: name, Length: length}, nil
}

// CreateDeviceMemory creates a device memory segment of size bytes and
// maps it into the controlling process.
//
// Device memory is backed by the host but is not part of guest system
// memory. It becomes visible to the guest only once MapMemorySegment has
// placed it at a guest physical address, which is how a boot ROM or a
// framebuffer is given to a guest.
func (vm *VM) CreateDeviceMemory(id int, name string, size uint64) (*MemoryRegion, error) {
	const op = "vm_create_devmem"

	if size == 0 {
		return nil, fmt.Errorf("vmm: device memory size is 0: %w", ErrInvalidArgument)
	}
	if id == SegmentSysMem {
		return nil, fmt.Errorf("vmm: segment %d is system memory, not device memory: %w",
			id, ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return nil, err
	}
	defer release()

	region, errno := ctx.CreateDevMem(id, name, size)
	if errno != 0 {
		return nil, vm.err(op, errno)
	}
	return &MemoryRegion{vm: vm, addr: 0, region: region}, nil
}

// MapMemorySegment maps [offset, offset+size) of a memory segment into
// the guest address space at gpa with the given protection.
func (vm *VM) MapMemorySegment(gpa GuestAddress, segmentID int, offset, size uint64, prot MemoryProtection) error {
	const op = "vm_mmap_memseg"

	if size == 0 {
		return fmt.Errorf("vmm: mapping size is 0: %w", ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.MMapMemSeg(uint64(gpa), segmentID, offset, size,
		int(prot)))
}

// UnmapMemorySegment removes the mapping covering [gpa, gpa+size) from
// the guest address space.
func (vm *VM) UnmapMemorySegment(gpa GuestAddress, size uint64) error {
	const op = "vm_munmap_memseg"

	if size == 0 {
		return fmt.Errorf("vmm: unmapping size is 0: %w", ErrInvalidArgument)
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.MUnmapMemSeg(uint64(gpa), size))
}

// MemoryMappings returns the guest address space as the kernel has it,
// in increasing address order.
//
// The whole space is walked in one cgo transition, so the cost does not
// grow with the number of ranges.
func (vm *VM) MemoryMappings() ([]MemoryMapEntry, error) {
	const op = "vm_mmap_getnext"

	ctx, release, err := vm.context(op)
	if err != nil {
		return nil, err
	}
	defer release()

	maps, errno := ctx.MemMaps()
	if errno != 0 {
		return nil, vm.err(op, errno)
	}

	out := make([]MemoryMapEntry, len(maps))
	for i, m := range maps {
		out[i] = MemoryMapEntry{
			GuestAddress:  GuestAddress(m.GPA),
			SegmentID:     m.SegID,
			SegmentOffset: m.SegOff,
			Length:        m.Length,
			Protection:    MemoryProtection(m.Prot),
			Flags:         m.Flags,
		}
	}
	return out, nil
}
