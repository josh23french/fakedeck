package deck

import (
	"fmt"
	"strconv"

	"github.com/josh23french/fakedeck/pkg/protocol"
)

// NotifyFlags keep the state of what updates should send/receive async messages
type NotifyFlags struct {
	Transport        bool
	Slot             bool
	Remote           bool
	Configuration    bool
	DroppedFrames    bool
	DisplayTimecode  bool
	TimelinePosition bool
	PlayRange        bool
	DynamicRange     bool
	Cache            bool
}

// Video Formats, prefixed with VideoFormat because apparently starting a const with a number is illegal now... :(
const (
	// SD
	VideoFormatNTSC  = "NTSC"
	VideoFormatPAL   = "PAL"
	VideoFormatNTSCp = "NTSCp"
	VideoFormatPALp  = "PALp"
	// 720
	VideoFormat720p50   = "720p50"
	VideoFormat720p5994 = "720p5994"
	VideoFormat720p60   = "720p60"
	// 1080
	VideoFormat1080p23976 = "1080p23976"
	VideoFormat1080p24    = "1080p24"
	VideoFormat1080p25    = "1080p25"
	VideoFormat1080p2997  = "1080p2997"
	VideoFormat1080p30    = "1080p30"
	VideoFormat1080i50    = "1080i50"
	VideoFormat1080i5994  = "1080i5994"
	VideoFormat1080i60    = "1080i60"
	// 4K
	VideoFormat4Kp23976 = "4Kp23976"
	VideoFormat4Kp24    = "4Kp24"
	VideoFormat4Kp25    = "4Kp25"
	VideoFormat4Kp2997  = "4Kp2997"
	VideoFormat4Kp30    = "4Kp30"
	// No 4Kp60 ???
)

// RemoteFlags keeps the state of the Deck's remote functionality...
type RemoteFlags struct {
	Enabled  bool
	Override bool
}

// Drive represents a drive you can insert into a slot
type Drive struct {
	VolumeName string
	Clips      []Clip
}

// NewDrive creates a new drive... durr
func NewDrive(name string) *Drive {
	return &Drive{
		VolumeName: name,
		Clips:      make([]Clip, 0),
	}
}

// WithClips sets the clips on a newly-created Drive and returns it so it's chainable
func (d *Drive) WithClips(clips []Clip) *Drive {
	d.Clips = clips
	return d
}

// Slot represents a slot in the deck
type Slot struct {
	ID            int
	Status        string // empty/mounting/error/mounted - not sure how to represent this in golang...
	VolumeName    string
	RecordingTime int
	VideoFormat   string // 720p5994
	Clips         []Clip
}

// Marshall turns the Slot into a slice of strings
func (s *Slot) Marshall() []string {
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("slot id: %v", s.ID))
	lines = append(lines, fmt.Sprintf("status: %v", s.Status))
	lines = append(lines, fmt.Sprintf("volume name: %v", s.VolumeName))
	lines = append(lines, fmt.Sprintf("recording time: %v", s.RecordingTime))
	lines = append(lines, fmt.Sprintf("video format: %v", s.VideoFormat))
	return lines
}

// Timecode is a timestamp
type Timecode string

// Clip represents a clip
type Clip struct {
	ID       int
	Name     string
	In       Timecode
	Out      Timecode
	Start    Timecode
	Duration Timecode
}

// Transport represents a transport
type Transport struct {
	Status          string
	Speed           int
	SlotID          int
	DisplayTimecode Timecode
	Timecode        Timecode
	ClipID          int
	VideoFormat     string
	Loop            bool
}

// Marshall turns the Transport into a slice of strings
func (t *Transport) Marshall() []string {
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("status: %v", t.Status))
	lines = append(lines, fmt.Sprintf("speed: %v", t.Speed))

	slot := "none"
	if t.SlotID > 0 {
		slot = strconv.FormatInt(int64(t.SlotID), 10)
	}
	lines = append(lines, fmt.Sprintf("slot id: %v", slot))

	clip := "1"
	if t.ClipID > 0 {
		clip = strconv.FormatInt(int64(t.ClipID), 10)
	}
	lines = append(lines, fmt.Sprintf("clip id: %v", clip))

	lines = append(lines, fmt.Sprintf("display timecode: %v", t.DisplayTimecode))
	lines = append(lines, fmt.Sprintf("timecode: %v", t.Timecode))
	lines = append(lines, fmt.Sprintf("video format: %v", t.VideoFormat))
	lines = append(lines, fmt.Sprintf("loop: %v", t.Loop))
	return lines
}

// Deck represents the deck state
type Deck interface {
	GetModel() string                        // returns the model of the deck
	GetProtocol() string                     // returns the protocol version supported
	ProcessCommand(*protocol.Command) string // returns a response to the command
	PowerOn()                                // start the server and output
	PowerOff()                               // clean up server and output
	// ClientConnected()                        // resets the per-client settings when the client connects
}
