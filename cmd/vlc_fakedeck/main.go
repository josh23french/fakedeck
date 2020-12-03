package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/josh23french/fakedeck/pkg/deck"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	d := deck.NewDeck("FakeDeck 9000", 2)

	// Initialize libVLC. Additional command line arguments can be passed in
	// to libVLC by specifying them in the Init function.
	if err := vlc.Init("--no-video", "--quiet"); err != nil {
		log.Fatal(err)
	}
	defer vlc.Release()

	// Create a new player.
	player, err := vlc.NewPlayer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		player.Stop()
		player.Release()
	}()

	d.InsertDrive(1, *deck.NewDrive("Test").WithClips([]deck.Clip{
		{
			ID:       1,
			Name:     "test.mp4",
			In:       "00:00:00:00",
			Out:      "00:00:09:59",
			Start:    "00:00:09:59",
			Duration: "00:00:09:59",
		},
		{
			ID:       2,
			Name:     "test2.mp4",
			In:       "00:00:10:00",
			Out:      "00:00:19:59",
			Start:    "00:00:00:00",
			Duration: "00:00:09:59",
		},
	}))

	d.PowerOn()

	for {
		select {
		case <-sigs:
			d.PowerOff()
			return
		default:
			// nada
		}
	}
}
