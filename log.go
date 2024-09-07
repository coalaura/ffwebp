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

	fmt.Printf(fm+"\n", args...)
}

func fatalf(code int, fm string, args ...interface{}) {
	fmt.Printf("ERROR: "+fm+"\n", args...)

	os.Exit(code)
}
