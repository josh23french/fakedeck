package main

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"sort"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/josh23french/fakedeck/pkg/protocol"
	"github.com/rs/zerolog/log"
)

type Slot struct {
	sync.RWMutex
	clips   []*DiskClip
	path    string // path to folder
	watcher *fsnotify.Watcher
}

func NewSlot(path string) (*Slot, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("error creating watcher")
		return nil, err
	}
	s := &Slot{
		RWMutex: sync.RWMutex{},
		clips:   make([]*DiskClip, 0),
		path:    path,
		watcher: watcher,
	}

	// start the loop... can be stopped by s.watcher.Close()
	go s.loop()
	watcher.Add(s.path)

	// Read all clips into vlc media objects
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		log.Error().Err(err).Msgf("error reading directory: %v", s.path)
		return nil, err
	}
	for idx, file := range files {
		log.Info().Msgf("File %v: %v", idx, file)
		path := filepath.Join(s.path, file.Name())
		newClip, err := NewDiskClip(path)
		if err != nil {
			log.Error().Err(err).Msgf("error creating new disk clip: %v", file.Name())
			continue
		}
		err = s.AddClip(newClip)
		if err != nil {
			log.Error().Err(err).Msgf("error adding initial clip to slot: %v", file.Name())
			continue // if a clip fails, it doesn't mean there's something wrong with the slot
		}
	}

	return s, nil
}

func (s *Slot) GetClip(name string) (*DiskClip, error) {
	s.RLock()
	defer s.RUnlock()
	for _, clip := range s.Clips() {
		if clip.Name == name {
			return clip, nil
		}
	}
	return nil, errors.New(protocol.ErrOutOfRange)
}

func (s *Slot) AddClip(clip *DiskClip) error {
	s.Lock()
	defer s.Unlock()

	// check for existing clips first with the same name, and just overwrite
	for idx, existingClip := range s.clips {
		if existingClip.Name == clip.Name {
			s.clips[idx] = clip
			return nil
		}
	}

	s.clips = append(s.clips, clip)
	sort.SliceStable(s.clips, func(i, j int) bool {
		return s.clips[i].Name < s.clips[j].Name
	})
	return nil
}

func (s *Slot) RemoveClip(name string) error {
	s.Lock()
	defer s.Unlock()
	newLen := len(s.clips) - 1
	idx := -1
	for i, clip := range s.clips {
		if clip.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New(protocol.ErrOutOfRange)
	}

	copy(s.clips[idx:], s.clips[idx+1:])
	s.clips[newLen] = nil
	s.clips = s.clips[:newLen]
	return nil
}

func (s *Slot) Clips() []*DiskClip {
	return s.clips
}

func (s *Slot) loop() {
	log.Info().Msgf("started watcher handler loop: %v", s.path)
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				log.Error().Msg("watcher.Events not ok")
				return
			}
			log.Info().Msgf("event: %v", event)
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				log.Info().Msgf("saw new/changed file: %v", event.Name)
				newClip, err := NewDiskClip(event.Name)
				if err != nil {
					log.Error().Err(err).Msgf("error creating new disk clip: %v", event.Name)
					continue
				}
				s.AddClip(newClip) // Name is the whole path already
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
				log.Info().Msgf("saw deleted/renamed file: %v", event.Name)
				name := filepath.Base(event.Name)
				s.RemoveClip(name)
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msgf("error: %v", err)
		}
	}
}
