package protocol

import (
	"strings"
)

// Failure response codes (100-199)
const (
	ErrSyntax                = "100 syntax error"
	ErrUnsupportedParameter  = "101 unsupported parameter"
	ErrInvalidValue          = "102 invalid value"
	ErrUnsupported           = "103 unsupported"
	ErrDiskFull              = "104 disk full"
	ErrNoDisk                = "105 no disk"
	ErrDiskError             = "106 disk error"
	ErrTimelineEmpty         = "107 timeline empty"
	ErrInternal              = "108 internal error"
	ErrOutOfRange            = "109 out of range"
	ErrNoInput               = "110 no input"
	ErrRemoteControlDisabled = "111 remote control disabled"
	ErrConnRejected          = "120 connection rejected"
	ErrInvalidState          = "150 invalid state"
	ErrInvalidCodec          = "151 invalid codec"
	ErrInvalidFormat         = "160 invalid format"
	ErrInvalidToken          = "161 invalid token"
	ErrFormatNotPrepared     = "162 format not prepared"
)

// Succecssful response codes (200-299)
// 200 ok

// Asynchronous response codes (500-599)
//

// Connection Response (500)
//
// 500 connection info:
// protocol version: {Version}
// model: {Model Name}
//

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
			key := strings.TrimSpace(splitAtColon[0])
			splitAtSpace := strings.SplitN(splitAtColon[1][1:], " ", 2)

			val := strings.TrimSpace(splitAtSpace[0])
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

func (c *Command) Marshall() string {
	str := c.Name + ":\r\n"
	for param, value := range c.Parameters {
		str += param + ": " + value + "\r\n"
	}
	return str
}
