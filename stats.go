//go:build freebsd && amd64

package vmm

import "time"

// Stat is one VMM statistics counter for a vCPU.
//
// Index is the counter's position in the kernel's table. It is stable
// for a given kernel but not across kernels, so match on Description
// rather than Index when the value has to mean the same thing across
// hosts.
type Stat struct {
	Index       int
	Description string
	Value       uint64
}

// StatsSnapshot is a set of counters with the time the kernel sampled
// them.
//
// The timestamp comes from the kernel, not from the host clock at the
// moment of the call, which is what makes rates computed from two
// snapshots meaningful.
type StatsSnapshot struct {
	Time  time.Time
	Stats []Stat
}

// Stats reads every statistics counter for this vCPU.
//
// The call is serialised across the whole process. libvmmapi hands back
// pointers into static storage here, one buffer thread-local and one
// genuinely shared, so the values and their descriptions are copied out
// under a lock before this returns. Concurrent callers are therefore
// safe but do not proceed in parallel.
func (cpu *VCPU) Stats() ([]Stat, error) {
	snapshot, err := cpu.StatsSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.Stats, nil
}

// StatsSnapshot reads every statistics counter for this vCPU along with
// the kernel's timestamp for the sample.
func (cpu *VCPU) StatsSnapshot() (StatsSnapshot, error) {
	const op = "vm_get_stats"

	h, release, err := cpu.handle(op)
	if err != nil {
		return StatsSnapshot{}, err
	}
	defer release()

	// The statistics path needs both the context, for the counter
	// descriptions, and the vCPU, for the values.
	entries, sampled, errno := cpu.vm.ctx.Stats(h)
	if errno != 0 {
		return StatsSnapshot{}, cpu.err(op, errno)
	}

	stats := make([]Stat, len(entries))
	for i, e := range entries {
		stats[i] = Stat{
			Index:       e.Index,
			Description: e.Description,
			Value:       e.Value,
		}
	}
	return StatsSnapshot{Time: sampled, Stats: stats}, nil
}
