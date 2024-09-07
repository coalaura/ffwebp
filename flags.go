package main

import (
	"os"
	"strconv"
	"strings"
)

type Argument struct {
	IsNil bool
	Name  string
	Value string
}

type Arguments struct {
	Arguments map[string]Argument
}

var (
	arguments Arguments
)

// I don't like golang flags package
func init() {
	arguments = Arguments{
		Arguments: make(map[string]Argument),
	}

	var (
		arg   string
		val   string
		index int

		current Argument
	)

	for i := 1; i < len(os.Args); i++ {
		arg = os.Args[i]

		if arg[0] == '-' && len(arg) > 1 {
			if arg[1] == '-' {
				index = strings.Index(arg[2:], "=")

				if index >= 0 {
					val = ""

					if index+1 < len(arg) {
						val = arg[2+index+1:]
					}

					arguments.Set(Argument{
						Name:  arg[2 : 2+index],
						Value: val,
					})
				} else {
					arguments.Set(Argument{
						Name: arg[2:],
					})
				}

				current = Argument{}
			} else {
				current = Argument{
					Name: arg[1:],
				}
			}
		} else {
			current.Value = arg

			arguments.Set(current)

			current = Argument{}
		}
	}

	if current.Name != "" {
		arguments.Set(current)
	}
}

func (a *Arguments) Set(arg Argument) {
	a.Arguments[arg.Name] = arg
}

func (a *Arguments) Get(short, long string) Argument {
	arg, ok := a.Arguments[short]

	if !ok && long != short {
		arg, ok = a.Arguments[long]
	}

	if !ok {
		return Argument{
			IsNil: true,
			Name:  long,
		}
	}

	return arg
}

func (a *Arguments) GetString(short, long string) string {
	return a.Get(short, long).String()
}

func (a *Arguments) GetBool(short, long string, def bool) bool {
	return a.Get(short, long).Bool(def)
}

func (a *Arguments) GetInt64(short, long string, def, min, max int64) int64 {
	return a.Get(short, long).Int64(def, min, max)
}

func (a *Arguments) GetUint64(short, long string, def, min, max uint64) uint64 {
	return a.Get(short, long).Uint64(def, min, max)
}

func (a *Arguments) GetFloat64(short, long string, def, min, max float64) float64 {
	return a.Get(short, long).Float64(def, min, max)
}

func (a Argument) String() string {
	return a.Value
}

func (a Argument) Bool(def bool) bool {
	if a.IsNil {
		return def
	}

	if a.Value == "false" || a.Value == "0" {
		return false
	}

	return true
}

func (a Argument) Int64(def, min, max int64) int64 {
	if a.IsNil {
		return def
	}

	i, err := strconv.ParseInt(a.Value, 10, 64)
	if err != nil {
		return def
	}

	return minmax(i, min, max)
}

func (a Argument) Uint64(def, min, max uint64) uint64 {
	if a.IsNil {
		return def
	}

	i, err := strconv.ParseUint(a.Value, 10, 64)
	if err != nil {
		return def
	}

	return minmax(i, min, max)
}

func (a Argument) Float64(def, min, max float64) float64 {
	if a.IsNil {
		return def
	}

	i, err := strconv.ParseFloat(a.Value, 64)
	if err != nil {
		return def
	}

	return minmax(i, min, max)
}

func minmax[T int64 | uint64 | float64](val, min, max T) T {
	if min != 0 && val < min {
		return min
	}

	if max != 0 && val > max {
		return max
	}

	return val
}
