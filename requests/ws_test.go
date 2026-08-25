package requests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsServer stands up a real websocket endpoint. Using a real server rather
// than a mock keeps the dial path, headers and handshake under test.
func wsServer(t *testing.T, handle func(*websocket.Conn)) (sessionReply, *httptest.Server) {
	t.Helper()

	// The client sends IRCCloud's Origin, which the default same-origin
	// check would reject against a local test server.
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}))
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")

	return sessionReply{Session: "test-token", WSHost: host, WSPath: "/", scheme: "ws"}, srv
}

// NewConnection used to call log.Fatal, which exits the process and skips
// every deferred cleanup in main.
func TestNewConnectionReturnsErrorInsteadOfExiting(t *testing.T) {
	_, err := NewConnection(sessionReply{WSHost: "127.0.0.1:1", WSPath: "/", scheme: "ws"})
	if err == nil {
		t.Fatal("expected an error dialling an unreachable host")
	}
}

func TestNewConnectionSucceedsAgainstARealServer(t *testing.T) {
	data, _ := wsServer(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"header"}`))
		time.Sleep(50 * time.Millisecond)
	})

	conn, err := NewConnection(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer conn.Close()

	msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if string(msg) != `{"type":"header"}` {
		t.Errorf("got %q", msg)
	}
}

// The session cookie must be sent, or the server has no way to authenticate.
func TestNewConnectionSendsSessionCookie(t *testing.T) {
	got := make(chan string, 1)

	// The client sends IRCCloud's Origin, which the default same-origin
	// check would reject against a local test server.
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Cookie")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	data := sessionReply{Session: "test-token", WSHost: strings.TrimPrefix(srv.URL, "http://"), WSPath: "/", scheme: "ws"}
	conn, err := NewConnection(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer conn.Close()

	select {
	case cookie := <-got:
		if !strings.Contains(cookie, "session=test-token") {
			t.Errorf("Cookie = %q, want it to carry session=test-token", cookie)
		}
	case <-time.After(time.Second):
		t.Fatal("server never saw a request")
	}
}

// Backoff must grow and stay bounded, or a dead server means a hot loop.
func TestBackoffGrowsAndIsCapped(t *testing.T) {
	var prev time.Duration

	for attempt := 0; attempt < 12; attempt++ {
		d := backoffFor(attempt)

		if d <= 0 {
			t.Fatalf("attempt %d: backoff = %v, want positive", attempt, d)
		}
		if d > maxBackoff {
			t.Errorf("attempt %d: backoff = %v, exceeds cap %v", attempt, d, maxBackoff)
		}
		if attempt > 0 && d < prev {
			t.Errorf("attempt %d: backoff shrank from %v to %v", attempt, prev, d)
		}
		prev = d
	}
}

// A dropped connection must be redialled rather than ending the session.
func TestReconnectRedialsAfterTheServerDropsTheConnection(t *testing.T) {
	var dials int32

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&dials, 1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Drop the first connection immediately; serve the second.
		if n == 1 {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte("after-reconnect"))
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	data := sessionReply{Session: "tok", WSHost: strings.TrimPrefix(srv.URL, "http://"), WSPath: "/", scheme: "ws"}

	conn, err := NewConnection(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected the first connection to drop")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Reconnect(ctx); err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}

	msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read after reconnect failed: %v", err)
	}
	if string(msg) != "after-reconnect" {
		t.Errorf("got %q, want %q", msg, "after-reconnect")
	}
	if got := atomic.LoadInt32(&dials); got != 2 {
		t.Errorf("server saw %d dials, want 2", got)
	}
}

// Reconnect must give up when told to, rather than looping forever.
func TestReconnectStopsWhenContextIsCancelled(t *testing.T) {
	conn := &Connection{session: sessionReply{WSHost: "127.0.0.1:1", WSPath: "/", scheme: "ws"}}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := conn.Reconnect(ctx)

	if err == nil {
		t.Fatal("expected Reconnect to return an error when cancelled")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("Reconnect took %v, should have honoured the deadline", elapsed)
	}
}
