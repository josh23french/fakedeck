package deck

import (
	"errors"
	"fmt"
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

// Deck represents the deck state
type Deck struct {
	server    Server
	Model     string
	NumSlots  int
	Slots     []Slot
	Transport Transport
	Notify    NotifyFlags
	Remote    RemoteFlags
	Clips     []Clip
}

// NewDeck creates a new Deck
func NewDeck(model string, numSlots int) *Deck {
	slots := make([]Slot, 0)
	for i := 1; i <= numSlots; i++ {
		slots = append(slots, Slot{
			ID:            i,
			Status:        "empty",
			VolumeName:    "",
			RecordingTime: 0,
			VideoFormat:   VideoFormat720p5994, // Should be loaded from some saved state mimicing the Deck's NVRAM?
			Clips:         make([]Clip, 0),
		})
	}
	deck := &Deck{
		server:   Server{}, // uninitialized now, but will be before end of constructor
		Model:    model,
		NumSlots: numSlots,
		Slots:    slots,
		Transport: Transport{
			SlotID:      0,
			Speed:       0,
			ClipID:      0,
			VideoFormat: VideoFormat720p5994,
			Loop:        false,
		},
		Notify: NotifyFlags{ // All notifications are off by default
			Transport:        false,
			Slot:             false,
			Remote:           false,
			Configuration:    false,
			DroppedFrames:    false,
			DisplayTimecode:  false,
			TimelinePosition: false,
			PlayRange:        false,
			DynamicRange:     false,
		},
		Remote: RemoteFlags{
			Enabled:  true, // Ours will default to true because there's otherwise no point to building this...
			Override: false,
		},
		Clips: make([]Clip, 0),
	}
	deck.server = *NewServer(deck)
	return deck
}

// PowerOn starts up the deck, including the server
func (d *Deck) PowerOn() {
	fmt.Printf("Booting FakeDeck Model \"%v\" with %v slots...\n", d.Model, d.NumSlots)
	d.server.Serve()
}

// PowerOff shuts everything down that we own
func (d *Deck) PowerOff() {
	d.server.Close()
}

func (d *Deck) slotOccupied(slot int) (bool, error) {
	for _, s := range d.Slots {
		if s.ID == slot {
			switch s.Status {
			case "empty":
				return false, nil
			case "error":
			case "mounting":
			case "mounted":
			default:
				return true, nil
			}
		}
	}
	return false, errors.New("slot doesn't exist") // Could also mean the slot doesn't exist
}

// SelectedSlot returns the currently-active slot of the Deck
func (d *Deck) SelectedSlot() int {
	return d.Transport.SlotID
}

// SelectSlot makes the slot active and loads the clips
func (d *Deck) SelectSlot(slot int) error {
	for _, s := range d.Slots {
		if s.ID == slot {
			fmt.Printf("Loading slot %+v\n", s)
			d.Clips = s.Clips // Load the clips!
			// TODO: only load clips of the currently-active video format?
			d.Transport.SlotID = s.ID
			return nil
		}
	}
	return errors.New("slot went away?")
}

// InsertDrive does what it says on the tin
func (d *Deck) InsertDrive(slot int, drive Drive) error {
	occ, err := d.slotOccupied(slot)
	if err != nil {
		return err
	}
	if occ {
		return errors.New("slot doesn't exist or is occupied")
	}
	// Insert the drive
	for i := range d.Slots {
		if d.Slots[i].ID == slot {
			d.Slots[i].Status = "mounting"
			d.Slots[i].VolumeName = drive.VolumeName
			d.Slots[i].Clips = drive.Clips
			d.Slots[i].Status = "mounted"

			// When inserting a drive, and we don't have one selected, select the newly-inserted one automatically
			if d.SelectedSlot() == 0 {
				return d.SelectSlot(d.Slots[i].ID)
			}

			return nil
		}
	}
	return errors.New("slot went away?")
}
