package memebot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type MemeSearcher interface {
	// Returns ErrNoMemeFound if no meme could be found.
	FindMeme(keyword string) (Meme, error)
}

// ErrNoMemeFound is returned from MemeSearcher.FindMeme.
var ErrNoMemeFound = errors.New("no meme found")

type ErrorHandler interface {
	OnNoMemeFound(keyword string) (reply string)
	OnPhraseNotUnderstood(phrase, sample string) (reply string)
	OnHelp(sample string) (reply string)
}

type DefaultErrorHandler struct{}

func (h DefaultErrorHandler) OnNoMemeFound(keyword string) string {
	return fmt.Sprintf("Sorry, I couldn't find a meme for “%s”.", keyword)
}

func (h DefaultErrorHandler) OnPhraseNotUnderstood(phrase, sample string) string {
	return fmt.Sprintf("Sorry, I'm not sure what you mean by:\n> %s\n%s", phrase, h.OnHelp(sample))
}

func (DefaultErrorHandler) OnHelp(sample string) string {
	return fmt.Sprint("Try ", sample)
}

type MemeBotConfig struct {
	Parser   MessageParser
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
	channelsById map[string]*slack.Channel
}

var (
	ErrInvalidAuthToken = errors.New("invalid auth token")
	ErrConnectionFailed = errors.New("failed to connect to slack")
)

const DefaultReplyTimeout = 5 * time.Second

func NewMemeBot(authToken string, config MemeBotConfig) (bot *MemeBot, err error) {
	if config.Searcher == nil {
		err = errors.New("Searcher must be specified")
		return
	}

	// Setup default config values.
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultErrorHandler{}
	}
	if config.MaxReplyTimeout <= 0 {
		config.MaxReplyTimeout = DefaultReplyTimeout
	}
	if config.Log == nil {
		config.Log = log.New(ioutil.Discard, "", 0)
	}

	if err = config.Parser.Validate(); err != nil {
		return
	}

	bot = &MemeBot{
		config:       config,
		channelsById: make(map[string]*slack.Channel),
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
	b.channelsById[ch.ID] = ch
}

func (b *MemeBot) removeChannel(id string) {
	if ch, found := b.channelsById[id]; found {
		b.config.Log.Print("[slack] left channel #", ch.Name)
		delete(b.channelsById, id)
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
	ctx, cancel := context.WithTimeout(ctx, b.config.MaxReplyTimeout)
	defer cancel()

	replyText := handleMessage(b.slackInfo.User, b.config, m)
	if replyText != "" {
		b.replyTo(ctx, m, replyText)
	}
}

func handleMessage(self *slack.UserDetails, config MemeBotConfig, m *slack.Message) string {
	keyword, mentioned, help := config.Parser.ParseMessage(self.Name, self.ID, m.Text)
	sample := config.Parser.KeywordParser.GenerateSample

	if !mentioned && !config.ParseAllMessages {
		return ""
	}

	if help {
		return config.ErrorHandler.OnHelp(sample())
	}

	if keyword == "" {
		if mentioned {
			return config.ErrorHandler.OnPhraseNotUnderstood(m.Text, sample())
		}
		return ""
	}

	meme, err := config.Searcher.FindMeme(keyword)
	if err == ErrNoMemeFound {
		if mentioned {
			// Only log if the bot was mentioned to prevent possibly leaking
			// sensitive messages to logs.
			config.Log.Println("no meme found for keyword:", keyword)
			return config.ErrorHandler.OnNoMemeFound(keyword)
		}
		return ""
	} else if err != nil {
		if mentioned {
			config.Log.Printf("error searching for '%s': %s", keyword, err)
			return config.ErrorHandler.OnNoMemeFound(keyword)
		}
		return ""
	}

	return meme.URL().String()
}

func (b *MemeBot) replyTo(ctx context.Context, msg *slack.Message, replyText string) {
	select {
	case <-ctx.Done():
		b.config.Log.Print("context done, not sending reply:", ctx.Err(), "\n\t", msg)
	default:
		b.rtm.SendMessage(b.rtm.NewOutgoingMessage(replyText, msg.Channel))
	}
}
