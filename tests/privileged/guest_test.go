//go:build freebsd && amd64 && vmm_privileged

// Package privileged_test drives the parts of the library that enter the
// guest or touch host hardware.
//
// These tests are excluded from the default build and run only with:
//
//	go test -tags vmm_privileged ./...
//
// They need root, vmm(4) and working hardware virtualization, and they
// execute real guest code, which is why they are not part of an ordinary
// test run.
package privileged_test

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	vmm "github.com/zombocoder/go-freebsd-vmm"
	"github.com/zombocoder/go-freebsd-vmm/tests/harness"
)

// guestMemory is enough for the test payload and the kernel's own
// bookkeeping.
const guestMemory = 64 << 20

// payloadAddr is where a test guest is loaded. Real mode with paging off
// makes the linear address the physical one, so this is both.
const payloadAddr = 0x1000

// probePort is the I/O port the guest writes to, and probeValue what it
// writes. Port 0x80 is the POST diagnostic port, which no emulated
// device claims, so an access to it can only have come from the payload.
const (
	probePort  = 0x80
	probeValue = 0x42
)

// haltPayload is a real-mode program that announces itself on a port and
// halts:
//
//	b0 42     mov al, 0x42
//	e6 80     out 0x80, al
//	f4        hlt
//
// It is deliberately this small. The point is to exercise entry, exit
// decoding and resumption, not to emulate a machine, so the payload has
// no dependence on any device model, firmware or memory layout beyond
// the bytes themselves.
var haltPayload = []byte{0xb0, probeValue, 0xe6, probePort, 0xf4}

// Offsets of the interesting instructions within haltPayload.
const (
	outOffset = 2
	hltOffset = 4
)

// spinPayload is a real-mode program that never finishes:
//
//	eb fe     jmp $
//
// A guest that does not come back on its own is what the suspend tests
// need, since that is the only way to observe a blocked Run returning.
var spinPayload = []byte{0xeb, 0xfe}

// maxExits bounds a run loop so a misbehaving guest fails the test
// instead of hanging it. Spurious exits are included in the count, and
// there can be a few.
const maxExits = 1000

func TestMain(m *testing.M) {
	os.Exit(harness.Run(m))
}

// bootedVM prepares a VM with the payload loaded and one activated vCPU
// ready to execute it.
func bootedVM(t *testing.T, payload []byte) (*vmm.VM, *vmm.VCPU) {
	t.Helper()

	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	// Load the payload into guest memory.
	region, err := vm.MapGuestMemory(payloadAddr, uint64(len(payload)))
	if err != nil {
		t.Fatalf("mapping guest memory at %#x: %v", payloadAddr, err)
	}
	copy(region.Bytes(), payload)

	cpu := harness.RequireVCPU(t, vm, 0)

	if err := cpu.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// VT-x needs unrestricted guest mode to execute real-mode code.
	// Hardware without it cannot run these payloads at all.
	if err := cpu.SetCapability(vmm.CapUnrestrictedGuest, 1); err != nil {
		t.Skipf("this host cannot run unrestricted guests: %v", err)
	}

	// Make HLT come back to the VMM instead of idling in the guest.
	if err := cpu.SetCapability(vmm.CapHaltExit, 1); err != nil {
		t.Fatalf("enabling halt exit: %v", err)
	}

	// vcpu_reset leaves CS based at the reset vector near the top of
	// the address space. Rebase it at zero and point RIP at the
	// payload, so the guest starts where it was loaded.
	if err := cpu.SetSegmentDescriptor(vmm.RegCS, vmm.SegmentDescriptor{
		Base:   0,
		Limit:  0xffff,
		Access: 0x0093,
	}); err != nil {
		t.Fatalf("rebasing CS: %v", err)
	}
	if err := cpu.SetRegisters([]vmm.RegisterValue{
		{Register: vmm.RegCS, Value: 0},
		{Register: vmm.RegRIP, Value: payloadAddr},
	}); err != nil {
		t.Fatalf("setting CS and RIP: %v", err)
	}

	if err := cpu.Activate(); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	return vm, cpu
}

