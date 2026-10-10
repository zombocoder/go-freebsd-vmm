//go:build freebsd && amd64

package vmmapi

import "unsafe"

// This file is the only place in the project where a C pointer is turned
// into a Go slice. Keeping every such conversion here is what lets the
// rest of the code be read without auditing for pointer tricks.

// Region is a host mapping of guest memory. The memory belongs to C: it
// was mapped by libvmmapi into the controlling process and is not
// managed by the Go garbage collector.
//
// A Region stays valid only while the context it came from is open.
type Region struct {
	ptr  unsafe.Pointer
	size uint64
}

// newRegion wraps a mapping returned by C.
func newRegion(ptr unsafe.Pointer, size uint64) *Region {
	if ptr == nil || size == 0 {
		return nil
	}
	return &Region{ptr: ptr, size: size}
}

// Valid reports whether the region holds a mapping.
func (r *Region) Valid() bool { return r != nil && r.ptr != nil && r.size > 0 }

// Size returns the length of the mapping in bytes.
func (r *Region) Size() uint64 {
	if r == nil {
		return 0
	}
	return r.size
}

// Bytes exposes the mapping as a Go slice.
//
// The slice aliases guest physical memory directly: writing to it writes
// to the guest. Its backing array is C memory, so it must not be
// retained past the lifetime of the owning context, and appending to it
// copies rather than extending the mapping.
func (r *Region) Bytes() []byte {
	if !r.Valid() {
		return nil
	}
	return unsafe.Slice((*byte)(r.ptr), r.size)
}

// pointerAt returns a host pointer to offset off within the region,
// reporting whether the offset is inside it.
func (r *Region) pointerAt(off uint64) (unsafe.Pointer, bool) {
	if !r.Valid() || off >= r.size {
		return nil, false
	}
	return unsafe.Add(r.ptr, uintptr(off)), true
}
