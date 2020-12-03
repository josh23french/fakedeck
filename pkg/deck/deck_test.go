package deck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlotMarshall(t *testing.T) {
	slot := &Slot{
		ID:            1,
		Status:        "empty",
		VolumeName:    "",
		RecordingTime: 0,
		VideoFormat:   VideoFormat720p5994,
		Clips:         make([]Clip, 0),
	}

	joinedLines := strings.Join(slot.Marshall(), "\r\n") + "\r\n"
	assert.Equal(t, "slot id: 1\r\nstatus: empty\r\nvolume name: \r\nrecording time: 0\r\nvideo format: 720p5994\r\n", joinedLines, "should marshall slot correctly")
}
