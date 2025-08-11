package effects

import (
	"fmt"
	"image"
	"strings"

	"github.com/urfave/cli/v3"
)

type Effect interface {
	String() string
	Apply(image.Image, string) (image.Image, error)
}

type EffectConfig struct {
	Effect    Effect
	Arguments string
}

var (
	apply    string
	names    []string
	registry = make(map[string]Effect)
)

func Register(e Effect) {
	name := e.String()

	names = append(names, name)

	registry[name] = e
}

func HasEffects() bool {
	return len(registry) > 0
}

func Flags(flags []cli.Flag) []cli.Flag {
	if len(registry) == 0 {
		return flags
	}

	return append(flags,
		&cli.StringFlag{
			Name:        "effects",
			Aliases:     []string{"e"},
			Usage:       fmt.Sprintf("list of effects to apply (%s)", strings.Join(names, ", ")),
			Value:       "",
			Destination: &apply,
		},
	)
}

func ApplyAll(img image.Image) (image.Image, int, error) {
	if apply == "" {
		return img, 0, nil
	}

	list, err := Parse()
	if err != nil {
		return nil, 0, err
	} else if len(list) == 0 {
		return img, 0, nil
	}

	for _, e := range list {
		img, err = e.Effect.Apply(img, e.Arguments)
		if err != nil {
			return nil, 0, err
		}
	}

	return img, len(list), nil
}

func Parse() ([]EffectConfig, error) {
	var result []EffectConfig

	for entry := range strings.SplitSeq(apply, ",") {
		var (
			name      = entry
			arguments string
		)

		if index := strings.Index(entry, ":"); index != -1 {
			name = entry[:index]
			arguments = entry[index+1:]
		}

		effect, ok := registry[name]
		if !ok {
			return nil, fmt.Errorf("invalid effect: %s", name)
		}

		result = append(result, EffectConfig{
			Effect:    effect,
			Arguments: arguments,
		})
	}

	return result, nil
}
