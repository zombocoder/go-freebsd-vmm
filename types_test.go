//go:build freebsd && amd64

package vmm

import (
	"errors"
	"testing"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

func TestTopologyValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      Topology
		wantErr bool
	}{
		{name: "single cpu", in: Topology{1, 1, 1, 1}},
		{name: "four cores", in: Topology{1, 4, 1, 4}},
		{name: "two sockets smt", in: Topology{2, 4, 2, 16}},

		// MaxCPUs is read-only, so leaving it unset is not a
		// mistake: SetTopology does not send it, because
		// vm_set_topology(9) declares its maxcpus parameter
		// __unused.
		{name: "maxcpus unset", in: Topology{Sockets: 1, Cores: 2, Threads: 1}},

		{name: "zero sockets", in: Topology{0, 1, 1, 1}, wantErr: true},
		{name: "zero cores", in: Topology{1, 0, 1, 1}, wantErr: true},
		{name: "zero threads", in: Topology{1, 1, 0, 1}, wantErr: true},
		{name: "all zero", in: Topology{}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("validate(%v) = nil, want error", tc.in)
				}
				if !errors.Is(err, ErrInvalidArgument) {
					t.Errorf("validate(%v) = %v, want ErrInvalidArgument", tc.in, err)
				}
				return
			}
			if err != nil {
				t.Errorf("validate(%v) failed: %v", tc.in, err)
			}
		})
	}
}

func TestTopologyString(t *testing.T) {
	got := Topology{Sockets: 1, Cores: 4, Threads: 1, MaxCPUs: 4}.String()
	want := "sockets=1 cores=4 threads=1 maxcpus=4"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestMemoryProtectionString(t *testing.T) {
	tests := []struct {
		in   MemoryProtection
		want string
	}{
		{ProtNone, "---"},
		{ProtRead, "r--"},
		{ProtWrite, "-w-"},
		{ProtExec, "--x"},
		{ProtRead | ProtWrite, "rw-"},
		{ProtRead | ProtExec, "r-x"},
		{ProtRead | ProtWrite | ProtExec, "rwx"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("MemoryProtection(%d).String() = %q, want %q",
				int(tc.in), got, tc.want)
		}
	}
}

func TestMemoryFlagsString(t *testing.T) {
	tests := []struct {
		in   MemoryFlags
		want string
	}{
		{0, "0"},
		{MemoryInCore, "incore"},
		{MemoryWired, "wired"},
		{MemoryInCore | MemoryWired, "incore|wired"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("MemoryFlags(%d).String() = %q, want %q",
				uint32(tc.in), got, tc.want)
		}
	}
}

// TestMemoryFlagsMatchFreeBSD checks the flags are the kernel's bits
// rather than Go-side bit positions that happen to look right.
func TestMemoryFlagsMatchFreeBSD(t *testing.T) {
	if uint32(MemoryInCore) != uint32(vmmapi.MemFlagInCore) {
		t.Errorf("MemoryInCore = %#x, VM_MEM_F_INCORE = %#x",
			uint32(MemoryInCore), uint32(vmmapi.MemFlagInCore))
	}
	if uint32(MemoryWired) != uint32(vmmapi.MemFlagWired) {
		t.Errorf("MemoryWired = %#x, VM_MEM_F_WIRED = %#x",
			uint32(MemoryWired), uint32(vmmapi.MemFlagWired))
	}
	if MemoryInCore == MemoryWired {
		t.Error("MemoryInCore and MemoryWired are the same bit")
	}
}

func TestMemoryMappingStyleString(t *testing.T) {
	tests := []struct {
		in   MemoryMappingStyle
		want string
	}{
		{MemoryMapNone, "none"},
		{MemoryMapAll, "all"},
		{MemoryMapSparse, "sparse"},
		{MemoryMappingStyle(99), "unknown(99)"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestGuestAddressString(t *testing.T) {
	tests := []struct {
		in   GuestAddress
		want string
	}{
		{0, "0x0"},
		{0x1000, "0x1000"},
		{4 << 30, "0x100000000"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("GuestAddress(%d).String() = %q, want %q",
				uint64(tc.in), got, tc.want)
		}
	}
}

func TestMemoryMapEntryString(t *testing.T) {
	e := MemoryMapEntry{
		GuestAddress: 0,
		SegmentID:    0,
		Length:       0x10000000,
		Protection:   ProtRead | ProtWrite,
	}
	want := "0x0-0x10000000 seg=0 off=0x0 rw-"
	if got := e.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSuspendHowString(t *testing.T) {
	tests := []struct {
		in   SuspendHow
		want string
	}{
		{SuspendNone, "none"},
		{SuspendReset, "reset"},
		{SuspendPowerOff, "poweroff"},
		{SuspendHalt, "halt"},
		{SuspendTripleFault, "triplefault"},
		{SuspendHow(99), "unknown(99)"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestSuspendHowValid(t *testing.T) {
	valid := []SuspendHow{
		SuspendNone, SuspendReset, SuspendPowerOff, SuspendHalt,
		SuspendTripleFault,
	}
	for _, h := range valid {
		if !h.valid() {
			t.Errorf("%v.valid() = false, want true", h)
		}
	}

	for _, h := range []SuspendHow{-1, SuspendHow(vmmapi.SuspendLast), 99} {
		if h.valid() {
			t.Errorf("SuspendHow(%d).valid() = true, want false", int(h))
		}
	}
}

// TestSuspendHowAreDistinct guards the constants against collapsing onto
// one value, which would silently send the wrong reason to the kernel.
func TestSuspendHowAreDistinct(t *testing.T) {
	seen := map[SuspendHow]bool{}
	for _, h := range []SuspendHow{
		SuspendNone, SuspendReset, SuspendPowerOff, SuspendHalt,
		SuspendTripleFault,
	} {
		if seen[h] {
			t.Errorf("suspend reason %d appears twice", int(h))
		}
		seen[h] = true
	}
}

func TestTopologyVCPUs(t *testing.T) {
	tests := []struct {
		in   Topology
		want int
	}{
		{Topology{Sockets: 1, Cores: 1, Threads: 1}, 1},
		{Topology{Sockets: 1, Cores: 4, Threads: 1}, 4},
		{Topology{Sockets: 2, Cores: 4, Threads: 2}, 16},
		{Topology{}, 0},
	}
	for _, tc := range tests {
		if got := tc.in.VCPUs(); got != tc.want {
			t.Errorf("%v.VCPUs() = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestTopologyVCPUsDoesNotOverflow matters because the product is
// checked against the VM's vCPU count before it reaches the kernel, and
// uint16 arithmetic would wrap.
func TestTopologyVCPUsDoesNotOverflow(t *testing.T) {
	// 256 * 256 = 65536, which is 0 in uint16.
	got := Topology{Sockets: 256, Cores: 256, Threads: 1}.VCPUs()
	if got != 65536 {
		t.Errorf("VCPUs() = %d, want 65536", got)
	}
}
