package memebot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/nlopes/slack"
	"golang.org/x/net/context"
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
	FindMeme(keyword string) (Meme, error)
}

// ErrNoMemeFound is returned from MemeSearcher.FindMeme.
var ErrNoMemeFound = errors.New("no meme found")

type ErrorHandler interface {
	OnNoMemeFound(keyword string) (reply string)
	OnPhraseNotUnderstood(phrase string) (reply string)
}

type DefaultErrorHandler struct{}

func (DefaultErrorHandler) OnNoMemeFound(keyword string) string {
	return fmt.Sprintf("Sorry, I couldn't find a meme for “%s”", keyword)
}

func (DefaultErrorHandler) OnPhraseNotUnderstood(phrase string) string {
	return fmt.Sprintln("Sorry, I'm not sure what you mean by:\n>", phrase)
}

type MemeBotConfig struct {
	Parser   KeywordParser
	Searcher MemeSearcher

	// Defaults to DefaultErrorHandler{}.
	ErrorHandler ErrorHandler

	// Default will not print any log messages.
	Log *log.Logger

	// If a message hasn't been replied to in this time, don't reply.
	// Prevents the bot from replying to messages too late and not making sense.
	MaxReplyTimeout time.Duration

	// If true, will try to match keywords on all messages, not just ones where
	// the bot is mentioned.
	// The ErrorHandler's OnPhraseNotUnderstood will still only be called if the
	// bot was mentioned.
	ParseAllMessages bool
}

type MemeBot struct {
	config MemeBotConfig

	rtm       *slack.RTM
	slackInfo *slack.Info

	// Map of channel ID to channel.
	channels map[string]*slack.Channel
}

var (
	ErrInvalidAuthToken = errors.New("invalid auth token")
	ErrConnectionFailed = errors.New("failed to connect to slack")
)

const DefaultReplyTimeout = 5 * time.Second

func NewMemeBot(authToken string, config MemeBotConfig) (bot *MemeBot, err error) {
	// Setup default config values.
	if config.Parser == nil {
		panic("Parser must be specified")
	}
	if config.Searcher == nil {
		panic("Searcher must be specified")
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultErrorHandler{}
	}
	if config.MaxReplyTimeout <= 0 {
		config.MaxReplyTimeout = DefaultReplyTimeout
	}
	if config.Log == nil {
		config.Log = log.New(ioutil.Discard, "", 0)
	}

	bot = &MemeBot{
		config:   config,
		channels: make(map[string]*slack.Channel),
	}
	err = bot.dial(authToken)
	return
}

func (b *MemeBot) dial(authToken string) error {
	if b.rtm != nil {
		panic("bot already connected")
	}

	client := slack.New(authToken)
	b.rtm = client.NewRTM()

	go b.rtm.ManageConnection()
	if err := b.waitForConnection(); err != nil {
		return err
	}
	return nil
}

func (b *MemeBot) waitForConnection() error {
	for {
		rawEvent := <-b.rtm.IncomingEvents
		b.config.Log.Println("[slack]", rawEvent.Type)
		switch event := rawEvent.Data.(type) {

		case *slack.ConnectionErrorEvent:
			b.config.Log.Println("[slack]", event.Attempt, "errors connecting:", event)
			if event.Attempt > 3 {
				return ErrConnectionFailed
			}

		case *slack.InvalidAuthEvent:
			return ErrInvalidAuthToken

		case *slack.ConnectedEvent:
			b.slackInfo = event.Info
			for _, ch := range event.Info.Channels {
				b.addChannel(&ch)
			}
			return nil
		}
	}
}

func (b *MemeBot) addChannel(ch *slack.Channel) {
	b.config.Log.Print("[slack] joined channel #", ch.Name)
	b.channels[ch.ID] = ch
}

func (b *MemeBot) removeChannel(id string) {
	if ch, found := b.channels[id]; found {
		b.config.Log.Print("[slack] left channel #", ch.Name)
		delete(b.channels, id)
	}
}

func (b *MemeBot) Name() string {
	return b.slackInfo.User.Name
}

func (b *MemeBot) Run(ctx context.Context) {
	defer b.rtm.Disconnect()

	for {
		select {

		case rawEvent := <-b.rtm.IncomingEvents:
			switch event := rawEvent.Data.(type) {

			case *slack.MessageEvent:
				go b.handleMessage(ctx, (*slack.Message)(event))
			case *slack.ChannelJoinedEvent:
				b.addChannel(&event.Channel)
			case *slack.ChannelLeftEvent:
				b.removeChannel(event.Channel)
			case *slack.RTMError:
				b.config.Log.Println("[slack] RTM error:", rawEvent.Type)
			case *slack.LatencyReport:
				b.config.Log.Println("[slack] current latency:", event.Value)
			}

		case <-ctx.Done():
			b.config.Log.Println("context done, stopping bot...")
			return
		}
	}
}

func (b *MemeBot) handleMessage(ctx context.Context, m *slack.Message) {
	if !(b.config.ParseAllMessages || b.isSelfMentioned(m)) {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, b.config.MaxReplyTimeout)
	defer cancel()

	keyword, matched := b.config.Parser.ParseKeyword(m.Text)
	if !matched {
		if b.isSelfMentioned(m) {
			b.replyTo(ctx, m, b.config.ErrorHandler.OnPhraseNotUnderstood(m.Text))
		}
		return
	}

	meme, err := b.config.Searcher.FindMeme(keyword)
	if err == ErrNoMemeFound {
		if b.isSelfMentioned(m) {
			// Only log if the bot was mentioned to prevent possibly leaking
			// sensitive messages to logs.
			b.config.Log.Println("no meme found for keyword:", keyword)
			b.replyTo(ctx, m, b.config.ErrorHandler.OnNoMemeFound(keyword))
		}
		return
	} else if err != nil {
		if b.isSelfMentioned(m) {
			// Only log if the bot was mentioned to prevent possibly leaking
			// sensitive messages to logs.
			b.config.Log.Printf("error searching for '%s': %s", keyword, err)
			b.replyTo(ctx, m, b.config.ErrorHandler.OnNoMemeFound(keyword))
		}
		return
	}

	b.replyTo(ctx, m, meme.URL().String())
}

func (b *MemeBot) isSelfMentioned(m *slack.Message) bool {
	return strings.HasPrefix(m.Text, "<@"+b.slackInfo.User.ID+">")
}

func (b *MemeBot) replyTo(ctx context.Context, msg *slack.Message, replyText string) {
	select {
	case <-ctx.Done():
		b.config.Log.Print("context done, not sending reply:", ctx.Err(), "\n\t", msg)
	default:
		b.rtm.SendMessage(b.rtm.NewOutgoingMessage(replyText, msg.Channel))
	}
}
