/*

mybot - Illustrative Slack bot in Go

Copyright (c) 2015 RapidLoop

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package memebot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// These two structures represent the response of the Slack API rtm.start.
// Only some fields are included. The rest are ignored by json.Unmarshal.

type ResponseRtmStart struct {
	Ok       bool      `json:"ok"`
	Error    string    `json:"error"`
	Url      string    `json:"url"`
	Self     Self      `json:"self"`
	Channels []Channel `json:"channels"`
}

// Self contains information about this bot's identity on the server.
type Self struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Channel struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	IsMember bool   `json:"is_member"`
}

// Starts a websocket-based Real Time API session and return the websocket
// and the ID of the (bot-)user whom the token belongs to.
func slackConnect(token string) (*websocket.Conn, *ResponseRtmStart, error) {
	response, err := slackStart(token)
	if err != nil {
		return nil, nil, err
	}

	ws, err := websocket.Dial(response.Url, "", "https://api.slack.com/")
	if err != nil {
		return nil, nil, err
	}

	return ws, response, nil
}

// slackStart does a rtm.start, and returns a websocket URL and user ID. The
// websocket URL can be used to initiate an RTM session.
func slackStart(token string) (response *ResponseRtmStart, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}
	response = &ResponseRtmStart{}
	err = json.Unmarshal(body, response)
	if err != nil {
		return
	}

	if !response.Ok {
		err = fmt.Errorf("Slack error: %s", response.Error)
		return
	}

	return
}

// Message are the messages read off and written into the websocket. Since this
// struct serves as both read and write, we include the "Id" field which is
// required only for writing.
type Message struct {
	Id      uint64 `json:"id"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func (m Message) IsMessage() bool {
	return m.Type == "message"
}

func (m Message) IsUserMentioned(id string) bool {
	return strings.HasPrefix(m.Text, "<@"+id+">")
}

func (m Message) Reply(reply string) Message {
	m.Text = reply
	return m
}

func getMessage(ctx context.Context, ws *websocket.Conn) (m Message, err error) {
	if deadline, ok := ctx.Deadline(); ok {
		ws.SetReadDeadline(deadline)
	}
	if err = websocket.JSON.Receive(ws, &m); err != nil {
		return
	}
	return
}

var counter uint64

func postMessage(ws *websocket.Conn, m Message) error {
	m.Id = atomic.AddUint64(&counter, 1)
	return websocket.JSON.Send(ws, m)
}
