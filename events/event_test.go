package events

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// These helpers used to call log.Fatal on unparseable input, which exits
// the process. If that regresses, this test binary dies rather than
// reporting a failure -- which is itself the signal.

func TestGetTopicTextReturnsTheString(t *testing.T) {
	if got := getTopicText(json.RawMessage(`"hello world"`)); got != "hello world" {
		t.Errorf("getTopicText = %q, want %q", got, "hello world")
	}
}

func TestGetTopicTextSurvivesUnexpectedShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"object instead of string", `{"text":"hello"}`},
		{"null", `null`},
		{"number", `42`},
		{"array", `["hello"]`},
		{"empty", ``},
		{"truncated", `"unterminated`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getTopicText(json.RawMessage(tc.raw)); got != "" {
				t.Errorf("getTopicText = %q, want empty for malformed input", got)
			}
		})
	}
}

func TestGetTopicNameReadsNestedText(t *testing.T) {
	if got := getTopicName(json.RawMessage(`{"text":"channel topic"}`)); got != "channel topic" {
		t.Errorf("getTopicName = %q, want %q", got, "channel topic")
	}
}

func TestGetTopicNameSurvivesUnexpectedShapes(t *testing.T) {
	for _, raw := range []string{`"a bare string"`, `null`, `[]`, ``, `{`} {
		t.Run(raw, func(t *testing.T) {
			if got := getTopicName(json.RawMessage(raw)); got != "" {
				t.Errorf("getTopicName = %q, want empty for malformed input", got)
			}
		})
	}
}

func responseWith(body string) *http.Response {
	return &http.Response{Body: io.NopCloser(strings.NewReader(body))}
}

func TestParseBacklogSortsByTime(t *testing.T) {
	got := parseBacklog(responseWith(`[{"eid":3},{"eid":1},{"eid":2}]`))

	if len(got) != 3 {
		t.Fatalf("got %d events, want 3", len(got))
	}
	if got[0].Time != 1 || got[2].Time != 3 {
		t.Errorf("events not sorted by time: %v %v %v", got[0].Time, got[1].Time, got[2].Time)
	}
}

// A bad backlog response must not kill a client that is otherwise fine.
func TestParseBacklogSurvivesGarbage(t *testing.T) {
	for _, body := range []string{`<html>500</html>`, ``, `{"not":"an array"}`, `[{"eid":`} {
		t.Run(body, func(t *testing.T) {
			if got := parseBacklog(responseWith(body)); len(got) != 0 {
				t.Errorf("parseBacklog returned %d events for garbage input", len(got))
			}
		})
	}
}
