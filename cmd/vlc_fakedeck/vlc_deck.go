package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/josh23french/fakedeck/pkg/deck"
	"github.com/josh23french/fakedeck/pkg/protocol"
	"github.com/rs/zerolog/log"
	"trimmer.io/go-timecode/timecode"
)

// #cgo LDFLAGS: -lX11
// #include <stdlib.h>
// #include <X11/Xlib.h>
import "C"

// State is the ephemeral state of the deck
type State struct {
	loop       bool // Are we in a looping mode?
	singleClip bool // Are we in single clip mode?
	slotID     uint // 1-indexed slot ID; 0 means "none"
}

type VLCDeck struct {
	app      *gtk.Application
	timeline *TimelinePlayer
	player   *vlc.Player
	server   *deck.Server
	notify   deck.NotifyFlags
	state    State
	slots    []*Slot
	rate     timecode.Rate
}

const appID string = "com.jafrench.fakedeck.vlc_fakedeck"

func VLCDeckNew() *VLCDeck {
	C.XInitThreads()

	app, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal().Err(err).Msg("error initializing GTK Application")
	}

	// Initialize libVLC
	if err := vlc.Init("--quiet"); err != nil {
		log.Fatal().Err(err).Msg("error initializing VLC")
	}

	player, err := vlc.NewPlayer()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting Player from ListPlayer")
	}

	// Create slots
	basePath := "/home/playout/slots/" // could come from os.Args or flags
	slots := make([]*Slot, 0)
	for slotID := uint(1); slotID <= 1; slotID++ {
		slot, err := NewSlot(filepath.Join(basePath, fmt.Sprintf("%v/", slotID)))
		if err != nil {
			log.Fatal().Err(err).Msg("error making slot")
		}
		slots = append(slots, slot)
	}

	rate := timecode.Rate60DF

	d := &VLCDeck{
		app:      app,
		timeline: NewTimelinePlayer(player, rate),
		player:   player,
		server:   nil,
		notify:   deck.NotifyFlags{},
		state: State{
			slotID: 1, // gotta at least have one
		},
		slots: slots,
		rate:  rate,
	}
	d.server = deck.NewServer(d)
	slot, err := d.CurrentSlot()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting current slot")
	}

	for _, diskClip := range slot.Clips() {
		err := d.timeline.AddClip(diskClip)
		if err != nil {
			log.Fatal().Err(err).Msg("error adding clip from slot to timeline")
		}
	}

	app.Connect("activate", d.onActivate)
	app.Connect("shutdown", func() {
		glib.IdleAdd(d.PowerOff)
	})

	// VLC Events
	// Player
	eventManager, err := d.player.EventManager()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting event manager")
	}

	eventID, err := eventManager.Attach(vlc.MediaPlayerTimeChanged, d.vlcEventHandler, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error attaching event")
	}
	log.Info().Msgf("attached event: %v", eventID)

	d.timeline.server = d.server

	return d
}

func (d *VLCDeck) CurrentSlot() (*Slot, error) {
	if d.state.slotID == 0 {
		return nil, errors.New("no slots")
	}
	return d.slots[d.state.slotID-1], nil
}

func (d *VLCDeck) onActivate() {
	appWin, err := gtk.ApplicationWindowNew(d.app)
	if err != nil {
		log.Fatal().Err(err).Msg("error initializing GTK ApplicationWindow")
	}
	area, err := gtk.DrawingAreaNew()
	appWin.Add(area)
	appWin.ShowAll()
	d.app.AddWindow(appWin)

	win, err := area.ToWidget().GetWindow()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting appWin gdk.Window")
	}
	d.player.SetXWindow(win.GetXID())

	appWin.Fullscreen()

	display, _ := appWin.GetDisplay()
	blankCursor, _ := gdk.CursorNewFromName(display, "none")
	win.SetCursor(blankCursor)
}

func (d *VLCDeck) GetModel() string {
	return "VLCDeck"
}

func (d *VLCDeck) GetProtocol() string {
	return "1.11"
}

