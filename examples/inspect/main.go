//go:build freebsd && amd64

// Command inspect reports what the library knows about the host and
// about one running VM.
//
//	inspect           # host and library only
//	inspect vm01      # and the VM named vm01
package main

import (
	"fmt"
	"log"
	"os"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("inspect: ")

	fmt.Printf("libvmmapi version: %v\n", vmm.APIVersion())
	fmt.Printf("max VM name length: %d\n", vmm.MaxNameLen)
	fmt.Printf("features:\n")
	for _, f := range vmm.Features() {
		fmt.Printf("  %s\n", f)
	}

	if len(os.Args) < 2 {
		return
	}

	vm, err := vmm.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	fmt.Printf("\nvm: %s\n", vm.Name())

	topology, err := vm.Topology()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  topology: %v\n", topology)
	fmt.Printf("  low memory:  %d bytes\n", vm.LowMemorySize())
	fmt.Printf("  high memory: %d bytes at %v\n",
		vm.HighMemorySize(), vm.HighMemoryBase())
	fmt.Printf("  memory flags: %v\n", vm.MemoryFlags())

	for _, set := range []struct {
		name string
		fn   func() (vmm.CPUSet, error)
	}{
		{"active", vm.ActiveCPUs},
		{"suspended", vm.SuspendedCPUs},
		{"debug", vm.DebugCPUs},
	} {
		cpus, err := set.fn()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  %-9s cpus: %v\n", set.name, cpus)
	}
}
