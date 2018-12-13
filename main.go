package main

import (
	"fmt"

	"github.com/shirou/gopsutil/disk"
)

func main() {
	partitions, err := disk.Partitions(true)
	if err != nil {
		panic(err)
	}

	for _, p := range partitions {
		fmt.Printf("%+v\n", p)
		ioCounters, err := disk.IOCounters(p.Device)
		if err != nil {
			panic(err)
		}

		for k, v := range ioCounters {
			fmt.Printf("  %s: %+v\n", k, v)
		}

		fmt.Println("")
		fmt.Println("")
	}
}