func (d *VLCDeck) ProcessCommand(cmd *protocol.Command) string {
	switch cmd.Name {
	case "help":
		return `201 help:
no help.
lol
`
	case "notify":
		for param, valStr := range cmd.Parameters {
			var valBool bool
			switch valStr {
			case "true":
				valBool = true
			case "false":
				valBool = false
			default:
				return protocol.ErrOutOfRange
			}

			switch param {
			case "transport":
				d.notify.Transport = valBool
			case "slot":
				d.notify.Slot = valBool
			case "remote":
				d.notify.Remote = valBool
			case "configuration":
				d.notify.Configuration = valBool
			case "dropped frames":
				d.notify.DroppedFrames = valBool
			case "display timecode":
				d.notify.DisplayTimecode = valBool
			case "timeline position":
				d.notify.TimelinePosition = valBool
			case "playrange":
				d.notify.PlayRange = valBool
			case "cache":
				d.notify.Cache = valBool
			case "dynamic range":
				d.notify.DynamicRange = valBool
			default:
				return protocol.ErrUnsupportedParameter
			}
		}

		return "200 ok"
	case "play":
		// if player isn't playing, can't set speed... will have to deal with slight hiccups :(
		err := d.timeline.Play()
		if err != nil {
			log.Error().Err(err).Msg("error playing player")
			return protocol.ErrInternal
		}

		// Single Clip
		if singleClip, ok := cmd.Parameters["singleClip"]; ok {
			singleClipBool, err := strconv.ParseBool(singleClip)
			if err != nil {
				return protocol.ErrOutOfRange
			}
			d.timeline.singleClip = singleClipBool
		}

		// Looping
		if loop, ok := cmd.Parameters["loop"]; ok {
			loopBool, err := strconv.ParseBool(loop)
			if err != nil {
				return protocol.ErrOutOfRange
			}
			d.timeline.SetLoop(loopBool)
		}

		// Speed
		if speedStr, ok := cmd.Parameters["speed"]; ok {
			// Convert to int64
			speed, err := strconv.ParseInt(speedStr, 10, 0)
			if err != nil {
				log.Error().Err(err).Msg("error converting play: speed parameter to int")
				return protocol.ErrSyntax
			}

			// Check param for range...
			if speed < 0 || speed > 1600 {
				// VLC does not support playing backwards
				return protocol.ErrOutOfRange
			}

			// speedFloat should be between 0 and 16 now
			speedFloat := float32(speed) / 100.0

			if speedFloat == 0 {
				err := d.timeline.Stop()
				if err != nil {
					log.Error().Err(err).Msg("error setting playback rate 0/stop")
					return protocol.ErrInternal
				}
			} else {
				err = d.player.SetPlaybackRate(speedFloat)
				if err != nil {
					log.Error().Err(err).Msgf("error setting playback rate %v", speedFloat)
					return protocol.ErrInternal
				}
			}
		}

		return "200 ok"
	case "stop":
		err := d.timeline.Stop()
		if err != nil {
			log.Error().Err(err).Msg("error pausing player")
			return protocol.ErrInternal
		}
		return "200 ok"
	case "remote":
		return "210 remote info:\r\nenabled: true\r\noverride: false\r\n"
	case "clips count":
		return fmt.Sprintf("214 clips count:\r\nclip count: %v\r\n", d.timeline.Count())
	case "disk list":
		return d.diskList(cmd.Parameters)
	case "clips get":
		return d.clipsGet(cmd.Parameters)
	case "goto":
		if clipIDStr, ok := cmd.Parameters["clip id"]; ok {
			if clipIDStr[0] == '+' || clipIDStr[0] == '-' {
				// relative
				offset, err := strconv.ParseUint(clipIDStr[1:], 10, 0)
				if err != nil {
					log.Error().Err(err).Msg("error parsing clip id")
					return protocol.ErrSyntax
				}

				if clipIDStr[0] == '+' {
					for n := uint64(0); n < offset; n++ {
						err := d.timeline.Next()
						if err != nil {
							log.Error().Err(err).Msg("error going through clips to get to offset")
							return protocol.ErrOutOfRange
						}
					}
				} else {
					for n := uint64(0); n < offset; n++ {
						err := d.timeline.Previous()
						if err != nil {
							log.Error().Err(err).Msg("error going through clips to get to offset")
							return protocol.ErrOutOfRange
						}
					}
				}
				return "200 ok"
			} else {
				// absolute
				clipID, err := strconv.ParseUint(clipIDStr, 10, 0)
				if err != nil {
					log.Error().Err(err).Msg("error parsing clip id")
					return protocol.ErrSyntax
				}
				err = d.timeline.PlayClip(uint(clipID))
				if err != nil {
					log.Error().Err(err).Msgf("error playing clip id %v", clipID)
					return protocol.ErrInternal
				}
				return "200 ok"
			}
		}
		return protocol.ErrUnsupportedParameter
	case "slot info":
		slotID := int64(1) // we only have one slot at this point... should come from deck's state
		if slotStr, ok := cmd.Parameters["slot id"]; ok {
			var err error // this is here so slotID below refers to the one in the upper scope
			slotID, err = strconv.ParseInt(slotStr, 10, 0)
			if err != nil {
				log.Error().Err(err).Msg("error parsing slot id")
				return protocol.ErrOutOfRange
			}
		}
		cmd := protocol.Command{
			Name:       "202 slot info:",
			Parameters: make(map[string]string, 0),
		}
		cmd.Parameters["slot id"] = strconv.FormatInt(slotID, 10)
		cmd.Parameters["status"] = "mounted"                      // always mounted at this point; could be "empty"
		cmd.Parameters["volume name"] = "Untitled"                // lol
		cmd.Parameters["recording time"] = "0"                    // we don't record.
		cmd.Parameters["video format"] = deck.VideoFormat720p5994 // should come from deck state, if we are going to be controlling the output resolution
		cmd.Parameters["blocked"] = "false"

		return cmd.Marshall()
	case "transport info":
		cmd := protocol.Command{
			Name:       "208 transport info:",
			Parameters: make(map[string]string, 0),
		}
		slot := strconv.FormatUint(uint64(d.state.slotID), 10)
		if d.state.slotID == 0 {
			slot = "none"
		}
		cmd.Parameters["status"] = d.timeline.TransportStatus()
		cmd.Parameters["speed"] = d.timeline.TransportSpeed()                         // -1600 through 1600
		cmd.Parameters["slot id"] = slot                                              // or none
		cmd.Parameters["clip id"] = strconv.FormatUint(uint64(d.timeline.clipID), 10) // or none??!? (HDS Mini shows clip id: 1 even when the timeline is clear!)
		cmd.Parameters["single clip"] = strconv.FormatBool(d.timeline.singleClip)
		cmd.Parameters["display timecode"] = d.timeline.Timecode().String() // timecode on front of deck
		cmd.Parameters["timecode"] = d.timeline.Timecode().String()         // timecode on timeline/playlist
		cmd.Parameters["video format"] = "720p5994"
		cmd.Parameters["loop"] = strconv.FormatBool(d.state.loop)
		cmd.Parameters["timeline"] = strconv.FormatInt(d.timeline.Timecode().Frame(), 10) // number of framess into timeline??
		cmd.Parameters["input video format"] = "none"
		cmd.Parameters["dynamic range"] = "none"

		return cmd.Marshall()
	}

	log.Warn().Msgf("unsupported command: %v", cmd)
	return protocol.ErrUnsupported
}

