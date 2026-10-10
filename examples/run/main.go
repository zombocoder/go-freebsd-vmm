//go:build freebsd && amd64

// Command run loads a tiny real-mode payload into a guest and executes
// it, printing each VM exit.
//
//	run            # creates a VM, runs the payload, destroys it
//
// The payload writes a byte to a port and halts, so the output shows
// the two exits that matter plus however many spurious ones the Go
// runtime caused:
//
//	inout at rip=0x1002: out 0x80 size=1 value=0x42
//	hlt at rip=0x1004
//
// Needs root and vmm(4).
package main

import (
	"fmt"
	"log"
	"os"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

// Where the payload is loaded. Real mode with paging off makes the
// linear address the physical one.
const payloadAddr = 0x1000

// payload is real-mode code:
//
//	b0 42     mov al, 0x42
//	e6 80     out 0x80, al
//	f4        hlt
var payload = []byte{0xb0, 0x42, 0xe6, 0x80, 0xf4}

// maxExits stops the loop if the guest never halts.
const maxExits = 1000

func main() {
	log.SetFlags(0)
	log.SetPrefix("run: ")

	name := "govmm-run-example"
	if len(os.Args) == 2 {
		name = os.Args[1]
	}

	vm, err := vmm.OpenWithFlags(name, vmm.OpenCreate|vmm.OpenDestroyOnClose)
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	if err := boot(vm); err != nil {
		log.Fatal(err)
	}
}

// boot sets the VM up and runs the payload to completion.
func boot(vm *vmm.VM) error {
	if err := vm.SetTopology(vmm.Topology{
		Sockets: 1, Cores: 1, Threads: 1, MaxCPUs: 1,
	}); err != nil {
		return err
	}
	if err := vm.SetupMemory(64<<20, vmm.MemoryMapAll); err != nil {
		return err
	}

	region, err := vm.MapGuestMemory(payloadAddr, uint64(len(payload)))
	if err != nil {
		return err
	}
	copy(region.Bytes(), payload)

	cpu, err := vm.VCPU(0)
	if err != nil {
		return err
	}
	defer cpu.Close()

	if err := cpu.Reset(); err != nil {
		return err
	}

	// Real-mode execution needs unrestricted guest mode on VT-x, and
	// HLT has to come back to us rather than idling in the guest.
	if err := cpu.SetCapability(vmm.CapUnrestrictedGuest, 1); err != nil {
		return fmt.Errorf("this host cannot run unrestricted guests: %w", err)
	}
	if err := cpu.SetCapability(vmm.CapHaltExit, 1); err != nil {
		return err
	}

	// Reset leaves CS based at the reset vector; rebase it at zero and
	// start at the payload.
	if err := cpu.SetSegmentDescriptor(vmm.RegCS, vmm.SegmentDescriptor{
		Base:   0,
		Limit:  0xffff,
		Access: 0x0093,
	}); err != nil {
		return err
	}
	if err := cpu.SetRegisters([]vmm.RegisterValue{
		{Register: vmm.RegCS, Value: 0},
		{Register: vmm.RegRIP, Value: payloadAddr},
	}); err != nil {
		return err
	}

	if err := cpu.Activate(); err != nil {
		return err
	}

	spurious := 0
	exits := 0

	err = cpu.RunLoop(func(exit vmm.Exit) error {
		exits++
		if exits > maxExits {
			return fmt.Errorf("the guest did not halt after %d exits", maxExits)
		}

		// The kernel leaves the guest whenever the host thread has
		// a signal pending, which under the Go runtime is often.
		if exit.Code.Spurious() {
			spurious++
			return nil
		}

		fmt.Println(exit)

		if exit.Code == vmm.ExitHLT {
			return vmm.ErrStopRunLoop
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n%d exits, %d of them spurious\n", exits, spurious)
	return nil
}