// TestGuestRunsToPortWriteAndHalt is the end-to-end test for execution:
// real guest code runs, the port write comes back as a decoded IOExit,
// and the guest resumes and stops.
//
// The halt does not arrive as ExitHLT, which is the surprise worth
// recording. vmm(4) treats HLT with interrupts disabled as a vCPU
// parking itself, and when every active vCPU has done so it halts the
// whole VM. A reset vCPU has interrupts disabled and this VM has one
// vCPU, so the payload's HLT suspends the VM and the exit is
// ExitSuspended carrying SuspendHalt.
func TestGuestRunsToPortWriteAndHalt(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	var sawIO, sawStop bool
	var spurious int

	for i := 0; i < maxExits && !sawStop; i++ {
		exit, err := cpu.Run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}

		if exit.Code.Spurious() {
			spurious++
			continue
		}

		switch exit.Code {
		case vmm.ExitInOut:
			if exit.IO == nil {
				t.Fatalf("%v carries no IO payload", exit)
			}
			if exit.IO.Port != probePort {
				t.Errorf("port = %#x, want %#x", exit.IO.Port, probePort)
			}
			if exit.IO.Direction != vmm.IOOut {
				t.Errorf("direction = %v, want out", exit.IO.Direction)
			}
			if exit.IO.Size != 1 {
				t.Errorf("size = %d, want 1", exit.IO.Size)
			}
			if exit.IO.Value&0xff != probeValue {
				t.Errorf("value = %#x, want %#x",
					exit.IO.Value&0xff, probeValue)
			}
			sawIO = true

		case vmm.ExitSuspended:
			if exit.Suspended == nil {
				t.Fatalf("%v carries no Suspended payload", exit)
			}
			if exit.Suspended.How != vmm.SuspendHalt {
				t.Errorf("suspend reason = %v, want halt",
					exit.Suspended.How)
			}
			// The HLT is one byte, so the guest stopped just
			// past it.
			if want := uint64(payloadAddr + hltOffset + 1); exit.RIP != want {
				t.Errorf("RIP = %#x, want %#x", exit.RIP, want)
			}
			sawStop = true

		case vmm.ExitHLT:
			// Reached on a host where halt detection is off, or
			// if the guest ever halts with interrupts enabled.
			sawStop = true

		default:
			t.Fatalf("unexpected exit: %v", exit)
		}
	}

	if !sawIO {
		t.Error("the guest never reached the port write")
	}
	if !sawStop {
		t.Error("the guest never stopped")
	}
	t.Logf("completed with %d spurious exits", spurious)
}

