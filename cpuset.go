//go:build freebsd && amd64

package vmm

import (
	"math/bits"
	"strconv"
	"strings"
	"syscall"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// cpuSetWordBits is the number of CPUs described by one word of a
// CPUSet's bitmap.
const cpuSetWordBits = 64

// CPUSet is a set of virtual CPU ids.
//
// It is the Go form of the kernel's cpuset_t, which is not exposed. A
// CPUSet is a value: copying one copies the set.
type CPUSet struct {
	words []uint64
}

// Contains reports whether id is in the set.
func (s CPUSet) Contains(id int) bool {
	if id < 0 {
		return false
	}
	word := id / cpuSetWordBits
	if word >= len(s.words) {
		return false
	}
	return s.words[word]&(1<<uint(id%cpuSetWordBits)) != 0
}

// Len returns how many CPUs are in the set.
func (s CPUSet) Len() int {
	n := 0
	for _, w := range s.words {
		n += bits.OnesCount64(w)
	}
	return n
}

// IDs returns the CPU ids in the set, in increasing order.
func (s CPUSet) IDs() []int {
	out := make([]int, 0, s.Len())
	for i, w := range s.words {
		for w != 0 {
			bit := bits.TrailingZeros64(w)
			out = append(out, i*cpuSetWordBits+bit)
			w &= w - 1
		}
	}
	return out
}

// String renders the set as a comma-separated list of ids, for example
// "0,1,2,3". An empty set renders as "[]".
func (s CPUSet) String() string {
	ids := s.IDs()
	if len(ids) == 0 {
		return "[]"
	}

	var b strings.Builder
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(id))
	}
	return b.String()
}

// cpuSetOf runs one of the CPU state queries.
func (vm *VM) cpuSetOf(op string, fn func(*vmmapi.Context) ([]uint64, syscall.Errno)) (CPUSet, error) {
	ctx, release, err := vm.context(op)
	if err != nil {
		return CPUSet{}, err
	}
	defer release()

	words, errno := fn(ctx)
	if errno != 0 {
		return CPUSet{}, vm.err(op, errno)
	}
	return CPUSet{words: words}, nil
}

// ActiveCPUs returns the vCPUs the kernel considers active.
func (vm *VM) ActiveCPUs() (CPUSet, error) {
	return vm.cpuSetOf("vm_active_cpus", (*vmmapi.Context).ActiveCPUs)
}

// SuspendedCPUs returns the vCPUs that are suspended.
func (vm *VM) SuspendedCPUs() (CPUSet, error) {
	return vm.cpuSetOf("vm_suspended_cpus", (*vmmapi.Context).SuspendedCPUs)
}

// DebugCPUs returns the vCPUs halted for debugging.
func (vm *VM) DebugCPUs() (CPUSet, error) {
	return vm.cpuSetOf("vm_debug_cpus", (*vmmapi.Context).DebugCPUs)
}
