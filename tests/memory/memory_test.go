//go:build freebsd && amd64

// Package memory_test exercises guest memory against a real kernel:
// allocation, host mappings, segments and the guest address space map.
package memory_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	vmm "github.com/zombocoder/go-freebsd-vmm"
	"github.com/zombocoder/go-freebsd-vmm/tests/harness"
)

// guestMemory is small enough to stay inside the low memory region, so
// the tests do not depend on the high memory layout.
const guestMemory = 256 << 20

func TestMain(m *testing.M) {
	os.Exit(harness.Run(m))
}

func TestSetupMemorySizes(t *testing.T) {
	vm := harness.RequireVM(t)

	if got := vm.LowMemorySize(); got != 0 {
		t.Errorf("LowMemorySize() before SetupMemory = %d, want 0", got)
	}

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory(%d) = %v", guestMemory, err)
	}

	if got := vm.LowMemorySize(); got != guestMemory {
		t.Errorf("LowMemorySize() = %d, want %d", got, guestMemory)
	}
	if got := vm.HighMemorySize(); got != 0 {
		t.Errorf("HighMemorySize() = %d, want 0 for a VM below the low memory limit", got)
	}
	if got := vm.HighMemoryBase(); got != 0 {
		t.Errorf("HighMemoryBase() = %v, want 0 when there is no high memory", got)
	}

	t.Logf("low memory limit: %#x", vm.LowMemoryLimit())
}

// TestSetupMemoryAboveLowLimit covers the split layout: memory past the
// low memory limit goes to a second region starting at HighMemoryBase,
// leaving the 32-bit MMIO hole clear.
func TestSetupMemoryAboveLowLimit(t *testing.T) {
	vm := harness.RequireVM(t)

	// Keep the limit small so the test does not have to allocate
	// several gigabytes to cross it.
	const limit = 64 << 20
	const size = 192 << 20

	if err := vm.SetLowMemoryLimit(limit); err != nil {
		t.Fatalf("SetLowMemoryLimit(%d) = %v", limit, err)
	}
	if got := vm.LowMemoryLimit(); got != limit {
		t.Fatalf("LowMemoryLimit() = %d, want %d", got, limit)
	}

	if err := vm.SetupMemory(size, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory(%d) = %v", size, err)
	}

	low, high := vm.LowMemorySize(), vm.HighMemorySize()
	if low != limit {
		t.Errorf("LowMemorySize() = %d, want %d", low, limit)
	}
	if high != size-limit {
		t.Errorf("HighMemorySize() = %d, want %d", high, size-limit)
	}
	if low+high != size {
		t.Errorf("low + high = %d, want %d", low+high, size)
	}

	base := vm.HighMemoryBase()
	if base == 0 {
		t.Error("HighMemoryBase() = 0 but there is high memory")
	}
	t.Logf("low=%d high=%d base=%v", low, high, base)
}

// TestMapGuestMemoryRoundTrip is the test that the mapping really
// aliases guest memory: a write through one mapping is visible through
// another of the same addresses.
func TestMapGuestMemoryRoundTrip(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	const offset = 0x100000
	const length = 4096

	region, err := vm.MapGuestMemory(offset, length)
	if err != nil {
		t.Fatalf("MapGuestMemory(%#x, %d) = %v", offset, length, err)
	}
	if !region.Valid() {
		t.Fatal("MapGuestMemory returned an invalid region")
	}
	if got := region.Size(); got != length {
		t.Errorf("Size() = %d, want %d", got, length)
	}
	if got := region.Address(); got != offset {
		t.Errorf("Address() = %v, want %#x", got, offset)
	}

	want := []byte("go-freebsd-vmm guest memory probe")
	buf := region.Bytes()
	if len(buf) != length {
		t.Fatalf("Bytes() has length %d, want %d", len(buf), length)
	}
	copy(buf, want)

	// A second mapping of the same guest addresses sees the write.
	again, err := vm.MapGuestMemory(offset, length)
	if err != nil {
		t.Fatalf("MapGuestMemory a second time = %v", err)
	}
	if got := again.Bytes()[:len(want)]; !bytes.Equal(got, want) {
		t.Errorf("second mapping reads %q, want %q", got, want)
	}

	// And the page beyond the written bytes is untouched.
	if trailing := buf[len(want):]; bytes.Count(trailing, []byte{0}) != len(trailing) {
		t.Error("the bytes after the probe are not zero")
	}
}