// TestRunLoopDrivesTheGuest covers the same payload through RunLoop,
// which pins the goroutine to a host thread.
func TestRunLoopDrivesTheGuest(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	var exits int
	var sawIO bool

	err := cpu.RunLoop(func(exit vmm.Exit) error {
		exits++
		if exits > maxExits {
			return errors.New("the guest did not stop")
		}
		if exit.Code.Spurious() {
			return nil
		}

		switch exit.Code {
		case vmm.ExitInOut:
			sawIO = true
			return nil
		case vmm.ExitSuspended, vmm.ExitHLT:
			return vmm.ErrStopRunLoop
		default:
			return errors.New("unexpected exit: " + exit.String())
		}
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if !sawIO {
		t.Error("the guest never reached the port write")
	}
}

// TestRIPAdvancesPastTheInstruction checks the kernel resumes after the
// faulting instruction without the caller adjusting RIP, which is what
// makes the loops above terminate.
func TestRIPAdvancesPastTheInstruction(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	for i := 0; i < maxExits; i++ {
		exit, err := cpu.Run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if exit.Code.Spurious() {
			continue
		}
		if exit.Code != vmm.ExitInOut {
			t.Fatalf("first real exit was %v, want inout", exit)
		}

		if want := uint64(payloadAddr + outOffset); exit.RIP != want {
			t.Errorf("exit RIP = %#x, want %#x", exit.RIP, want)
		}
		if exit.InstLength != 2 {
			t.Errorf("InstLength = %d, want 2", exit.InstLength)
		}

		rip, err := cpu.Register(vmm.RegRIP)
		if err != nil {
			t.Fatalf("Register(RegRIP): %v", err)
		}
		if rip != exit.RIP {
			t.Errorf("RIP register = %#x, exit reported %#x", rip, exit.RIP)
		}
		return
	}
	t.Fatal("no I/O exit within the exit budget")
}

// TestSuspendBringsTheGuestBack is the answer to there being no
// cancellation on Run: the guest spins forever, and another goroutine
// suspending the VM is what makes the blocked call return.
func TestSuspendBringsTheGuestBack(t *testing.T) {
	vm, cpu := bootedVM(t, spinPayload)

	exits := make(chan vmm.Exit, 1)
	errs := make(chan error, 1)

	go func() {
		for i := 0; i < maxExits; i++ {
			exit, err := cpu.Run()
			if err != nil {
				errs <- err
				return
			}
			if exit.Code.Spurious() {
				continue
			}
			exits <- exit
			return
		}
		errs <- errors.New("the spinning guest produced only spurious exits")
	}()

	// Give the guest a moment to actually be running, so the suspend
	// has something to interrupt.
	time.Sleep(50 * time.Millisecond)

	if err := vm.Suspend(vmm.SuspendPowerOff); err != nil {
		t.Fatalf("Suspend: %v", err)
	}

	select {
	case exit := <-exits:
		if exit.Code != vmm.ExitSuspended {
			t.Fatalf("exit after Suspend was %v, want suspended", exit)
		}
		if exit.Suspended == nil {
			t.Fatalf("%v carries no Suspended payload", exit)
		}
		if exit.Suspended.How != vmm.SuspendPowerOff {
			t.Errorf("suspend reason = %v, want poweroff",
				exit.Suspended.How)
		}
	case err := <-errs:
		t.Fatalf("Run: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10s of the VM being suspended")
	}
}

// TestRunAfterSuspendIsRefused records that a suspended vCPU cannot be
// entered again: vm_run(2) checks the vCPU against the VM's suspended
// set and reports EINVAL. Resuming means Reinitialize, not another Run.
func TestRunAfterSuspendIsRefused(t *testing.T) {
	vm, cpu := bootedVM(t, haltPayload)

	// Run until the guest halts, which suspends the VM.
	var stopped bool
	for i := 0; i < maxExits && !stopped; i++ {
		exit, err := cpu.Run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		switch exit.Code {
		case vmm.ExitSuspended, vmm.ExitHLT:
			stopped = true
		}
	}
	if !stopped {
		t.Fatal("the guest never stopped")
	}

	suspended, err := vm.SuspendedCPUs()
	if err != nil {
		t.Fatalf("SuspendedCPUs: %v", err)
	}
	if !suspended.Contains(0) {
		t.Skipf("vcpu 0 is not suspended after the halt (%v); halt detection may be off",
			suspended)
	}

	_, err = cpu.Run()
	if err == nil {
		t.Fatal("Run on a suspended vcpu succeeded, want an error")
	}
	if !errors.Is(err, syscall.EINVAL) {
		t.Errorf("Run on a suspended vcpu = %v, want EINVAL", err)
	}
	t.Logf("refused with: %v", err)
}

// TestInjectInterruptsIntoAnIdleVCPU exercises the injection paths
// against a real vCPU that is not executing. Nothing observes the
// interrupts; what is tested is that the kernel accepts them and that
// the errno path is wired up.
func TestInjectInterruptsIntoAnIdleVCPU(t *testing.T) {
	vm, cpu := bootedVM(t, haltPayload)

	if err := cpu.InjectNMI(); err != nil {
		t.Errorf("InjectNMI: %v", err)
	}
	if err := cpu.LAPICIRQ(0xef); err != nil {
		t.Errorf("LAPICIRQ(0xef): %v", err)
	}
	if err := cpu.LAPICLocalIRQ(0xef); err != nil {
		t.Errorf("LAPICLocalIRQ(0xef): %v", err)
	}

	// #GP, vector 13, carries an error code.
	errcode := uint32(0)
	if err := cpu.InjectException(13, &errcode, false); err != nil {
		t.Errorf("InjectException(13): %v", err)
	}

	pincount, err := vm.IOAPICPinCount()
	if err != nil {
		t.Fatalf("IOAPICPinCount: %v", err)
	}
	if pincount <= 0 {
		t.Errorf("IOAPICPinCount = %d, want a positive count", pincount)
	}
	t.Logf("ioapic has %d pins", pincount)

	if err := vm.IOAPICPulse(2); err != nil {
		t.Errorf("IOAPICPulse(2): %v", err)
	}
	if err := vm.IOAPICAssert(2); err != nil {
		t.Errorf("IOAPICAssert(2): %v", err)
	}
	if err := vm.IOAPICDeassert(2); err != nil {
		t.Errorf("IOAPICDeassert(2): %v", err)
	}
	if err := vm.ISAPulse(4, 4); err != nil {
		t.Errorf("ISAPulse(4, 4): %v", err)
	}
	if err := vm.ISASetIRQTrigger(4, vmm.TriggerEdge); err != nil {
		t.Errorf("ISASetIRQTrigger: %v", err)
	}

	// An out-of-range pin has to come back as an errno, not silently
	// succeed.
	if err := vm.IOAPICAssert(pincount + 100); err == nil {
		t.Error("IOAPICAssert past the last pin succeeded, want an error")
	}
}

// TestLAPICMSI delivers a message signalled interrupt through the VM's
// local APICs.
func TestLAPICMSI(t *testing.T) {
	vm, _ := bootedVM(t, haltPayload)

	// A fixed-delivery message to APIC id 0, vector 0x30: the address
	// is the conventional 0xfee00000 window with the destination in
	// bits 19:12, and the message low byte is the vector.
	const addr = 0xfee00000
	const msg = 0x30

	if err := vm.LAPICMSI(addr, msg); err != nil {
		t.Errorf("LAPICMSI(%#x, %#x): %v", addr, msg, err)
	}
}

func TestIntInfoRoundTrip(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	pending, interrupted, err := cpu.IntInfo()
	if err != nil {
		t.Fatalf("IntInfo: %v", err)
	}
	t.Logf("pending=%#x interrupted=%#x", pending, interrupted)

	// Stage an external interrupt on vector 0x20.
	info := vmm.IntInfoValid | vmm.IntInfoHWInterrupt | 0x20
	if err := cpu.SetIntInfo(info); err != nil {
		t.Fatalf("SetIntInfo(%#x): %v", info, err)
	}

	pending, _, err = cpu.IntInfo()
	if err != nil {
		t.Fatalf("IntInfo after set: %v", err)
	}
	if pending&vmm.IntInfoValid == 0 {
		t.Errorf("pending = %#x, want the valid bit set", pending)
	}
	if got := vmm.IntInfoVector(pending); got != 0x20 {
		t.Errorf("pending vector = %#x, want 0x20", got)
	}

	// Clear it again so the guest is not left with a staged event.
	if err := cpu.SetIntInfo(0); err != nil {
		t.Errorf("SetIntInfo(0): %v", err)
	}
}

func TestX2APICStateRoundTrip(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	before, err := cpu.X2APICState()
	if err != nil {
		t.Fatalf("X2APICState: %v", err)
	}
	t.Logf("x2apic state is %v", before)

	for _, want := range []vmm.X2APICState{vmm.X2APICEnabled, vmm.X2APICDisabled} {
		if err := cpu.SetX2APICState(want); err != nil {
			t.Fatalf("SetX2APICState(%v): %v", want, err)
		}
		got, err := cpu.X2APICState()
		if err != nil {
			t.Fatalf("X2APICState: %v", err)
		}
		if got != want {
			t.Errorf("X2APICState() = %v, want %v", got, want)
		}
	}

	if err := cpu.SetX2APICState(before); err != nil {
		t.Errorf("restoring x2apic state: %v", err)
	}
}

func TestRTCRoundTrip(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	// The NVRAM area past the time and status registers is free for
	// anything, so writing there cannot disturb timekeeping.
	const offset = 0x40
	const value = 0xa5

	if err := vm.RTCWrite(offset, value); err != nil {
		t.Fatalf("RTCWrite(%#x, %#x): %v", offset, value, err)
	}
	got, err := vm.RTCRead(offset)
	if err != nil {
		t.Fatalf("RTCRead(%#x): %v", offset, err)
	}
	if got != value {
		t.Errorf("RTCRead(%#x) = %#x, want %#x", offset, got, value)
	}

	want := time.Date(2026, time.October, 10, 12, 0, 0, 0, time.UTC)
	if err := vm.SetRTCTime(want); err != nil {
		t.Fatalf("SetRTCTime: %v", err)
	}
	rtc, err := vm.RTCTime()
	if err != nil {
		t.Fatalf("RTCTime: %v", err)
	}
	if rtc.Unix() != want.Unix() {
		t.Errorf("RTCTime() = %v, want %v", rtc, want)
	}
}

func TestHPETCapabilities(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	caps, err := vm.HPETCapabilities()
	if err != nil {
		t.Fatalf("HPETCapabilities: %v", err)
	}
	if caps == 0 {
		t.Error("HPETCapabilities = 0, want a capability word")
	}
	t.Logf("hpet capabilities: %#x", caps)
}

func TestAPICIDToVCPU(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	id, err := vm.APICIDToVCPU(0)
	if err != nil {
		t.Fatalf("APICIDToVCPU(0): %v", err)
	}
	if id != 0 {
		t.Errorf("APICIDToVCPU(0) = %d, want 0", id)
	}
}

func TestGuestPageMap(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	// Touch the page first so the host has backed it.
	region, err := vm.MapGuestMemory(payloadAddr, 4096)
	if err != nil {
		t.Fatalf("MapGuestMemory: %v", err)
	}
	region.Bytes()[0] = 1

	pte, err := vm.GuestPageMap(payloadAddr)
	if err != nil {
		t.Fatalf("GuestPageMap(%#x): %v", payloadAddr, err)
	}
	if len(pte) == 0 {
		t.Error("GuestPageMap returned no entries")
	}
	for i, entry := range pte {
		t.Logf("level %d: %#x", i, entry)
	}
}

// TestGuestVirtualCopyInRealMode reads the payload back through the
// guest's own address space, which in real mode with paging off is the
// identity.
func TestGuestVirtualCopyInRealMode(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	paging := vmm.PagingState{
		CR3:        0,
		CPL:        0,
		CPUMode:    vmm.CPUModeReal,
		PagingMode: vmm.PagingModeFlat,
	}

	got := make([]byte, len(haltPayload))
	if err := cpu.ReadGuestVirtual(payloadAddr, got, paging); err != nil {
		t.Fatalf("ReadGuestVirtual(%#x): %v", payloadAddr, err)
	}
	for i := range haltPayload {
		if got[i] != haltPayload[i] {
			t.Fatalf("read back %#x, want %#x", got, haltPayload)
		}
	}

	// Write a marker just past the payload and read it back.
	marker := []byte{0xde, 0xad}
	addr := uint64(payloadAddr + len(haltPayload))
	if err := cpu.WriteGuestVirtual(addr, marker, paging); err != nil {
		t.Fatalf("WriteGuestVirtual(%#x): %v", addr, err)
	}

	back := make([]byte, len(marker))
	if err := cpu.ReadGuestVirtual(addr, back, paging); err != nil {
		t.Fatalf("ReadGuestVirtual(%#x): %v", addr, err)
	}
	for i := range marker {
		if back[i] != marker[i] {
			t.Errorf("read back %#x, want %#x", back, marker)
			break
		}
	}
}

// TestGuestVirtualCopySpansPages checks the chunking in the bridge, which
// exists because vm_copy_setup(3) asserts rather than fails when its
// iovec array is too short for the range.
func TestGuestVirtualCopySpansPages(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	paging := vmm.PagingState{
		CPUMode:    vmm.CPUModeReal,
		PagingMode: vmm.PagingModeFlat,
	}

	// Start part way into a page and run well past the chunk size, so
	// both the partial first page and the chunk boundary are covered.
	const addr = 0x200000 + 0x800
	const size = 300 << 10

	want := make([]byte, size)
	for i := range want {
		want[i] = byte(i)
	}

	if err := cpu.WriteGuestVirtual(addr, want, paging); err != nil {
		t.Fatalf("WriteGuestVirtual(%#x, %d bytes): %v", addr, size, err)
	}

	got := make([]byte, size)
	if err := cpu.ReadGuestVirtual(addr, got, paging); err != nil {
		t.Fatalf("ReadGuestVirtual(%#x, %d bytes): %v", addr, size, err)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d of %d read back as %#x, want %#x",
				i, size, got[i], want[i])
		}
	}
}

