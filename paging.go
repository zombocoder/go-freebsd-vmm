//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// CPUMode is the mode the guest processor is executing in.
type CPUMode int

const (
	// CPUModeReal is 16-bit real mode.
	CPUModeReal CPUMode = vmmapi.CPUModeReal

	// CPUModeProtected is 32-bit protected mode.
	CPUModeProtected CPUMode = vmmapi.CPUModeProtected

	// CPUModeCompatibility is IA-32e compatibility mode, long mode
	// with CS.L clear.
	CPUModeCompatibility CPUMode = vmmapi.CPUModeCompatibility

	// CPUMode64Bit is IA-32e 64-bit mode, long mode with CS.L set.
	CPUMode64Bit CPUMode = vmmapi.CPUMode64Bit
)

// String renders the mode, falling back to its number for a mode this
// package does not name.
func (m CPUMode) String() string {
	switch m {
	case CPUModeReal:
		return "real"
	case CPUModeProtected:
		return "protected"
	case CPUModeCompatibility:
		return "compatibility"
	case CPUMode64Bit:
		return "64bit"
	default:
		return fmt.Sprintf("cpumode(%d)", int(m))
	}
}

// PagingMode is the paging structure the guest is using.
type PagingMode int

const (
	// PagingModeFlat is paging disabled, linear equals physical.
	PagingModeFlat PagingMode = vmmapi.PagingModeFlat

	// PagingMode32 is 32-bit two-level paging.
	PagingMode32 PagingMode = vmmapi.PagingMode32

	// PagingModePAE is 32-bit physical address extension paging.
	PagingModePAE PagingMode = vmmapi.PagingModePAE

	// PagingMode64 is four-level long mode paging.
	PagingMode64 PagingMode = vmmapi.PagingMode64

	// PagingMode64LA57 is five-level long mode paging, 57-bit linear
	// addresses.
	PagingMode64LA57 PagingMode = vmmapi.PagingMode64LA57
)

// String renders the mode, falling back to its number for a mode this
// package does not name.
func (m PagingMode) String() string {
	switch m {
	case PagingModeFlat:
		return "flat"
	case PagingMode32:
		return "32"
	case PagingModePAE:
		return "pae"
	case PagingMode64:
		return "64"
	case PagingMode64LA57:
		return "64-la57"
	default:
		return fmt.Sprintf("pagingmode(%d)", int(m))
	}
}

// PagingState is everything needed to translate a guest linear address:
// where the guest's page tables are, what privilege level the access is
// made at, and which paging structure to walk.
//
// A VM exit that involves a memory access reports the state it happened
// under, so the usual way to get one is from an exit rather than by
// assembling it from registers.
type PagingState struct {
	CR3        uint64
	CPL        int
	CPUMode    CPUMode
	PagingMode PagingMode
}

// String renders the state as
// "cr3=0x1000 cpl=0 mode=64bit paging=64".
func (p PagingState) String() string {
	return fmt.Sprintf("cr3=%#x cpl=%d mode=%s paging=%s",
		p.CR3, p.CPL, p.CPUMode, p.PagingMode)
}

// validate rejects a state the kernel would refuse, before it reaches C.
func (p PagingState) validate() error {
	if p.CPL < 0 || p.CPL > 3 {
		return fmt.Errorf("vmm: paging cpl %d is not 0 to 3: %w",
			p.CPL, ErrInvalidArgument)
	}
	switch p.CPUMode {
	case CPUModeReal, CPUModeProtected, CPUModeCompatibility, CPUMode64Bit:
	default:
		return fmt.Errorf("vmm: unknown cpu mode %d: %w",
			int(p.CPUMode), ErrInvalidArgument)
	}
	switch p.PagingMode {
	case PagingModeFlat, PagingMode32, PagingModePAE, PagingMode64,
		PagingMode64LA57:
	default:
		return fmt.Errorf("vmm: unknown paging mode %d: %w",
			int(p.PagingMode), ErrInvalidArgument)
	}
	return nil
}

// bridge converts the state for the internal bridge.
func (p PagingState) bridge() vmmapi.Paging {
	return vmmapi.Paging{
		CR3:        p.CR3,
		CPL:        p.CPL,
		CPUMode:    int(p.CPUMode),
		PagingMode: int(p.PagingMode),
	}
}

// pagingFromRaw builds a PagingState from a decoded exit.
func pagingFromRaw(cr3 uint64, cpl, cpuMode, pagingMode int32) PagingState {
	return PagingState{
		CR3:        cr3,
		CPL:        int(cpl),
		CPUMode:    CPUMode(cpuMode),
		PagingMode: PagingMode(pagingMode),
	}
}
