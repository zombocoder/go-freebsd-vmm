//go:build freebsd && amd64

// Command create shows the VM lifecycle: create, open, inspect, destroy.
//
// Creating a VM needs privilege, so run it as root:
//
//	create vm01
package main

import (
	"fmt"
	"log"
	"os"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("create: ")

	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <vm-name>", os.Args[0])
	}
	name := os.Args[1]

	vm, err := vmm.CreateAndOpen(name)
	if err != nil {
		log.Fatal(err)
	}

	// Destroy rather than Close, so the example leaves nothing
	// behind. Deferring Close alongside it would also be safe: the
	// second call does nothing.
	defer func() {
		if err := vm.Destroy(); err != nil {
			log.Print(err)
		}
	}()

	fmt.Printf("created %s\n", vm.Name())

	topology, err := vm.Topology()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("topology: %v\n", topology)

	fmt.Printf("destroying %s\n", vm.Name())
}
