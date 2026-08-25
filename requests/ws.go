package requests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type Connection struct {
	WSConn *websocket.Conn

	// Retained so a dropped connection can be redialled with the same
	// session token.
	session sessionReply
}

type sayMessage struct {
	Method string `json:"_method"`
	Cid    int    `json:"cid"`
	To     string `json:"to"`
	Msg    string `json:"msg"`
}

type heartbeatMessage struct {
	Method         string `json:"_method"`
	SelectedBuffer int    `json:"selectedBuffer"`
	SeenEids       string `json:"seenEids"`
}

const (
	// A dead or unreachable server must not become a hot reconnect loop.
	baseBackoff = 500 * time.Millisecond
	maxBackoff  = 30 * time.Second
)

// backoffFor reports how long to wait before reconnect attempt n. It grows
// exponentially and is capped, so a long outage settles into polling once
// every maxBackoff rather than hammering the server.
func backoffFor(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Shifting past the cap would overflow, so stop early.
	if attempt > 16 {
		return maxBackoff
	}

	d := baseBackoff << uint(attempt)
	if d > maxBackoff {
		return maxBackoff
	}

	return d
}

func dial(data sessionReply) (*websocket.Conn, error) {
	scheme := data.scheme
	if scheme == "" {
		scheme = "wss"
	}

	address := url.URL{Scheme: scheme, Host: data.WSHost, Path: data.WSPath}

	headers := http.Header{}
	headers.Add("User-Agent", "irccloud-cli")
	headers.Add("Origin", "https://api.irccloud.com")
	headers.Add("Cookie", fmt.Sprintf("session=%s", data.Session))

	conn, _, err := websocket.DefaultDialer.Dial(address.String(), headers)
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", data.WSHost, err)
	}

	return conn, nil
}

// NewConnection dials the websocket. It returns an error rather than
// exiting, so main can run its deferred cleanup.
func NewConnection(data sessionReply) (*Connection, error) {
	conn, err := dial(data)
	if err != nil {
		return nil, err
	}

	return &Connection{WSConn: conn, session: data}, nil
}

func (c *Connection) Close() error {
	if c == nil || c.WSConn == nil {
		return nil
	}

	return c.WSConn.Close()
}

// Reconnect redials with backoff until it succeeds or ctx is cancelled.
// IRCCloud sessions are long lived, so a transient network blip must not
// end the session.
func (c *Connection) Reconnect(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		wait := backoffFor(attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		conn, err := dial(c.session)
		if err == nil {
			c.WSConn = conn
			return nil
		}
	}
}

func (c *Connection) SendHeartbeat(selected, cid, bid, eid int) {
	msg := &heartbeatMessage{
		Method:         "heartbeat",
		SelectedBuffer: selected,
		SeenEids:       makeSeenEids(cid, bid, eid),
	}

	data, _ := json.Marshal(msg)
	_ = c.writeMessage(data)
}

func (c *Connection) SendMessage(cid int, channel, message string) {
	msg := &sayMessage{
		Method: "say",
		Cid:    cid,
		To:     channel,
		Msg:    message,
	}

	data, _ := json.Marshal(msg)
	_ = c.writeMessage(data)
}

func (c *Connection) writeMessage(message []byte) error {
	return c.WSConn.WriteMessage(websocket.TextMessage, message)
}

func (c *Connection) ReadMessage() ([]byte, error) {
	_, msg, err := c.WSConn.ReadMessage()

	return msg, err
}

// "{\\"3\":{\\"4\\":1343825583263721}}"
func makeSeenEids(cid, bid, eid int) string {
	return fmt.Sprintf(`{"%d":{"%d":%d}}`, cid, bid, eid)
}
