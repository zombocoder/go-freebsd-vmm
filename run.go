//go:build freebsd && amd64

package vmm

import (
	"errors"
	"runtime"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// ErrStopRunLoop ends a RunLoop cleanly when returned by an ExitHandler.
// RunLoop itself then returns nil.
var ErrStopRunLoop = errors.New("vmm: stop run loop")

// ExitHandler services one VM exit.
//
// Returning nil enters the guest again. Returning ErrStopRunLoop ends the
// loop without an error; any other error ends it and is returned to the
// caller of RunLoop.
type ExitHandler func(Exit) error

// Run enters the guest and returns when the vCPU leaves it.
//
// The vCPU must have been activated first, or the kernel reports EINVAL.
// Guest memory must be set up and the vCPU's registers must describe
// somewhere to start; a freshly reset vCPU starts at the architectural
// power-on vector.
//
// # Blocking
//
// The call blocks in the kernel for as long as the guest runs, and there
// is no way to cancel it. That is why there is no variant taking a
// context.Context: an API that accepted one would have to either ignore
// cancellation or pretend to honour it. What does bring a vCPU back is
// VM.Suspend, VCPU.Suspend from another goroutine, or a signal arriving
// on the host thread.
//
// # After a suspend
//
// A suspended vCPU cannot be entered again: the kernel checks it against
// the VM's suspended set and reports EINVAL. The way back is
// VM.Reinitialize, not another Run. This arrives sooner than it sounds,
// because a guest that halts with interrupts disabled suspends the VM
// itself; see ExitHLT.
//
// # Spurious exits
//
// The kernel leaves the guest whenever the calling thread has work
// pending, including a pending signal, and reports ExitBogus. The Go
// runtime sends signals to its threads routinely, so these exits are
// common, far more so than under a C program such as bhyve. Use
// ExitCode.Spurious to recognise them; the response is to enter again.
//
// # Concurrency
//
// A single vCPU must not be run from two goroutines at once. Running
// different vCPUs of the same VM concurrently is the normal case and
// does not serialise.
func (cpu *VCPU) Run() (Exit, error) {
	const op = "vm_run"

	h, release, err := cpu.handle(op)
	if err != nil {
		return Exit{}, err
	}
	defer release()

	var raw vmmapi.RawExit
	words := make([]uint64, vmmapi.CPUSetWords())

	if errno := h.Run(&raw, words); errno != 0 {
		return Exit{}, cpu.err(op, errno)
	}

	// The destination mask is only meaningful for an IPI exit; for
	// everything else the kernel leaves it empty.
	var ipi CPUSet
	if ExitCode(raw.Code) == ExitIPI {
		ipi = CPUSet{words: words}
	}

	return decodeExit(&raw, ipi), nil
}

// RunLoop enters the guest repeatedly, calling handler for each exit.
//
// The loop ends when the handler returns an error, when Run fails, or
// when the handler returns ErrStopRunLoop, which is not an error.
//
// Every exit is passed to the handler, spurious ones included. Hiding
// them would be policy, and deciding what an exit means is the caller's
// job; the usual first line of a handler is therefore:
//
//	if exit.Code.Spurious() {
//		return nil
//	}
//
// # Thread affinity
//
// RunLoop pins its goroutine to one host thread for the duration, which
// Run does not do. This is not a kernel requirement, and it is worth
// being precise about why, because the obvious reasons are wrong: vmm(4)
// binds a vCPU to the calling CPU only for the length of one VM_RUN,
// clearing the binding on the way out, and it keeps no record of which
// thread entered the guest between calls. Guest FPU state is saved and
// restored inside each call rather than left on the thread.
//
// What the pinning buys is a dedicated host thread per vCPU, matching
// bhyve's model. A goroutine that migrated between iterations would have
// the kernel redo its per-thread entry work on a cold thread each time,
// and the blocking call would occupy whichever runtime thread it landed
// on. One locked thread per vCPU makes that explicit and bounded.
func (cpu *VCPU) RunLoop(handler ExitHandler) error {
	const op = "vm_run"

	if handler == nil {
		return errors.New("vmm: RunLoop needs a handler")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Reused across iterations: a run loop is the one place in this
	// library where per-iteration allocation is worth avoiding.
	var raw vmmapi.RawExit
	words := make([]uint64, vmmapi.CPUSetWords())

	for {
		h, release, err := cpu.handle(op)
		if err != nil {
			return err
		}
		errno := h.Run(&raw, words)
		release()

		if errno != 0 {
			return cpu.err(op, errno)
		}

		var ipi CPUSet
		if ExitCode(raw.Code) == ExitIPI {
			// The handler may keep the exit, so the mask has to
			// be its own copy rather than the reused buffer.
			ipi = CPUSet{words: append([]uint64(nil), words...)}
		}

		if err := handler(decodeExit(&raw, ipi)); err != nil {
			if errors.Is(err, ErrStopRunLoop) {
				return nil
			}
			return err
		}
	}
}
