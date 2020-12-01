package deck

import (
	"fmt"

	"github.com/josh23french/fakedeck/pkg/server"
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

// RemoteFlags keeps the state of the Deck's remote functionality...
type RemoteFlags struct {
	Enabled  bool
	Override bool
}

// Slot represents a slot in the deck
type Slot struct {
	ID            int
	Status        string // empty/mounting/error/mounted - not sure how to represent this in golang...
	VolumeName    string
	RecordingTime int
	VideoFormat   string // 720p5994
}

// Deck represents the deck state
type Deck struct {
	server   server.Server
	Model    string
	NumSlots int
	Slots    []Slot
	Notify   NotifyFlags
	Remote   RemoteFlags
}

// New creates a new Deck
func New(model string, numSlots int) *Deck {
	slots := make([]Slot, 0)
	for i := 0; i < numSlots; i++ {
		slots = append(slots, Slot{})
	}
	return &Deck{
		server:   *server.New(),
		Model:    model,
		NumSlots: numSlots,
		Slots:    slots,
		Notify: NotifyFlags{
			Transport:     false,
			Slot:          false,
			Remote:        false,
			Configuration: false,
		},
		Remote: RemoteFlags{
			Enabled:  true, // Ours will default to true because there's otherwise no point to building this...
			Override: false,
		},
	}
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
