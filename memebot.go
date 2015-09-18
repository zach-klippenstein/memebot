package memebot

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

type KeywordParser interface {
	ParseKeyword(msg string) (keyword string, matched bool)
}

type RegexpKeywordParser struct {
	*regexp.Regexp
}

func (p RegexpKeywordParser) ParseKeyword(msg string) (string, bool) {
	matches := p.FindStringSubmatch(msg)
	if len(matches) < 2 {
		return "", false
	}
	// Ignore the full-string match.
	matches = matches[1:]

	log.Println("keyword matches:", strings.Join(matches, ", "))

	// Find the first non-empty capture group value.
	// First element is the entire matched string, we only care about capture groups.
	for _, match := range matches {
		if match != "" {
			return match, true
		}
	}

	return "", false
}

type MemeSearcher interface {
	// Returns ErrNoMemeFound if no meme could be found.
	FindMeme(ctx context.Context, keyword string) (Meme, error)
}

// ErrNoMemeFound is returned from MemeSearcher.FindMeme.
var ErrNoMemeFound = errors.New("no meme found")

type Handler interface {
	OnNoMemeFound(keyword string) (reply string)
	OnPhraseNotUnderstood(phrase string) (reply string)
}

type DefaultHandler struct{}

func (DefaultHandler) OnNoMemeFound(keyword string) string {
	return fmt.Sprintf("Sorry, I couldn't find a meme for “%s”", keyword)
}

func (DefaultHandler) OnPhraseNotUnderstood(phrase string) string {
	return fmt.Sprintln("Sorry, I'm not sure what you mean by:\n>", phrase)
}

type MemeBot struct {
	Parser   KeywordParser
	Searcher MemeSearcher
	Handler  Handler

	// If a message hasn't been replied to in this time, don't reply.
	// Prevents the bot from replying to messages too late and not making sense.
	MaxReplyTimeout time.Duration

	startResponse *ResponseRtmStart
	ws            *websocket.Conn
}

func (b *MemeBot) Dial(authToken string) {
	if b.ws != nil {
		log.Panic("bot already connected")
	}

	ws, startResponse, err := slackConnect(authToken)
	if err != nil {
		log.Fatal("error connecting to slack:", err)
	}

	b.startResponse = startResponse
	b.ws = ws
}

func (b *MemeBot) Name() string {
	return b.startResponse.Self.Name
}

func (b *MemeBot) Channels() []Channel {
	return b.startResponse.Channels
}

func (b *MemeBot) Run(ctx context.Context) {
	defer b.Close()

	for {
		// read each incoming message
		m, err := getMessage(ctx, b.ws)
		if err != nil {
			log.Println("error reading message from websocket:", err)
		}

		if b.isMessageToSelf(m) {
			go b.handleMessageToSelf(ctx, m)
		}
	}
}

func (b *MemeBot) Close() error {
	if b.ws != nil {
		return b.ws.Close()
	}
	return nil
}

func (b *MemeBot) isMessageToSelf(msg Message) bool {
	return msg.IsMessage() && msg.IsUserMentioned(b.startResponse.Self.Id)
}

func (b *MemeBot) maxReplyTimeout() time.Duration {
	if b.MaxReplyTimeout > 0 {
		return b.MaxReplyTimeout
	}
	return 5 * time.Second
}

func (b *MemeBot) handleMessageToSelf(ctx context.Context, m Message) {
	ctx, cancel := context.WithTimeout(ctx, b.maxReplyTimeout())
	defer cancel()

	keyword, matched := b.Parser.ParseKeyword(m.Text)
	if !matched {
		reply := b.Handler.OnPhraseNotUnderstood(m.Text)
		b.postMessage(ctx, m.Reply(reply))
		return
	}

	meme, err := b.Searcher.FindMeme(ctx, keyword)
	if err == ErrNoMemeFound {
		log.Println("couldn't find a meme for keyword:", keyword)
		reply := b.Handler.OnNoMemeFound(keyword)
		b.postMessage(ctx, m.Reply(reply))
		return
	} else if err != nil {
		log.Printf("error searching for '%s': %s", keyword, err)
		reply := b.Handler.OnNoMemeFound(keyword)
		b.postMessage(ctx, m.Reply(reply))
		return
	}

	b.postMessage(ctx, m.Reply(meme.URL().String()))
}

func (b *MemeBot) postMessage(ctx context.Context, msg Message) {
	select {
	case <-ctx.Done():
		log.Print("context done, not sending reply:", ctx.Err(), "\n\t", msg)
		return
	default:
		if err := postMessage(b.ws, msg); err != nil {
			log.Print("error posting message:", err, "\n\t", msg)
		}
	}
}