func TestGuestLinearToPhysicalInRealMode(t *testing.T) {
	_, cpu := bootedVM(t, haltPayload)

	paging := vmm.PagingState{
		CPUMode:    vmm.CPUModeReal,
		PagingMode: vmm.PagingModeFlat,
	}

	gpa, err := cpu.GuestLinearToPhysical(paging, payloadAddr, vmm.ProtRead)
	if err != nil {
		t.Fatalf("GuestLinearToPhysical(%#x): %v", payloadAddr, err)
	}
	// Paging is off, so the translation is the identity.
	if gpa != payloadAddr {
		t.Errorf("GuestLinearToPhysical(%#x) = %v, want %#x",
			payloadAddr, gpa, payloadAddr)
	}

	nofault, err := cpu.GuestLinearToPhysicalNoFault(paging, payloadAddr,
		vmm.ProtRead)
	if err != nil {
		t.Fatalf("GuestLinearToPhysicalNoFault: %v", err)
	}
	if nofault != gpa {
		t.Errorf("the two translation variants disagree: %v and %v",
			nofault, gpa)
	}
}

// TestSnapshotSaveProducesBytes covers the low-level snapshot primitive.
// It checks only that the kernel writes something: the format is
// internal and this library makes no promise about it.
//
// Snapshot support is a kernel build option. A kernel without it has no
// ioctl to call and reports ENOTTY, which is a skip rather than a
// failure.
func TestSnapshotSaveProducesBytes(t *testing.T) {
	vm, _ := bootedVM(t, haltPayload)

	for _, structure := range []vmm.SnapshotStructure{
		vmm.SnapshotVM,
		vmm.SnapshotLAPIC,
		vmm.SnapshotIOAPIC,
		vmm.SnapshotRTC,
	} {
		t.Run(structure.String(), func(t *testing.T) {
			meta := &vmm.SnapshotMetadata{
				Structure: structure,
				Buffer:    make([]byte, 1<<20),
			}

			used, err := vm.SnapshotSave(meta)
			if err != nil {
				if errors.Is(err, syscall.ENOTTY) {
					t.Skip("this kernel was built without snapshot support")
				}
				t.Fatalf("SnapshotSave(%v): %v", structure, err)
			}
			if used == 0 {
				t.Errorf("SnapshotSave(%v) wrote 0 bytes", structure)
			}
			t.Logf("%v: %d bytes", structure, used)
		})
	}
}

