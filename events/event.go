package events

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
)

type member struct {
	Nick     string `json:"nick"`
	RealName string `json:"realname"`
	Server   string `json:"ircserver"`
	UserHost string `json:"userhost"`
	UserMask string `json:"usermask"`
	Mode     string `json:"mode"`
}

type topic struct {
	Text string `json:"text"`
}

type oobInclude struct {
	Url string
}

// {"84415":{"4440297":1605131885388611}}}
type BidToEid map[string]int

// cid -> bid -> eid
type Seen map[string]BidToEid

type eventData struct {
	Type       string
	Time       int64           `json:"eid"`
	Chan       string          `json:"chan"`
	Members    []member        `json:"members"`
	From       string          `json:"from"`
	Msg        string          `json:"msg"`
	Cid        int             `json:"cid"`
	Bid        int             `json:"bid"`
	Hostmask   string          `json:"hostmask"`
	Nick       string          `json:"nick"`
	NewNick    string          `json:"newnick"`
	OldNick    string          `json:"oldnick"`
	Topic      json.RawMessage `json:"topic"`
	Author     string          `json:"author"`
	BufferType string          `json:"buffer_type"`
	Name       string          `json:"name"`
	Archived   bool            `json:"archived"`
	Created    int64           `json:"created"`
	LastEid    int             `json:"last_seen_eid"`
	SeenEids   Seen            `json:"seenEids"`
	Data       []byte
}

// getTopicText reads a topic sent as a bare string. A single event
// arriving in an unexpected shape must degrade that event, not terminate
// the client, so an unparseable topic becomes an empty one.
func getTopicText(e json.RawMessage) string {
	var dst string

	if err := json.Unmarshal(e, &dst); err != nil {
		log.Printf("ignoring malformed topic: %v", err)
		return ""
	}

	return dst
}

func UserModeString(mode string) string {
	switch mode {
	case "o":
		return "@"
	case "h":
		return "%"
	case "v":
		return "+"
	default:
		return ""
	}
}

// getTopicName reads a topic sent as an object. As with getTopicText, an
// unexpected shape yields an empty topic rather than exiting.
func getTopicName(e json.RawMessage) string {
	dst := &topic{}

	if err := json.Unmarshal(e, dst); err != nil {
		log.Printf("ignoring malformed topic: %v", err)
		return ""
	}

	return dst.Text
}

func parseBacklog(backlog *http.Response) []eventData {
	backlogData := []eventData{}
	decoder := json.NewDecoder(backlog.Body)
	if err := decoder.Decode(&backlogData); err != nil {
		// A bad backlog response loses history, not the session.
		log.Printf("could not parse backlog: %v", err)
		return nil
	}

	sort.Slice(backlogData, func(i, j int) bool {
		return backlogData[i].Time < backlogData[j].Time
	})

	return backlogData
}
