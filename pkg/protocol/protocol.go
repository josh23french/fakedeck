package protocol

import "strings"

// Command represents a command
type Command struct {
	Name       string
	Parameters map[string]string
}

// FromString creates a command from a string... just like it says
func FromString(cmd string) *Command {
	parts := strings.SplitN(cmd, ":", 2)
	// "play: speed: 200"

	name := parts[0]
	params := make(map[string]string)

	if len(parts) == 2 {
		// parts[1] = " speed: 200 param2: val"
		// parts[1][1:] = "speed: 200 param2: val"
		paramParts := strings.SplitN(parts[1][1:], " ", 30)
		for {
			if len(paramParts) == 0 {
				// end of params
				break
			}
			if len(paramParts) < 2 {
				// invalid param... no value?
				break
			}
			key := paramParts[0]
			val := paramParts[1]
			if !strings.HasSuffix(key, ":") {
				// invalid key
				break
			}
			key = strings.TrimSuffix(key, ":")
			params[key] = val
			paramParts = paramParts[2:]
		}
	}

	return &Command{
		Name:       name,
		Parameters: params,
	}
}
