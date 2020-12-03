package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	assert.Equal(t, &Command{
		Name:       "play",
		Parameters: make(map[string]string),
	}, CommandFromString("play"), "")
}

func TestCommandWithParams(t *testing.T) {
	assert.Equal(t, &Command{
		Name: "play",
		Parameters: map[string]string{
			"speed":       "200",
			"loop":        "true",
			"single clip": "true",
		},
	}, CommandFromString("play: speed: 200 loop: true single clip: true"), "it should parse a command string correctly")

	assert.Equal(t, &Command{
		Name: "play",
		Parameters: map[string]string{
			"speed":       "200",
			"loop":        "true",
			"single clip": "true",
		},
	}, CommandFromString("play: speed: 200 single clip: true loop: true"), "it should parse a command string correctly")

	assert.Equal(t, &Command{
		Name: "play",
		Parameters: map[string]string{
			"speed": "200",
		},
	}, CommandFromString("play: speed: 200"), "it should parse a command string correctly")
}
