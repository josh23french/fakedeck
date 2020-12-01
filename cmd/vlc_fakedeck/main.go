package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/josh23french/fakedeck/pkg/deck"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	deck := deck.New("FakeDeck 9000", 4)
	deck.PowerOn()

	for {
		select {
		case <-sigs:
			deck.PowerOff()
			return
		default:
			// nada
		}
	}
}
