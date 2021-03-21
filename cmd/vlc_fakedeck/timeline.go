package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"sync"
	"time"

	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/josh23french/fakedeck/pkg/deck"
	"github.com/josh23french/fakedeck/pkg/protocol"
	"github.com/rs/zerolog/log"
	"trimmer.io/go-timecode/timecode"
)

type StopMode int

const (
	LastFrame StopMode = iota
	NextFrame
	Black
)

// TimelinePlayer is a HyperDeck-like replacement for vlc.MediaList, because it sucks
type TimelinePlayer struct {
	sync.RWMutex             // need to Lock when we mess with the clips slice
	player       *vlc.Player // reference to the Player
	clips        []Clip      // set of clips

	// calculated
	clipID       uint              // current clip ID
	prevClipsDur timecode.Timecode // sum of duration of all clips prior to clipID

	// options
	loop       bool     // are we looping the timeline?
	singleClip bool     // are we only playing the one clip?
	stopMode   StopMode // what happens when we stop? (end of timeline or singleClip, not manually)

	// state
	blanked bool // true if the media in the player is not the clip and is the blank material

	// stuff that probably belongs elsewhere
	rate   timecode.Rate
	server *deck.Server
}

func NewTimelinePlayer(player *vlc.Player, rate timecode.Rate) *TimelinePlayer {
	t := &TimelinePlayer{
		RWMutex:      sync.RWMutex{},
		player:       player,
		clips:        []Clip{},
		clipID:       1,
		prevClipsDur: timecode.New(0, rate),
		loop:         false,
		singleClip:   false,
		stopMode:     Black,
		blanked:      false,
		rate:         rate,
	}

	em, err := t.player.EventManager()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting player EventManager")
	}
	em.Attach(vlc.MediaPlayerEndReached, t.onEndReached, nil)

	return t
}

func (t *TimelinePlayer) onEndReached(event vlc.Event, userData interface{}) {
	log.Info().Msg("onEndReached!!")
	go func() {
		t.RLock()
		defer t.RUnlock()
		noNextClip := int(t.clipID)+1 >= len(t.clips)
		if t.singleClip || noNextClip {
			if t.loop {
				if t.singleClip {
					t.PlayClip(t.clipID)
				} else {
					t.PlayClip(1)
				}
				return
			}
			switch t.stopMode {
			case LastFrame:
				go t.Stop()
			case NextFrame:
				if noNextClip {
					err := t.StopOnBlack()
					if err != nil {
						log.Error().Err(err).Msg("error going to next clip after previous clip end reached")
					}
					break
				}
				t.player.SetPause(true)
				go t.Next()
			case Black:
				err := t.StopOnBlack()
				if err != nil {
					log.Error().Err(err).Msg("error going to next clip after previous clip end reached")
				}
			}
		} else {
			err := t.Next()
			if err != nil {
				log.Error().Err(err).Msg("error going to next clip after previous clip end reached")
			}
		}
	}()
}

func (t *TimelinePlayer) Timecode() timecode.Timecode {
	clipTime, err := t.player.MediaTime()
	if err != nil {
		// If the media hasn't started yet, we're at zero
		if err.Error() == "No active input" {
			return timecode.New(0, t.rate)
		}
		log.Fatal().Err(err).Msg("error getting media time")
	}
	return t.prevClipsDur.Add(time.Duration(clipTime) * time.Millisecond)
}

// TransportStatus returns the current transport status:
//  preview, stopped, play, forward, rewind, jog, shuttle, or record
func (t *TimelinePlayer) TransportStatus() string {
	if !t.blanked && t.player.IsPlaying() {
		if t.player.PlaybackRate() != 1 {
			return "forward"
		}
		return "play"
	}
	return "stopped"
}

func (t *TimelinePlayer) TransportSpeed() string {
	if t.player.IsPlaying() {
		rate := t.player.PlaybackRate() * 100
		return strconv.FormatInt(int64(rate), 10)
	}
	return "0"
}

func (t *TimelinePlayer) Play() error {
	state, err := t.player.MediaState()
	if err != nil {
		return fmt.Errorf("error getting media state: %w", err)
	}
	if t.blanked || state == vlc.MediaEnded {
		t.player.SetMedia(t.GetClipByID(t.clipID).media)
		t.player.SetMediaTime(0)
	}
	t.player.Play()
	t.sendAsyncTransportInfo()
	return nil
}

