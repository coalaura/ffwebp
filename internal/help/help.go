package help

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v3"
)

type HelpTopic struct {
	Name        string
	Description string
	Content     []byte
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
		Usage:     "Show help or a specific topic",
		ArgsUsage: "[topic]",
		Action: func(ctx context.Context, c *cli.Command) error {
			args := c.Args().Slice()

			if len(args) == 0 {
				parent := c.Root()

				if parent == nil {
					parent = c
				}

				return cli.ShowAppHelp(parent)
			}

			topic := strings.ToLower(strings.TrimSpace(args[0]))

			switch topic {
			case "topics":
				return printTopics(c.Writer)
			default:
				return printTopic(c.Writer, topic)
			}
		},
	}
}

func printTopics(w io.Writer) error {
	var length int

	for _, topic := range topics {
		length = max(length, len(topic.Name))
	}

	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "   ffwebp help [topic]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "TOPICS:")

	for _, topic := range topics {
		fmt.Fprintf(w, "   %-*s - %s\n", length, topic.Name, topic.Description)
	}

	return nil
}

func printTopic(w io.Writer, name string) error {
	var topic *HelpTopic

	for _, tp := range topics {
		if tp.Name == name {
			topic = &tp
		}
	}

	if topic == nil {
		return fmt.Errorf("unknown help topic: %q (see `ffwebp help topics`)", name)
	}

	_, err := w.Write(topic.Content)
	return err
}
