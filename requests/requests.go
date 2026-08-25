package requests

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const formtokenUrl = "https://www.irccloud.com/chat/auth-formtoken"
const sessionUrl = "https://www.irccloud.com/chat/login"

type sessionReply struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Session string `json:"session"`
	Uid     uint32 `json:"uid"`
	APIHost string `json:"api_host"`
	WSHost  string `json:"websocket_host"`
	WSPath  string `json:"websocket_path"`
	URL     string `json:"url"`
}

type formtokenReply struct {
	Id      uint32
	Success bool
	Token   string
}

func GetBacklog(apiUrl, token, endpoint string) *http.Response {
	path := fmt.Sprintf("%s%s", apiUrl, endpoint)
	client := http.Client{}

	req, _ := http.NewRequest("GET", path, nil)
	req.Header.Add("User-Agent", "irccloud-cli")
	req.Header.Add("Origin", "https://api.irccloud.com")
	req.Header.Add("Cookie", fmt.Sprintf("session=%s", token))

	resp, err := client.Do(req)

	if err != nil {
		log.Printf("Error fetching %s\n", path)
	}

	return resp
}

func GetSessionToken(user, pass string) (sessionReply, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	formToken, err := getFormtoken(httpClient)

	if err != nil {
		return sessionReply{}, fmt.Errorf("error getting session token: %v", err)
	}

	form := url.Values{}
	form.Add("token", formToken)
	form.Add("email", user)
	form.Add("password", pass)

	httpRequest, _ := http.NewRequest("POST", sessionUrl, strings.NewReader(form.Encode()))
	httpRequest.Header.Add("X-Auth-FormToken", formToken)
	httpRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(httpRequest)

	if err != nil {
		return sessionReply{}, fmt.Errorf("could not reach %s: %w", sessionUrl, err)
	}

	defer resp.Body.Close()
	return parseSession(resp)
}

func parseSession(response *http.Response) (sessionReply, error) {
	decoder := json.NewDecoder(response.Body)
	rep := &sessionReply{}

	if err := decoder.Decode(&rep); err != nil {
		if response.StatusCode < 200 || response.StatusCode > 299 {
			return sessionReply{}, fmt.Errorf("login failed: server returned %s", response.Status)
		}

		return sessionReply{}, fmt.Errorf("error parsing auth reply: %w", err)
	}

	if !rep.Success {
		return sessionReply{}, fmt.Errorf("login rejected: %s", describeLoginFailure(rep.Message))
	}

	return *rep, nil
}

// describeLoginFailure turns IRCCloud'"'"'s terse reason codes into something
// actionable. Unknown codes are passed through rather than swallowed.
func describeLoginFailure(message string) string {
	switch message {
	case "":
		return "no reason given by the server"
	case "auth":
		return "auth: incorrect email or password"
	case "email_not_verified":
		return "email_not_verified: check your inbox for a verification link"
	case "rate_limited":
		return "rate_limited: too many attempts, wait and try again"
	case "otp_required", "totp_required":
		return message + ": this account has two-factor auth enabled, which this client does not yet support"
	default:
		return message
	}
}

func getFormtoken(client *http.Client) (string, error) {
	httpRequest, _ := http.NewRequest("POST", formtokenUrl, nil)
	httpRequest.Header.Add("Content-Length", "0")
	resp, err := client.Do(httpRequest)

	if err != nil {
		return "", fmt.Errorf("error getting form token: %w", err)
	}

	defer resp.Body.Close()

	return parseToken(resp)
}

func parseToken(response *http.Response) (string, error) {
	decoder := json.NewDecoder(response.Body)
	rep := &formtokenReply{}
	err := decoder.Decode(&rep)

	if err != nil {
		return "", fmt.Errorf("can't parse token response: %w", err)
	}

	return rep.Token, nil
}