func (t *TimelinePlayer) sendAsyncTransportInfo() {
	go func() {
		time.Sleep(100 * time.Millisecond)
		note := protocol.Command{
			Name: "508 transport info",
			Parameters: map[string]string{
				"status":      t.TransportStatus(),
				"speed":       t.TransportSpeed(),
				"loop":        strconv.FormatBool(t.loop),
				"single clip": strconv.FormatBool(t.singleClip),
				"clip id":     strconv.FormatUint(uint64(t.clipID), 10),
			},
		}
		t.server.AsyncSend(note.Marshall())
	}()
}

func (t *TimelinePlayer) PlayClip(clipID uint) error {
	t.player.SetMedia(t.GetClipByID(clipID).media)
	t.player.SetMediaTime(0)
	t.player.Play()
	t.clipID = clipID
	return nil
}

func (t *TimelinePlayer) Stop() error {
	t.player.SetPause(true)
	t.sendAsyncTransportInfo()
	return nil
}

func (t *TimelinePlayer) StopOnBlack() error {
	img := image.NewGray(image.Rect(0, 0, 1, 1))
	buf := bytes.NewBuffer(nil)
	err := png.Encode(buf, img)
	if err != nil {
		return fmt.Errorf("error encoding blank screen image %w", err)
	}
	blank, err := vlc.NewMediaFromReadSeeker(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("error creating new media from blank screen image %w", err)
	}
	t.player.SetMedia(blank)
	t.player.Play()
	t.sendAsyncTransportInfo()
	return nil
}

func (t *TimelinePlayer) Next() error {
	nextClip := t.clipID + 1
	if int(nextClip) >= len(t.clips) {
		return errors.New(protocol.ErrOutOfRange)
	}
	t.player.SetMedia(t.GetClipByID(nextClip).media)
	t.clipID = nextClip
	return nil
}

func (t *TimelinePlayer) Previous() error {
	nextClip := t.clipID - 1
	if nextClip < 1 {
		return errors.New(protocol.ErrOutOfRange)
	}
	t.player.SetMedia(t.GetClipByID(nextClip).media)
	t.clipID = nextClip
	return nil
}

func (t *TimelinePlayer) GetCurrentClip() (*Clip, error) {
	if len(t.clips) == 0 {
		return nil, errors.New(protocol.ErrTimelineEmpty)
	}
	return &t.clips[t.clipID-1], nil
}

func (t *TimelinePlayer) GetClipByID(clipID uint) *Clip {
	return &t.clips[clipID-1]
}

func (t *TimelinePlayer) GetClips() []Clip {
	return t.clips
}
func (t *TimelinePlayer) Count() int {
	return len(t.clips)
}
func (t *TimelinePlayer) SetLoop(loop bool) {
	t.loop = loop
}

func (t *TimelinePlayer) recalcTimeline(from int) {

}

// AddClip appends a clip to the timeline
//
//  clips add: name: {name}                            append a clip to timeline
//  clips add: clip id: {n} name: {name}               insert clip before existing clip {n}
//  clips add: in: {inT} out: {outT} name: {name}      append the {inT} to {outT} portion of clip
//  clips remove: clip id: {n}                         remove clip {n} from the timeline
func (t *TimelinePlayer) AddClip(clip *DiskClip) error {
	t.clips = append(t.clips, Clip{
		Name:     clip.Name,
		path:     clip.path,
		media:    clip.media,
		Duration: clip.Duration,
		Start:    t.prevClipsDur,
	})
	t.prevClipsDur += clip.Duration
	if media, err := t.player.Media(); err == nil && media == nil {
		log.Info().Msg("player media not set; setting to added clip")
		t.player.SetMedia(clip.media)
	}
	return nil
}

func (t *TimelinePlayer) InsertClip(clip *Clip, afterClipID uint) error {
	afterIdx := afterClipID - 1

	// insert into slice avoiding creating a new slice
	t.clips = append(t.clips, Clip{})
	copy(t.clips[afterIdx+1:], t.clips[afterIdx:])
	t.clips[afterIdx] = *clip
	return nil
}

// clips clear
func (t *TimelinePlayer) ClearClips() error {
	// Timeline doesn't own the clip, so just forget our references
	t.clips = make([]Clip, 0)
	return nil
}
