package memebot

import (
	"testing"

	"github.com/nlopes/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleMessage_ParseAllMessages_NoMention(t *testing.T) {
	meme := NewMockMeme("http://keyword.jpg")

	searcher, user, config, msg := CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, true, "do keyword")
	searcher.On("FindMeme", "keyword").Return(meme, nil)
	reply := handleMessage(user, config, msg)
	assert.Equal(t, "http://keyword.jpg", reply)

	searcher, user, config, msg = CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, true, "do keyword")
	searcher.On("FindMeme", "keyword").Return(nil, ErrNoMemeFound)
	reply = handleMessage(user, config, msg)
	// No mention, don't reply with an error.
	assert.Equal(t, "", reply)

	searcher, user, config, msg = CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, true, "keyword")
	searcher.On("FindMeme", "keyword").Return(meme, nil)
	reply = handleMessage(user, config, msg)
	assert.Equal(t, "", reply)
}

func TestHandleMessage_ParseAllMessages_Mention(t *testing.T) {
	meme := NewMockMeme("http://keyword.jpg")

	searcher, user, config, msg := CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, true, "name do keyword")
	searcher.On("FindMeme", "keyword").Return(meme, nil)
	reply := handleMessage(user, config, msg)
	assert.Equal(t, "http://keyword.jpg", reply)

	searcher, user, config, msg = CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, true, "name do keyword")
	searcher.On("FindMeme", "keyword").Return(nil, ErrNoMemeFound)
	reply = handleMessage(user, config, msg)
	assert.Equal(t, "Sorry, I couldn't find a meme for “keyword”.", reply)

	// Sample without mention.
	searcher, user, config, msg = CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{"keyword"}, true, "name keyword")
	reply = handleMessage(user, config, msg)
	assert.Equal(t, `Sorry, I'm not sure what you mean by:
> name keyword
Try something like “do keyword”`, reply)

	// Sample with mention.
	searcher, user, config, msg = CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{"keyword"}, false, "name keyword")
	reply = handleMessage(user, config, msg)
	assert.Equal(t, `Sorry, I'm not sure what you mean by:
> name keyword
Try something like “@name do keyword”`, reply)
}

func TestHandleMessage_RequireMention(t *testing.T) {
	searcher, user, config, msg := CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, false, "name do keyword")
	meme := NewMockMeme("http://keyword.jpg")
	searcher.On("FindMeme", "keyword").Return(meme, nil)
	reply := handleMessage(user, config, msg)
	assert.Equal(t, "http://keyword.jpg", reply)

	searcher, user, config, msg = CreateArgsForHandleMessage(t, `^do (\w+)$`, []string{}, false, "do keyword")
	meme = NewMockMeme("http://keyword.jpg")
	searcher.On("FindMeme", "keyword").Return(meme, nil)
	reply = handleMessage(user, config, msg)
	assert.Equal(t, "", reply)
}

func CreateArgsForHandleMessage(t *testing.T, keywordPattern string, keywords []string, parseAllMessages bool, msgText string) (searcher *MockSearcher, user *slack.UserDetails, config MemeBotConfig, msg *slack.Message) {
	parser, err := NewRegexpKeywordParser(keywordPattern, keywords)
	require.NoError(t, err)

	searcher = new(MockSearcher)

	config = MemeBotConfig{
		Parser: MessageParser{
			KeywordParser: parser,
		},
		ParseAllMessages: parseAllMessages,
		Searcher:         searcher,
	}
	config.Validate()

	user = &slack.UserDetails{
		Name: "name",
		ID:   "id",
	}
	msg = &slack.Message{
		Msg: slack.Msg{
			Text: msgText,
		},
	}
	return
}
