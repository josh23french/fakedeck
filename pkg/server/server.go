package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

// Server represents a FakeDeck server, responding to clients and updating its state
type Server struct {
	clientIP string // We can only ever serve a single client; this is where we keep track of who it is
	quit     chan interface{}
}

// New constructs a new Server... duh
func New() *Server {
	return &Server{
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
				c.Write([]byte("500 connection info:\r\nprotocol version: 1.11\r\nmodel: FakeDeck 9000\r\n\r\n"))

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

					fmt.Printf("Request: %v\n", req)
					if req == "ping" {
						res = "200 ok"
					}
					if strings.HasPrefix(req, "notify:") {
						res = "209 ok"
					}
					if strings.HasPrefix(req, "transport info") {
						res = "208 transport info:\r\nstatus: stopped\r\nspeed: 0\r\nslot id: 1\r\ndisplay timecode: 00:00:00:00\r\ntimecode: 00:00:00:00\r\nclip id: 1\r\nvideo format: none\r\nloop: false\r\n"
					}
					if req == "remote" {
						res = "210 remote info:\r\nenabled: true\r\noverride: false\r\n"
					}
					if req == "clips count" {
						res = "214 clips count:\r\nclip count: 1\r\n"
					}
					if strings.HasPrefix(req, "slot info: slot id: ") {
						parts := strings.Split(req, " ")
						slotID := parts[len(parts)-1]
						res = fmt.Sprintf("202 slot info:\r\nslot id: %v\r\nstatus: mounted\r\nvolume name: Untitled\r\nrecording time: 0\r\nvideo format: 720p5994\r\n", slotID)
					}

					if strings.HasPrefix(req, "clips get: clip id: ") {
						// HH:MM:SS:FF (NDF)
						parts := strings.Split(req, " ")
						clipID := parts[len(parts)-1]
						// version 1: id: name startT duration
						// version 2: id: startT duration inT outT name
						res = fmt.Sprintf("205 clips info:\r\nclip count: 1\r\n%v: 00:00:00:00 00:00:10:00 00:00:00:00 00:00:10:00 ClipOne.mp4\r\n", clipID)
					}

					// Shut down the connection when we get the request to do so only.
					if req == "quit" {
						fmt.Printf("Told to quit... closing connection.")
						c.Close()
						s.clientIP = "" // clear the client so another can connect
						return
					}
					fmt.Printf("Responding with: %v\n", res)
					c.Write([]byte(res + "\r\n"))
				}
			}(conn)
		}
	}()
}
