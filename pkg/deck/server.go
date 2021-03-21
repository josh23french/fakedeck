package deck

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/josh23french/fakedeck/pkg/protocol"
)

// Server represents a FakeDeck server, responding to clients and updating its state
type Server struct {
	deck     Deck
	player   *vlc.Player
	clientIP string // We can only ever serve a single client; this is where we keep track of who it is
	conn     net.Conn
	sync.RWMutex
	quit chan interface{}
}

// NewServer constructs a new Server... duh
func NewServer(d Deck) *Server {
	return &Server{
		deck:     d,
		clientIP: "",
		conn:     nil,
		quit:     make(chan interface{}),
	}
}

func (s *Server) SetPlayer(player *vlc.Player) {
	s.player = player
}

// Close stops the server
func (s *Server) Close() {
	log.Info().Msg("closing the server...")
	s.quit <- 1
}

// Serve starts a server
func (s *Server) Serve() {
	go func() {
		l, err := net.Listen("tcp", ":9993")
		if err != nil {
			log.Fatal().Err(err).Msg("could not start server")
		}
		go func() {
			<-s.quit
			err := l.Close()
			if err != nil {
				log.Fatal().Err(err).Msg("error closing listener")
			}
		}()
		for {
			// Wait for a connection.
			conn, err := l.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					log.Info().Msg("connection closing...")
					return
				}
				log.Warn().Err(err).Msg("error accepting connection")
				continue
			}
			// Handle the connection in a new goroutine.
			// The loop then returns to accepting, so that
			// multiple connections may be served concurrently.
			go func(c net.Conn) {
				clientIP := strings.SplitN(c.RemoteAddr().String(), ":", 2)[0]
				if s.clientIP != "" && clientIP != s.clientIP {
					log.Info().Msg("ClientIP isn't the one we are supposed to be talking to... closing.")
					c.Write([]byte(protocol.ErrConnRejected + "\r\n"))
					c.Close()
					return
				}
				if s.conn != nil {
					s.conn.Close()
					s.conn = nil
				}
				s.clientIP = clientIP
				s.conn = c

				watchdogSet := false
				var watchdog *time.Timer
				var watchdogDur time.Duration

				reader := bufio.NewReader(c)
				c.Write([]byte(fmt.Sprintf("500 connection info:\r\nprotocol version: %v\r\nmodel: %v\r\n\r\n", s.deck.GetProtocol(), s.deck.GetModel())))

				for {
					res := "108 internal error"
					req, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							// s.Unlock()
							continue
						}
						log.Error().Err(err).Msg("error reading from connection")
						c.Close()
						s.clientIP = "" // clear the client so another can connect
						// s.Unlock()
						return
					}

					if watchdogSet {
						watchdog.Reset(watchdogDur)
					}

					req = strings.TrimRight(req, "\r\n")
					cmd := protocol.CommandFromString(req)

					log.Info().Msgf("got request: %v", req)
					switch cmd.Name {
					case "":
						// empty command - ignore it
						// s.Unlock()
						continue
					case "ping":
						// Protocol level doesn't need to be processed by the deck
						res = "200 ok"
						break
					case "watchdog":
						periodStr, ok := cmd.Parameters["period"]
						if !ok {
							res = protocol.ErrSyntax
							break
						}
						period, err := strconv.ParseInt(periodStr, 10, 0)
						if err != nil {
							res = protocol.ErrOutOfRange
							break
						}
						if watchdogSet {
							// Stop any previous watchdog
							if !watchdog.Stop() {
								<-watchdog.C
							}
						}

						watchdogDur = time.Duration(period) * time.Second
						if period > 0 {
							watchdog = time.AfterFunc(watchdogDur, func() {
								log.Info().Msgf("watchdog timeout for %v", c.RemoteAddr())
								c.Close()
								s.clientIP = "" // clear the client so another can connect
							})
							watchdogSet = true
						} else {
							watchdogSet = false
						}
						res = "200 ok"
					case "quit": // Shut down this connection when we get the request to do so only.
						log.Info().Msg("told to quit; closing connection")
						c.Close()
						s.clientIP = "" // clear the client so another can connect
						return
					default:
						res = s.deck.ProcessCommand(cmd)
					}

					log.Info().Msgf("responding with: %v", res)

					toWrite := []byte(res + "\r\n")
					s.Lock()
					written, err := c.Write(toWrite)
					s.Unlock()
					if err != nil {
						log.Error().Err(err).Msg("error writing response")
					}
					if written != len(toWrite) {
						log.Error().Err(err).Msg("full response not written")
					}
					log.Debug().Msgf("wrote %v bytes to %v", written, s.conn.RemoteAddr().String())
					// give our async messages a little time to grab the lock if they need it... ?
					time.Sleep(100 * time.Millisecond)
				}
			}(conn)
		}
	}()
}

func (s *Server) AsyncSend(msg string) {
	if s.conn != nil {
		log.Info().Msgf(`AsyncSending "%v"`, msg)
		s.Lock()
		defer s.Unlock()
		log.Info().Msg(`AsyncSend got lock`)
		written, err := s.conn.Write([]byte(msg + "\r\n"))
		if err != nil {
			log.Error().Err(err).Msg("error writing async message")
		}
		log.Debug().Msgf("wrote %v bytes to %v", written, s.conn.RemoteAddr().String())
	}
}
