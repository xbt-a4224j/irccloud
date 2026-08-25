package requests

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func replyWith(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// Regression test: a rejected login used to render as "invalid login:
// %!w(<nil>)" because %w was passed a nil error, discarding the server's
// actual reason.
func TestParseSessionSurfacesServerReason(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		contains string
	}{
		{"bad credentials", `{"success":false,"message":"auth"}`, "incorrect email or password"},
		{"rate limited", `{"success":false,"message":"rate_limited"}`, "too many attempts"},
		{"unverified email", `{"success":false,"message":"email_not_verified"}`, "verification link"},
		{"two factor", `{"success":false,"message":"otp_required"}`, "two-factor"},
		{"unknown code passed through", `{"success":false,"message":"weird_new_code"}`, "weird_new_code"},
		{"no reason given", `{"success":false}`, "no reason given"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSession(replyWith(200, tc.body))
			if err == nil {
				t.Fatal("expected an error for an unsuccessful login")
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("error = %q, want it to contain %q", err, tc.contains)
			}
			if strings.Contains(err.Error(), "%!w") {
				t.Errorf("error has a broken format verb: %q", err)
			}
		})
	}
}

func TestParseSessionReportsHTTPStatusOnGarbageBody(t *testing.T) {
	_, err := parseSession(replyWith(503, "<html>service unavailable</html>"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "503") && !strings.Contains(err.Error(), "Service Unavailable") {
		t.Errorf("error = %q, want it to mention the HTTP status", err)
	}
}

func TestParseSessionAcceptsSuccess(t *testing.T) {
	body := `{"success":true,"session":"tok","uid":7,"api_host":"https://api.irccloud.com","websocket_host":"api.irccloud.com","websocket_path":"/websocket/1"}`

	got, err := parseSession(replyWith(200, body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Session != "tok" || got.APIHost != "https://api.irccloud.com" {
		t.Errorf("session not parsed correctly: %+v", got)
	}
}

// The session token must never end up in an error string.
func TestParseSessionErrorsDoNotLeakToken(t *testing.T) {
	_, err := parseSession(replyWith(200, `{"success":false,"message":"auth","session":"SECRET-TOKEN"}`))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaked the session token: %q", err)
	}
}
