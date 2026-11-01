package help

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/urfave/cli/v3"
)

type HelpTopic struct {
	Name        string
	Description string
	Content     []byte
}

type DetailedHelper interface {
	Help(io.Writer) error
}

var (
	//go:embed topics/patterns.txt
	topicPatterns []byte

	topics = []HelpTopic{
		{"topics", "List available help topics", nil},
		{"patterns", "Input/output path patterns: globs, %d sequences, templates", topicPatterns},
	}
)

func Command() *cli.Command {
	return &cli.Command{
		Name:      "help",
		Usage:     "Show help or a topic (topics, patterns or a format name)",
		ArgsUsage: "[topic]",
		Action: func(ctx context.Context, c *cli.Command) error {
			return ShowCommandHelp(ctx, c.Root(), c.Args().First())
		},
	}
}

func ShowCommandHelp(ctx context.Context, command *cli.Command, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))

	if name == "" {
		return cli.ShowAppHelp(command)
	}

	if command.Command(name) != nil {
		return cli.DefaultShowCommandHelp(ctx, command, name)
	}

	writer := command.Root().Writer

	if name == "topics" {
		return printTopics(writer)
	}

	return printTopic(writer, name)
}

func printTopics(w io.Writer) error {
	var length int

	for _, topic := range topics {
		length = max(length, len(topic.Name))
	}

	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "   ffwebp help [topic]")
	fmt.Fprintln(w, "   ffwebp --help [topic]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "TOPICS:")

	for _, topic := range topics {
		fmt.Fprintf(w, "   %-*s - %s\n", length, topic.Name, topic.Description)
	}

	formats := codec.All()

	slices.SortFunc(formats, func(first, second codec.Codec) int {
		return strings.Compare(first.String(), second.String())
	})

	fmt.Fprintln(w)
	fmt.Fprintln(w, "FORMAT TOPICS (names and extensions are accepted):")

	writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	for _, format := range formats {
		fmt.Fprintf(writer, "   %s\t%s\n", format.String(), strings.Join(format.Extensions(), ", "))
	}

	return writer.Flush()
}

func printTopic(w io.Writer, name string) error {
	for _, topic := range topics {
		if topic.Name == name {
			_, err := w.Write(topic.Content)
			return err
		}
	}

	format, ok := codec.Get(name)
	if ok {
		return printFormat(w, format)
	}

	for _, format := range codec.All() {
		if slices.Contains(format.Extensions(), name) {
			return printFormat(w, format)
		}
	}

	return fmt.Errorf("unknown help topic: %q (see `ffwebp help topics`)", name)
}

func printFormat(w io.Writer, format codec.Codec) error {
	fmt.Fprintln(w, "FORMAT:")
	fmt.Fprintf(w, "   %s", format.String())

	if format.CanEncode() {
		fmt.Fprint(w, " (encode, decode)")
	} else {
		fmt.Fprint(w, " (decode)")
	}

	fmt.Fprintln(w)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "FORMAT OPTIONS:")

	flags := format.Flags(nil)
	if len(flags) == 0 {
		fmt.Fprintln(w, "   No format-specific options.")
	}

	var options strings.Builder

	writer := tabwriter.NewWriter(&options, 0, 4, 2, ' ', 0)

	for _, flag := range flags {
		fmt.Fprintf(writer, "   %s\n", flag.String())
	}

	err := writer.Flush()
	if err != nil {
		return err
	}

	err = writeHelp(w, options.String())
	if err != nil {
		return err
	}

	if detailed, ok := format.(DetailedHelper); ok {
		err = detailed.Help(w)
		if err != nil {
			return err
		}
	}

	fmt.Fprintln(w)
	_, err = fmt.Fprintln(w, "Shared conversion options: ffwebp --help")

	return err
}
