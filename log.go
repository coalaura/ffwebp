package main

import (
	"fmt"
	"os"

	"github.com/coalaura/arguments"
)

var (
	silent bool
)

func debug(msg string) {
	if silent {
		return
	}

	b := arguments.NewBuilder(true)

	b.Mute()
	b.WriteString("# ")
	b.WriteString(msg)

	println(b.String())
}

func fatalf(format string, args ...interface{}) {
	must(fmt.Errorf(format, args...))
}

func must(err error) {
	if err == nil {
		return
	}

	print("\033[38;5;160mERROR: \033[38;5;248m")
	println(err.Error())

	os.Exit(1)
}
