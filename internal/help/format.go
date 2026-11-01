package help

import (
	"io"
	"strings"

	"github.com/urfave/cli/v3"
)

const optionIndent = "       "

func PrintHelp(writer io.Writer, template string, data any) {
	var output strings.Builder

	cli.DefaultPrintHelp(&output, template, data)
	_ = writeHelp(writer, output.String())
}

func writeHelp(writer io.Writer, text string) error {
	var (
		output         strings.Builder
		previousOption bool
	)

	output.Grow(len(text))

	for line := range strings.Lines(text) {
		name, description, ok := optionColumns(line)
		if !ok {
			previousOption = false
			output.WriteString(line)

			continue
		}

		if previousOption {
			output.WriteByte('\n')
		}

		output.WriteString("   ")
		output.WriteString(name)
		output.WriteByte('\n')
		output.WriteString(optionIndent)
		output.WriteString(description)
		output.WriteByte('\n')
		previousOption = true
	}

	_, err := io.WriteString(writer, output.String())
	return err
}

func optionColumns(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "--") {
		return "", "", false
	}

	name, description, ok := strings.Cut(line, "  ")
	return name, strings.TrimSpace(description), ok
}
