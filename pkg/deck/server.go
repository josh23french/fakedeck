package deck

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/josh23french/fakedeck/pkg/protocol"
)

// Server represents a FakeDeck server, responding to clients and updating its state
type Server struct {
	deck     *Deck
	clientIP string // We can only ever serve a single client; this is where we keep track of who it is
	quit     chan interface{}
}

// NewServer constructs a new Server... duh
func NewServer(d *Deck) *Server {
	return &Server{
		deck:     d,
		clientIP: "",
		quit:     make(chan interface{}),
	}
}

// Close stops the server
func (s *Server) Close() {
	fmt.Println("Closing the server...")
	s.quit <- 1
}

// Serve starts a server
func (s *Server) Serve() {
	go func() {
		l, err := net.Listen("tcp", ":9993")
		if err != nil {
			log.Fatal(err)
		}
		defer l.Close()
		go func() {
			<-s.quit
			l.Close()
		}()
		for {
			// Wait for a connection.
			conn, err := l.Accept()
			if err != nil {
				fmt.Printf("%v", fmt.Errorf("Error accepting connection: %w", err))
				continue
			}
			// Handle the connection in a new goroutine.
			// The loop then returns to accepting, so that
			// multiple connections may be served concurrently.
			go func(c net.Conn) {
				clientIP := strings.SplitN(c.RemoteAddr().String(), ":", 2)[0]
				if s.clientIP != "" && clientIP != s.clientIP {
					fmt.Printf("ClientIP isn't the one we are supposed to be talking to... closing.\n")
					c.Write([]byte("120 connection rejected\r\n"))
					c.Close()
					return
				}

				reader := bufio.NewReader(c)
				c.Write([]byte(fmt.Sprintf("500 connection info:\r\nprotocol version: 1.11\r\nmodel: %v\r\n\r\n", s.deck.Model)))

			conn_loop:
				for {
					res := "108 internal error"
					req, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							continue
						}
						fmt.Println(fmt.Errorf("Error: %w", err))
						c.Close()
						return
					}

					req = strings.TrimRight(req, "\r\n")
					cmd := protocol.CommandFromString(req)

					fmt.Printf("Request: %v\n", req)
					switch cmd.Name {
					case "ping":
						res = "200 ok"
						break
					case "notify":
						// for each param, set the corresponding notifyflag
						s.deck.Notify.Configuration = true

						res = "209 ok"
						break
					case "transport info":
						res = "208 transport info:\r\nstatus: stopped\r\nspeed: 0\r\nslot id: 1\r\ndisplay timecode: 00:00:00:00\r\ntimecode: 00:00:00:00\r\nclip id: 1\r\nvideo format: 720p5994\r\nloop: false\r\n"
						break
					case "remote":
						res = remoteInfo(s.deck)
						break
					case "goto":
						// goto: clip id: 2
						res = "ok"
						break
					case "play":
						// play: single clip: true loop: false speed: 100
						s.deck.Transport.Status = "playing"
						s.deck.Transport.Speed = 100
						s.deck.Transport.Timecode = "00:00:00:01"
						res = "ok"
						break
					case "clips count":
						res = clipsCount(s.deck)
						break
					case "slot info":
						slot := s.deck.Slots[s.deck.Transport.SlotID]
						if slotID, ok := cmd.Parameters["slot id"]; ok {
							slotID, err := strconv.ParseInt(slotID, 10, 0)
							if err != nil {
								fmt.Printf("Error parsing int from slot id: %v\n", err)
								break // out to internal error
							}
							for _, s := range s.deck.Slots {
								if s.ID == int(slotID) {
									slot = s
									break
								}
							}
						}
						lines := make([]string, 0)
						lines = append(lines, "202 slot info:")
						lines = append(lines, slot.Marshall()...)

						res = strings.Join(lines, "\r\n") + "\r\n"
						break
					case "clips get":
						// HH:MM:SS:FF (NDF)
						clips := s.deck.Clips
						// askedForSingleClip := false
						if clipID, ok := cmd.Parameters["clip id"]; ok {
							clipID, err := strconv.ParseInt(clipID, 10, 0)
							if err != nil {
								fmt.Printf("Error parsing int from slot id: %v\n", err)
								c.Write([]byte(protocol.ErrorInvalidValue + "\r\n"))
								continue conn_loop
							}
							// askedForSingleClip = true
							newClips := make([]Clip, 0)
							for _, c := range s.deck.Clips {
								if c.ID == int(clipID) {
									newClips = append(newClips, c)
									break
								}
							}
							clips = newClips
						}
						// version 1: id: name startT duration
						// version 2: id: startT duration inT outT name

						lines := make([]string, 0)
						lines = append(lines, "205 clips info:")
						// if !askedForSingleClip {
						lines = append(lines, fmt.Sprintf("clip count: %v", len(clips)))
						// }
						for _, clip := range clips {
							lines = append(lines, fmt.Sprintf("%v: %v %v %v", clip.ID, clip.Name, clip.Start, clip.Duration))
							// lines = append(lines, fmt.Sprintf("%v: %v %v %v %v %v", clip.ID, clip.Start, clip.Duration, clip.In, clip.Out, clip.Name))
						}

						res = strings.Join(lines, "\r\n") + "\r\n"
						break
					case "quit": // Shut down the connection when we get the request to do so only.
						fmt.Printf("Told to quit... closing connection.")
						c.Close()
						s.clientIP = "" // clear the client so another can connect
						return
					default:
						res = "108 internal error"
					}

					fmt.Printf("Responding with: %v\n", res)
					c.Write([]byte(res + "\r\n"))
				}
			}(conn)
		}
	}()
}
