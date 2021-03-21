package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	deck := New("172.16.49.78:9993")
	assert.NotNil(t, deck)
}
