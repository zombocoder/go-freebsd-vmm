//go:build freebsd && amd64

// Command stats prints the VMM statistics counters for a vCPU.
//
//	stats vm01 0
//
// Counters whose value is zero are left out unless -all is given.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("stats: ")

	all := flag.Bool("all", false, "include counters that are zero")
	flag.Parse()

	if flag.NArg() != 2 {
		log.Fatalf("usage: %s [-all] <vm-name> <vcpu>", os.Args[0])
	}
	id, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		log.Fatalf("vcpu: %v", err)
	}

	vm, err := vmm.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	cpu, err := vm.VCPU(id)
	if err != nil {
		log.Fatal(err)
	}
	defer cpu.Close()

	snapshot, err := cpu.StatsSnapshot()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s vcpu %d, sampled at %v\n\n",
		vm.Name(), cpu.ID(), snapshot.Time.Format("15:04:05.000"))

	shown := 0
	for _, s := range snapshot.Stats {
		if s.Value == 0 && !*all {
			continue
		}
		fmt.Printf("%-48s %12d\n", s.Description, s.Value)
		shown++
	}

	fmt.Printf("\n%d of %d counters shown\n", shown, len(snapshot.Stats))
}
