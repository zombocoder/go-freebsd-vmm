//go:build freebsd && amd64

package vmm

import "fmt"

// Topology describes how a VM's virtual CPUs are arranged.
//
// MaxCPUs is read-only. It is the number of vCPUs the VM has, which is
// what bounds the ids VM.VCPU accepts, and it is fixed when the VM is
// created: the kernel takes it from the hw.vmm.maxcpu tunable, which
// defaults to the number of host CPUs. SetTopology ignores the field,
// because vm_set_topology(9) declares its maxcpus parameter __unused.
//
// What SetTopology does change is how those vCPUs are presented to the
// guest. Sockets*Cores*Threads must not exceed MaxCPUs.
type Topology struct {
	Sockets uint16
	Cores   uint16
	Threads uint16
	MaxCPUs uint16
}

// VCPUs returns how many vCPUs the arrangement describes.
func (t Topology) VCPUs() int {
	return int(t.Sockets) * int(t.Cores) * int(t.Threads)
}

// String renders the topology as "sockets=1 cores=4 threads=1 maxcpus=4".
func (t Topology) String() string {
	return fmt.Sprintf("sockets=%d cores=%d threads=%d maxcpus=%d",
		t.Sockets, t.Cores, t.Threads, t.MaxCPUs)
}

// validate rejects the topologies that are obviously wrong, leaving the
// rest to the kernel. Reproducing every kernel rule here would only
// drift out of step with it.
//
// MaxCPUs is not checked: SetTopology does not send it, so a caller that
// leaves it zero is not making a mistake.
func (t Topology) validate() error {
	switch {
	case t.Sockets == 0:
		return fmt.Errorf("vmm: topology has 0 sockets: %w", ErrInvalidArgument)
	case t.Cores == 0:
		return fmt.Errorf("vmm: topology has 0 cores: %w", ErrInvalidArgument)
	case t.Threads == 0:
		return fmt.Errorf("vmm: topology has 0 threads: %w", ErrInvalidArgument)
	}
	return nil
}

// Topology reads the VM's CPU topology.
func (vm *VM) Topology() (Topology, error) {
	const op = "vm_get_topology"

	ctx, release, err := vm.context(op)
	if err != nil {
		return Topology{}, err
	}
	defer release()

	sockets, cores, threads, maxcpus, errno := ctx.Topology()
	if errno != 0 {
		return Topology{}, vm.err(op, errno)
	}
	return Topology{
		Sockets: sockets,
		Cores:   cores,
		Threads: threads,
		MaxCPUs: maxcpus,
	}, nil
}

// SetTopology sets how the VM's vCPUs are presented to the guest.
//
// It cannot change how many there are: t.MaxCPUs is ignored, because the
// kernel ignores it. Sockets*Cores*Threads must fit within the VM's
// existing vCPU count, which Topology reports.
//
// That bound is checked here rather than left to the kernel, which
// answers a request for more vCPUs than the VM has with a bare EINVAL.
// Knowing the limit is the useful half of that answer.
func (vm *VM) SetTopology(t Topology) error {
	const op = "vm_set_topology"

	if err := t.validate(); err != nil {
		return err
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	_, _, _, maxcpus, errno := ctx.Topology()
	if errno != 0 {
		return vm.err("vm_get_topology", errno)
	}
	if want := t.VCPUs(); want > int(maxcpus) {
		return fmt.Errorf("vmm: topology needs %d vcpus but the VM has %d: %w",
			want, maxcpus, ErrInvalidArgument)
	}

	return vm.err(op, ctx.SetTopology(t.Sockets, t.Cores, t.Threads,
		t.MaxCPUs))
}
