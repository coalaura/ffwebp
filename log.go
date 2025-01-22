package main

import (
	"fmt"
	"os"
)

var (
	silent bool
)

func info(fm string, args ...interface{}) {
	if silent {
		return
	}

	fmt.Printf(fm, args...)
	fmt.Println()
}

func fatalf(fm string, args ...interface{}) {
	fmt.Printf("ERROR: "+fm, args...)
	fmt.Println()

	os.Exit(1)
}
