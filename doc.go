//go:build freebsd && amd64

// Package vmm provides Go bindings to FreeBSD's virtual machine monitor.
//
// # What this binds
//
// vmm(4) is the FreeBSD kernel module that drives the hardware
// virtualization extensions of the processor: VT-x on Intel, AMD-V on
// AMD. It presents each virtual machine as a device under /dev/vmm and
// is controlled through ioctls on it. libvmmapi is the C library in the
// base system that wraps those ioctls, and this package wraps
// libvmmapi:
//
//	application
//	    |
//	    v
//	go-freebsd-vmm
//	    |
//	    v
//	libvmmapi
//	    |
//	    v
//	vmm(4)
//	    |
//	    v
//	VT-x / AMD-V
//
// Nothing here runs bhyve, bhyvectl or bhyveload, and nothing here
// shells out at all. Every operation is a library call.
//
// # Scope
//
// This package exposes VMM primitives: virtual machines, virtual CPUs,
// guest memory, registers, CPU topology, capabilities, statistics, guest
// execution and VM exits, interrupt injection, PCI passthrough, address
// translation and the low-level snapshot calls.
//
// It deliberately has no opinion about how a virtual machine should be
// configured. Device models, virtio, tap interfaces, disk images, UEFI
// firmware and command line construction are all absent, because they
// are bhyve's concerns rather than the kernel's. Software that needs
// those belongs on top of this package, not inside it.
//
// One boundary is worth naming because it looks like an omission.
// ExitInstEmul hands over the instruction bytes of an access the kernel
// wants emulated, which is how MMIO to a device modelled in userspace
// arrives, but nothing here decodes them: FreeBSD's instruction emulator
// is a kernel source file that bhyve compiles into itself, libvmmapi does
// not export it, and an x86 decoder is not a binding. VCPU.KernelEmulatedDevice
// covers the devices the kernel already models.
//
// # Executing a guest
//
// VCPU.Run enters the guest and returns the exit that brought the vCPU
// back, decoded into typed values. VCPU.RunLoop does that repeatedly on a
// pinned host thread, which is the shape a vCPU thread usually takes.
//
// Two things about exits surprise people coming from bhyve. The kernel
// leaves the guest whenever the calling thread has a signal pending, and
// the Go runtime signals its threads routinely, so ExitBogus arrives far
// more often here than in a C program; ExitCode.Spurious recognises the
// exits that mean nothing happened. And Run cannot be cancelled, which is
// why it takes no context.Context: a vCPU comes back when the guest
// exits, when the VM or the vCPU is suspended from elsewhere, or on a
// signal.
//
// # Privileges
//
// Creating, opening and controlling a VM requires privilege. An
// unprivileged process gets EPERM from Create and ENOENT or EACCES from
// Open. The host must also support hardware virtualization, and vmm(4)
// must be loadable; Create loads it if it is not already loaded.
//
// # Resource ownership
//
// Ownership is explicit and there are no finalizers. A VM owns one
// libvmmapi context, and the vCPU handles opened from it belong to that
// VM:
//
//	vm
//	|
//	+-- vcpu 0
//	+-- vcpu 1
//	+-- vcpu 2
//	+-- vcpu 3
//
// Close releases the context and keeps the VM in the kernel; Destroy
// releases the context and removes the VM. Both also release every vCPU
// handle, because libvmmapi frees the context those handles point into.
// Whichever of the two runs first wins and the other does nothing, so a
// deferred Close alongside an explicit Destroy is correct.
//
// Memory regions returned by MapGuestMemory alias guest memory and are
// owned by C. They stay valid only while their VM is open.
//
// # Error handling
//
// Failures reported by the kernel arrive as *Error, which keeps the
// errno so that callers can act on it:
//
//	if errors.Is(err, syscall.EBUSY) {
//		// the VM is running; stop it first
//	}
//
// Misuse of this API, as opposed to a kernel failure, wraps one of
// ErrClosed, ErrInvalidArgument or ErrUnsupported. Nothing panics
// because of a kernel error, an invalid VM state or a missing device;
// those are returned.
//
// # Threading and concurrency
//
// VM and VCPU methods are safe to call concurrently. Operations that
// only need the VM to be open take a shared lock, so work on
// independent vCPUs does not serialise; opening and closing handles take
// it exclusively. A single VCPU must not be driven from several
// goroutines at once.
//
// Statistics are the exception: libvmmapi keeps them in static storage,
// so VCPU.Stats is serialised across the whole process.
//
// # Portability
//
// FreeBSD on amd64. Other platforms fail to build with an explanation
// rather than offering an API that cannot work. Architecture differences
// are compile-time differences here: a register or an interrupt
// controller that does not exist on the target architecture is a
// compile error, not a runtime one.
//
// Across FreeBSD releases, feature availability is a question for
// Supports rather than for version arithmetic:
//
//	if vmm.Supports(vmm.FeatureSparseMemory) {
//		// ...
//	}
//
// APIVersion reports the libvmmapi version the package was built
// against: 2.0 on FreeBSD 14, 3.0 on FreeBSD 15.
//
// # Example
//
//	vm, err := vmm.CreateAndOpen("vm01")
//	if err != nil {
//		return err
//	}
//	defer vm.Destroy()
//
//	if err := vm.SetTopology(vmm.Topology{
//		Sockets: 1, Cores: 2, Threads: 1, MaxCPUs: 2,
//	}); err != nil {
//		return err
//	}
//	if err := vm.SetupMemory(256<<20, vmm.MemoryMapAll); err != nil {
//		return err
//	}
//
//	cpu, err := vm.VCPU(0)
//	if err != nil {
//		return err
//	}
//	defer cpu.Close()
//
//	if err := cpu.Activate(); err != nil {
//		return err
//	}
//
//	rip, err := cpu.Register(vmm.RegRIP)
//	if err != nil {
//		return err
//	}
//	fmt.Printf("RIP=%#x\n", rip)
package vmm
