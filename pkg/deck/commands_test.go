package deck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteInfo(t *testing.T) {
	d := NewDeck("FakeDeck 9000", 2)
	assert.Equal(t, "210 remote info:\r\nenabled: true\r\noverride: false\r\n", remoteInfo(d), "should return remoteinfo with enabled: true, override: false")

	d.Remote.Override = true
	assert.Equal(t, "210 remote info:\r\nenabled: true\r\noverride: true\r\n", remoteInfo(d), "should return remoteinfo with enabled: true, override: true")

	d.Remote.Enabled = false
	d.Remote.Override = false
	assert.Equal(t, "210 remote info:\r\nenabled: false\r\noverride: false\r\n", remoteInfo(d), "should return remoteinfo with enabled: false, override: false")
}

func TestClipsCount(t *testing.T) {
	d := NewDeck("FakeDeck 9000", 2)

	drive := NewDrive("TestDrive").WithClips([]Clip{{
		ID:   1,
		Name: "testclip1.mp4",
	}, {
		ID:   2,
		Name: "testclip2.mp4",
	}})
	assert.Equal(t, 0, d.SelectedSlot(), "no slot should be selected")
	assert.Equal(t, 2, len(drive.Clips), "drive should have 2 clips")

	err := d.InsertDrive(1, *drive)

	assert.Equal(t, nil, err, "should have no error inserting the drive")
	assert.Equal(t, 1, d.SelectedSlot(), "slot 1 should be selected")

	assert.Equal(t, 2, len(d.Clips), "deck should have 2 clips")
	assert.Equal(t, "214 clips count:\r\nclip count: 2\r\n", clipsCount(d), "it should return a proper 214 clips count message with the correct number of clips")
}
