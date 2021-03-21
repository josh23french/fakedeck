package main

import (
	"errors"
	"fmt"
	"path/filepath"

	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/rs/zerolog/log"
	"trimmer.io/go-timecode/timecode"
)

// Clip is a representation of a clip along with vlc.Media
type Clip struct {
	Name     string // this is the key
	Duration timecode.Timecode
	path     string // full path to file
	media    *vlc.Media
	Start    timecode.Timecode

	cIn  uint // Inpoint of the clip
	cOut uint // Outpoint of clip
	cdur uint // Length of clip itself

	tIn  uint // Where clip starts on the timeline
	tOut uint // Where clip ends on the timeline
	tdur uint // Length of clip on the timeline
}

type DiskClip struct {
	Name     string // this is the key
	Duration timecode.Timecode
	path     string // full path to file
	media    *vlc.Media
}

func NewDiskClip(path string) (*DiskClip, error) {
	media, err := vlc.NewMediaFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("error creating new media: %v", err)
	}

	em, err := media.EventManager()
	if err != nil {
		return nil, fmt.Errorf("error getting media EventManager: %v", err)
	}

	cancelParseHandler := false
	parseErr := make(chan error, 1)
	parseDone := make(chan interface{}, 1)

	parsedEvent, err := em.Attach(vlc.MediaParsedChanged, func(event vlc.Event, userData interface{}) {
		if cancelParseHandler {
			return
		}
		status, err := media.ParseStatus()
		if err != nil {
			log.Debug().Msg("sending to parseErr 1")
			parseErr <- err
			return
		}
		log.Debug().Msg("got MediaParsedChanged event!")
		if status == vlc.MediaParseDone {
			log.Debug().Msg("sending to parseDone")
			parseDone <- 1
			return
		}
		log.Debug().Msg("sending to parseErr 2")
		parseErr <- errors.New("MediaParsedChanged handler called, but parsing wasn't done!")
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("error attaching MediaParsedChanged handler: %v", err)
	}

	err = media.ParseWithOptions(-1)
	if err != nil {
		return nil, fmt.Errorf("error starting media parse: %v", err)
	}

	// wait for parse to finish... :(
	log.Debug().Msg("waiting for parse to finish")
loop:
	for {
		select {
		case <-parseDone:
			break loop
		case err := <-parseErr:
			log.Fatal().Err(err).Msg("error parsing media")
			break loop
		default:
			// If it's already parsed and we didn't get the event, avoid an inf loop
			if status, err := media.ParseStatus(); err == nil && status != vlc.MediaParseUnstarted {
				switch status {
				case vlc.MediaParseTimeout:
					log.Fatal().Err(err).Msg("media parsing timeout")
				case vlc.MediaParseFailed:
					log.Error().Err(err).Msg("media parsing failed")
					break loop // still might work for duration even if partial
				case vlc.MediaParseSkipped:
					log.Fatal().Err(err).Msg("media parsing skipped")
				case vlc.MediaParseDone:
					log.Debug().Msg("media parse was done without the event!")
					break loop
				default:
					log.Fatal().Err(err).Msg("unknown MediaParseStatus")
				}
			}

		}
	}
	cancelParseHandler = true
	log.Debug().Msg("detaching event...")
	em.Detach(parsedEvent)
	log.Debug().Msg("closing channels...")
	close(parseErr)
	close(parseDone)

	log.Debug().Msg("getting duration...")
	dur, err := media.Duration()
	if err != nil {
		return nil, fmt.Errorf("error getting media duration: %v", err)
	}

	log.Debug().Msg("returning new DiskClip!")
	return &DiskClip{
		Name:     filepath.Base(path),
		Duration: timecode.New(dur, timecode.Rate60DF),
		path:     path,
		media:    media,
	}, nil
}

func (c *DiskClip) Media() *vlc.Media {
	return c.media
}
