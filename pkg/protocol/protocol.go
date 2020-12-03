package protocol

import (
	"strings"
)

// Failure Response Codes
const (
	ErrorInvalidValue = "102 invalid value"
	ErrorUnsupported  = "103 unsupported"
)

// Command represents a command
type Command struct {
	Name       string
	Parameters map[string]string
}

// CommandFromString creates a command from a string... just like it says
func CommandFromString(cmd string) *Command {
	parts := strings.SplitN(cmd, ":", 2)
	name := parts[0]
	params := make(map[string]string)

	if len(parts) == 2 {
		remainder := parts[1][1:]
		for {
			splitAtColon := strings.SplitN(remainder, ":", 2)
			if len(splitAtColon) == 1 {
				// No colon... stop parsing params
				break
			}
			key := splitAtColon[0]
			splitAtSpace := strings.SplitN(splitAtColon[1][1:], " ", 2)

			val := splitAtSpace[0]
			params[key] = val

			remainder = ""
			if len(splitAtSpace) == 2 {
				remainder = splitAtSpace[1]
			}
		}
	}

	return &Command{
		Name:       name,
		Parameters: params,
	}
}