func (d *VLCDeck) PowerOn() {
	d.server.Serve()
	d.app.Run(os.Args)
}

func (d *VLCDeck) PowerOff() {
	d.server.Close()
	log.Debug().Msg("stopped server")

	d.app.Quit()
	log.Debug().Msg("quit app")

	err := d.timeline.Stop()
	if err != nil {
		log.Fatal().Err(err).Msg("error stopping timeline")
	}
	log.Debug().Msg("stopped timeline")

	err = vlc.Release()
	if err != nil {
		log.Fatal().Err(err).Msg("error releasing VLC")
	}
	log.Debug().Msg("released VLC")
}

func (d *VLCDeck) clipsGet(params map[string]string) (output string) {
	output += "205 clips info:\r\n"
	output += fmt.Sprintf("clips count: %v\r\n", d.timeline.Count())

	d.timeline.RLock()
	defer d.timeline.RUnlock()

	for idx, clip := range d.timeline.GetClips() {
		output += fmt.Sprintf("%v: %v %v %v\r\n", idx+1, clip.Name, clip.Start.String(), clip.Duration.String())
	}

	return output + "\r\n"
}

func (d *VLCDeck) diskList(params map[string]string) (output string) {
	slotID := d.state.slotID
	if slotStr, ok := params["slot id"]; ok {
		slot, err := strconv.ParseUint(slotStr, 10, 0)
		if err != nil {
			log.Error().Err(err).Msg("error parsing slot id")
			return protocol.ErrInternal
		}
		slotID = uint(slot)
	}
	output += "206 disk list:\r\n"
	output += fmt.Sprintf("slot id: %v\r\n", slotID)

	slot, err := d.CurrentSlot()
	if err != nil {
		return protocol.ErrNoDisk
	}

	slot.RLock()
	defer slot.RUnlock()

	for idx, clip := range slot.Clips() {
		output += fmt.Sprintf("%v: %v %v %v %v\r\n", idx+1, clip.Name, "QuickTimeProResLT", "720p5994", clip.Duration.String())
	}

	return output + "\r\n"
}

func (d *VLCDeck) vlcEventHandler(event vlc.Event, userData interface{}) {
	log.Debug().Msgf("got vlc event: %v", event)

	switch event {
	case vlc.MediaPlayerPositionChanged, vlc.MediaPlayerTimeChanged:
		log.Info().Msg("got position/time changed event")
		if d.notify.TimelinePosition {
			// Send 514 timeline position
			// timeline: 566
			//
			msg := fmt.Sprintf("514 timeline position:\r\ntimeline: %v\r\n", d.timeline.Timecode().Frame())
			d.server.AsyncSend(msg)
		}
		if d.notify.DisplayTimecode {
			// Send 513 display timecode:
			// display timecode: 00:00:06;02
			//
			msg := fmt.Sprintf("513 display timecode:\r\ndisplay timecode: %v\r\n", d.timeline.Timecode().String())
			d.server.AsyncSend(msg)
		}
	}
}