// TestRegionIsInvalidatedWithTheVM keeps a region from handing out a
// slice over memory whose VM has gone away.
func TestRegionIsInvalidatedWithTheVM(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	region, err := vm.MapGuestMemory(0, 4096)
	if err != nil {
		t.Fatalf("MapGuestMemory = %v", err)
	}
	if !region.Valid() {
		t.Fatal("region is invalid before the VM closed")
	}

	if err := vm.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	if region.Valid() {
		t.Error("Valid() = true after the VM closed, want false")
	}
	if region.Bytes() != nil {
		t.Error("Bytes() returned a slice after the VM closed")
	}
}

func TestMapGuestMemoryOutsideGuestMemory(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	// Well past the end of the guest's memory.
	if _, err := vm.MapGuestMemory(vmm.GuestAddress(guestMemory)*4, 4096); err == nil {
		t.Error("MapGuestMemory past the end of guest memory succeeded")
	}
}

func TestReverseMap(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	const base = 0x200000
	region, err := vm.MapGuestMemory(base, 8192)
	if err != nil {
		t.Fatalf("MapGuestMemory = %v", err)
	}

	for _, off := range []uint64{0, 1, 4095, 4096, 8191} {
		got, err := vm.ReverseMap(region, off)
		if err != nil {
			t.Fatalf("ReverseMap(off=%d) = %v", off, err)
		}
		if want := vmm.GuestAddress(base + off); got != want {
			t.Errorf("ReverseMap(off=%d) = %v, want %v", off, got, want)
		}
	}

	// An offset past the end of the region is refused rather than
	// producing a bogus address.
	if _, err := vm.ReverseMap(region, 8192); !errors.Is(err, vmm.ErrInvalidArgument) {
		t.Errorf("ReverseMap past the region = %v, want ErrInvalidArgument", err)
	}
}

func TestMemoryFlagsRoundTrip(t *testing.T) {
	vm := harness.RequireVM(t)

	before := vm.MemoryFlags()
	t.Logf("default memory flags: %v", before)

	if err := vm.SetMemoryFlags(vmm.MemoryInCore); err != nil {
		t.Fatalf("SetMemoryFlags(MemoryInCore) = %v", err)
	}
	if got := vm.MemoryFlags(); got != vmm.MemoryInCore {
		t.Errorf("MemoryFlags() = %v, want %v", got, vmm.MemoryInCore)
	}

	both := vmm.MemoryInCore | vmm.MemoryWired
	if err := vm.SetMemoryFlags(both); err != nil {
		t.Fatalf("SetMemoryFlags(%v) = %v", both, err)
	}
	if got := vm.MemoryFlags(); got != both {
		t.Errorf("MemoryFlags() = %v, want %v", got, both)
	}

	if err := vm.SetMemoryFlags(before); err != nil {
		t.Errorf("restoring memory flags to %v = %v", before, err)
	}
}

func TestSystemMemorySegment(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	seg, err := vm.MemorySegment(vmm.SegmentSysMem)
	if err != nil {
		t.Fatalf("MemorySegment(SegmentSysMem) = %v", err)
	}
	if seg.ID != vmm.SegmentSysMem {
		t.Errorf("ID = %d, want %d", seg.ID, vmm.SegmentSysMem)
	}
	// System memory segments are identified by a nul name.
	if seg.Name != "" {
		t.Errorf("Name = %q, want empty for system memory", seg.Name)
	}
	if seg.Length < guestMemory {
		t.Errorf("Length = %d, want at least %d", seg.Length, guestMemory)
	}
}