func TestRestoreTime(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	if err := vm.RestoreTime(); err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			t.Skip("this kernel was built without snapshot support")
		}
		t.Errorf("RestoreTime: %v", err)
	}
}

// TestPassthroughRequiresADevice documents what the passthrough calls do
// without one: the address validation and the error path are exercised,
// the assignment is not.
func TestPassthroughRequiresADevice(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	// An address no ppt(4) device is attached at. The kernel refuses
	// it, which is the behaviour being checked: the call reaches the
	// kernel and reports why rather than succeeding quietly.
	dev := vmm.PCIDevice{Bus: 255, Slot: 31, Function: 7}

	err := vm.AssignPassthrough(dev)
	if err == nil {
		t.Errorf("AssignPassthrough(%v) succeeded on an address with no ppt device", dev)
		if err := vm.UnassignPassthrough(dev); err != nil {
			t.Errorf("UnassignPassthrough: %v", err)
		}
		return
	}

	var e *vmm.Error
	if !errors.As(err, &e) {
		t.Fatalf("AssignPassthrough = %v, want a *vmm.Error carrying errno", err)
	}
	if e.Op != "vm_assign_pptdev" {
		t.Errorf("Error.Op = %q, want %q", e.Op, "vm_assign_pptdev")
	}
	if e.VM != vm.Name() {
		t.Errorf("Error.VM = %q, want %q", e.VM, vm.Name())
	}
	t.Logf("assigning %v reported %v", dev, err)
}

// TestLimitRightsIsOneWay drops the Capsicum rights on a VM's
// descriptor. It runs last in spirit: the VM is unusable for most
// operations afterwards, so it gets one of its own.
func TestLimitRightsIsOneWay(t *testing.T) {
	vm := harness.RequireVMWithCPUs(t, 1, guestMemory)

	if err := vm.LimitRights(); err != nil {
		t.Fatalf("LimitRights: %v", err)
	}

	// The VMM ioctls still work, which is the point of limiting
	// rather than closing.
	if _, err := vm.Topology(); err != nil {
		t.Errorf("Topology() after LimitRights: %v", err)
	}
}
