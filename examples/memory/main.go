//go:build freebsd && amd64

// Command memory allocates guest memory for a VM and shows the
// resulting guest address space.
//
//	memory vm01 256     # 256 MiB
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("memory: ")

	if len(os.Args) != 3 {
		log.Fatalf("usage: %s <vm-name> <megabytes>", os.Args[0])
	}
	mib, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		log.Fatalf("megabytes: %v", err)
	}

	vm, err := vmm.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	// Wiring guest memory down keeps it from being paged out, which
	// is what a VM that is about to run wants.
	if err := vm.SetMemoryFlags(vmm.MemoryWired); err != nil {
		log.Fatal(err)
	}

	if err := vm.SetupMemory(mib<<20, vmm.MemoryMapAll); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("low memory:  %d bytes (limit %#x)\n",
		vm.LowMemorySize(), vm.LowMemoryLimit())
	fmt.Printf("high memory: %d bytes at %v\n",
		vm.HighMemorySize(), vm.HighMemoryBase())

	fmt.Println("\nguest address space:")
	mappings, err := vm.MemoryMappings()
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range mappings {
		fmt.Printf("  %v\n", m)
	}

	// The host mapping aliases guest memory directly: this write is
	// visible to the guest at once.
	region, err := vm.MapGuestMemory(0, 4096)
	if err != nil {
		log.Fatal(err)
	}
	copy(region.Bytes(), []byte("written by the host"))

	fmt.Printf("\nfirst page of guest memory: %q\n", region.Bytes()[:19])
}