func TestDeviceMemory(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	const romSize = 64 << 10
	const romName = "testrom"

	region, err := vm.CreateDeviceMemory(vmm.SegmentBootROM, romName, romSize)
	if err != nil {
		t.Fatalf("CreateDeviceMemory(SegmentBootROM, %q, %d) = %v",
			romName, romSize, err)
	}
	if got := region.Size(); got != romSize {
		t.Errorf("Size() = %d, want %d", got, romSize)
	}
	// Device memory has no guest address until it is mapped.
	if got := region.Address(); got != 0 {
		t.Errorf("Address() = %v, want 0 before mapping", got)
	}

	seg, err := vm.MemorySegment(vmm.SegmentBootROM)
	if err != nil {
		t.Fatalf("MemorySegment(SegmentBootROM) = %v", err)
	}
	if seg.Name != romName {
		t.Errorf("segment name = %q, want %q", seg.Name, romName)
	}
	if seg.Length != romSize {
		t.Errorf("segment length = %d, want %d", seg.Length, romSize)
	}

	// Fill it, then place it in the guest address space just below
	// 4 GiB, where a boot ROM conventionally lives.
	want := bytes.Repeat([]byte{0xcc}, 16)
	copy(region.Bytes(), want)

	gpa := vmm.GuestAddress(uint64(4<<30) - romSize)
	if err := vm.MapMemorySegment(gpa, vmm.SegmentBootROM, 0, romSize,
		vmm.ProtRead|vmm.ProtExec); err != nil {
		t.Fatalf("MapMemorySegment(%v) = %v", gpa, err)
	}

	mappings, err := vm.MemoryMappings()
	if err != nil {
		t.Fatalf("MemoryMappings() = %v", err)
	}

	var found bool
	for _, m := range mappings {
		if m.GuestAddress == gpa && m.SegmentID == vmm.SegmentBootROM {
			found = true
			if m.Length != romSize {
				t.Errorf("mapping length = %d, want %d", m.Length, romSize)
			}
			if m.Protection&vmm.ProtWrite != 0 {
				t.Errorf("mapping protection = %v, want no write", m.Protection)
			}
		}
	}
	if !found {
		t.Errorf("MemoryMappings() = %v, want an entry for the ROM at %v",
			mappings, gpa)
	}

	if err := vm.UnmapMemorySegment(gpa, romSize); err != nil {
		t.Errorf("UnmapMemorySegment(%v) = %v", gpa, err)
	}
}

// TestMemoryMappingsCoverGuestMemory checks the whole-map walk, which
// happens in C in one transition, against the sizes the VM reports.
func TestMemoryMappingsCoverGuestMemory(t *testing.T) {
	vm := harness.RequireVM(t)

	if err := vm.SetupMemory(guestMemory, vmm.MemoryMapAll); err != nil {
		t.Fatalf("SetupMemory = %v", err)
	}

	mappings, err := vm.MemoryMappings()
	if err != nil {
		t.Fatalf("MemoryMappings() = %v", err)
	}
	if len(mappings) == 0 {
		t.Fatal("MemoryMappings() is empty after SetupMemory")
	}

	var total uint64
	var prevEnd uint64
	for i, m := range mappings {
		t.Logf("mapping %d: %v", i, m)

		if m.Length == 0 {
			t.Errorf("mapping %d has zero length", i)
		}
		if uint64(m.GuestAddress) < prevEnd {
			t.Errorf("mapping %d starts at %v, before the end of the previous one (%#x)",
				i, m.GuestAddress, prevEnd)
		}
		prevEnd = uint64(m.GuestAddress) + m.Length

		if m.SegmentID == vmm.SegmentSysMem {
			total += m.Length
		}
	}

	if want := vm.LowMemorySize() + vm.HighMemorySize(); total != want {
		t.Errorf("system memory mappings total %d, want %d", total, want)
	}
}
