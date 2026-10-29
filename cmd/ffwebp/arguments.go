package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/urfave/cli/v3"
)

func prepareArguments(arguments []string, command *cli.Command) ([]string, error) {
	// Completion intentionally supplies incomplete flags and values.
	if len(arguments) == 0 || arguments[len(arguments)-1] == "--generate-shell-completion" {
		return arguments, nil
	}

	prepared := make([]string, 0, len(arguments))
	prepared = append(prepared, arguments[0])

	for index := 1; index < len(arguments); index++ {
		argument := strings.TrimSpace(arguments[index])
		if argument == "--" || command.Command(argument) != nil {
			prepared = append(prepared, arguments[index:]...)

			break
		}

		if argument == "--join" || argument == "-join" {
			value := "autox1"

			if index+1 < len(arguments) && isJoinGridArgument(arguments[index+1]) {
				index++
				value = arguments[index]
			}

			prepared = append(prepared, "--join="+value)

			continue
		}

		prepared = append(prepared, arguments[index])

		if !isOptionArgument(argument) {
			continue
		}

		flag := argumentFlag(argument, command)
		if flag == nil {
			// Let the CLI report unknown flags and invalid short-option groups first.
			prepared = append(prepared, arguments[index+1:]...)

			break
		}

		if !flag.TakesValue() || strings.Contains(argument, "=") {
			continue
		}

		if index+1 == len(arguments) {
			return nil, fmt.Errorf("missing value for %s", argument)
		}

		next := strings.TrimSpace(arguments[index+1])
		if isOptionArgument(next) {
			return nil, fmt.Errorf("missing value for %s before %q; use %s=VALUE for a literal value starting with '-'", argument, next, argument)
		}

		index++
		prepared = append(prepared, arguments[index])
	}

	return prepared, nil
}

func argumentFlag(argument string, command *cli.Command) cli.DocGenerationFlag {
	name, _, _ := strings.Cut(strings.TrimLeft(argument, "-"), "=")

	flag := lookupArgumentFlag(name, command)
	if flag != nil || strings.HasPrefix(argument, "--") || !command.UseShortOptionHandling {
		return flag
	}

	// Match the CLI's short groups: only the final flag may take a value.
	for index, letter := range name {
		flag = lookupArgumentFlag(string(letter), command)
		if flag == nil || (flag.TakesValue() && index+utf8.RuneLen(letter) < len(name)) {
			return nil
		}
	}

	return flag
}

func lookupArgumentFlag(name string, command *cli.Command) cli.DocGenerationFlag {
	for _, flag := range command.Flags {
		if slices.Contains(flag.Names(), name) {
			documentation, _ := flag.(cli.DocGenerationFlag)

			return documentation
		}
	}

	switch name {
	case "help", "h":
		documentation, _ := cli.HelpFlag.(cli.DocGenerationFlag)

		return documentation
	case "version", "v":
		documentation, _ := cli.VersionFlag.(cli.DocGenerationFlag)

		return documentation
	}

	return nil
}

func isOptionArgument(value string) bool {
	if len(value) < 2 || value[0] != '-' {
		return false
	}

	letter, _ := utf8.DecodeRuneInString(value[1:])

	return letter == '-' || unicode.IsLetter(letter)
}

func isJoinGridArgument(value string) bool {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "auto") {
		return true
	}

	// Leave output filenames after bare --join positional, including 2x2.png.
	if strings.ContainsAny(value, ".\\/") {
		return false
	}

	separator := strings.IndexAny(value, "xX:,")
	if separator < 0 {
		return false
	}

	first := strings.TrimSpace(value[:separator])
	if strings.EqualFold(first, "auto") {
		return true
	}

	first = strings.TrimLeft(first, "+-")

	return len(first) > 0 && first[0] >= '0' && first[0] <= '9'
}
