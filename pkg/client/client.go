// Package client is a Hyperdeck protocol client
package client

import (
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents a Hyperdeck protocol client
type Client struct {
	remoteHost string   // remoteHost is the host:port string to (re)connect to
	conn       net.Conn // conn may be a valid connection, but it might not be...
}

// New creates a new Client
func New(remoteHost string) *Client {
	client := &Client{
		conn: nil,
	}
	err := client.tryConnect()
	if err != nil {
		log.Warn().Err(err).Msg("error connecting to remote host")
	}
	return client
}

func (c *Client) tryConnect() error {
	if c.conn != nil {
		c.conn.Close()
	}
	conn, err := net.DialTimeout("tcp", c.remoteHost, 1*time.Second)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}
