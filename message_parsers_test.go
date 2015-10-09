package memebot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageParser(t *testing.T) {
	for _, test := range []struct {
		pattern   string
		msg       string
		kw        string
		mentioned bool
		help      bool
	}{
		{
			pattern: `^(\w+)$`,
			msg:     "",
		},
		{
			// No mention, so this is treated as a regular parse.
			pattern: `^(\w+)$`,
			msg:     "help",
			kw:      "help",
			help:    false,
		},
		{
			pattern: `^meme (\w+)$`,
			msg:     "meme help",
			kw:      "help",
			help:    false,
		},
		{
			pattern:   `^(\w+)$`,
			msg:       "name help",
			mentioned: true,
			help:      true,
		},
		{
			pattern:   `^(\w+)$`,
			msg:       "<@id> help",
			mentioned: true,
			help:      true,
		},
		{
			pattern:   `^(\w+)$`,
			msg:       "name please help",
			mentioned: true,
		},
		{
			pattern: `(\w+)`,
			msg:     "kw",
			kw:      "kw",
		},
		{
			pattern: `(\w+)`,
			msg:     "kw foo bar",
			kw:      "kw",
		},
		{
			pattern: `(kw)`,
			msg:     "foo kw bar",
			kw:      "kw",
		},
		{
			pattern:   `(\w+)`,
			msg:       "name kw",
			kw:        "kw",
			mentioned: true,
		},
		{
			pattern:   `(\w+)`,
			msg:       "<@id>: kw",
			kw:        "kw",
			mentioned: true,
		},
		{
			pattern:   `(\w+)`,
			msg:       "name name",
			kw:        "name",
			mentioned: true,
		},
	} {
		kwParser, err := NewRegexpKeywordParser(test.pattern)
		require.NoError(t, err)
		parser := MessageParser{KeywordParser: kwParser}

		kw, mentioned, help := parser.ParseMessage("name", "id", test.msg)
		assert.Equal(t, test.kw, kw, "keyword %s for %+v", kw, test)
		assert.Equal(t, test.mentioned, mentioned, "mentioned %s for %+v", mentioned, test)
		assert.Equal(t, test.help, help, "help %s for %+v", help, test)
	}
}

func TestSlackPrefixMentionParser_Name(t *testing.T) {
	clean, mentioned := SlackPrefixMentionParser("name", "id", "name baz")
	assert.True(t, mentioned)
	assert.Equal(t, "baz", clean)

	clean, mentioned = SlackPrefixMentionParser("name", "id", "name: baz")
	assert.True(t, mentioned)
	assert.Equal(t, "baz", clean)

	clean, mentioned = SlackPrefixMentionParser("name", "id", "<@name>: baz")
	assert.False(t, mentioned)
	assert.Equal(t, "<@name>: baz", clean)
}

func TestSlackPrefixMentionParser_MentionOnly(t *testing.T) {
	clean, mentioned := SlackPrefixMentionParser("name", "", "name")
	assert.True(t, mentioned)
	assert.Equal(t, "", clean)

	clean, mentioned = SlackPrefixMentionParser("name", "", "name: ")
	assert.True(t, mentioned)
	assert.Equal(t, "", clean)
}

func TestSlackPrefixMention_ParserId(t *testing.T) {
	clean, mentioned := SlackPrefixMentionParser("name", "id", "<@id>: baz")
	assert.True(t, mentioned)
	assert.Equal(t, "baz", clean)

	clean, mentioned = SlackPrefixMentionParser("name", "id", "id baz")
	assert.False(t, mentioned)
	assert.Equal(t, "id baz", clean)
}

func TestNewRegexpKeywordParser(t *testing.T) {
	parser, err := NewRegexpKeywordParser(`(hello) world`)
	assert.NoError(t, err)
	assert.NotNil(t, parser.Regexp)

	parser, err = NewRegexpKeywordParser(`(hello) (world)`)
	assert.NoError(t, err)
	assert.NotNil(t, parser.Regexp)
}

func TestNewRegexpKeywordParser_RequiresCaptureGroup(t *testing.T) {
	_, err := NewRegexpKeywordParser(`hello world`)
	assert.EqualError(t, err, "keyword pattern must have at least 1 capturing group: /hello world/")
}

func TestRegexpKeywordParser(t *testing.T) {
	parser, _ := NewRegexpKeywordParser(`a (\w+) (b)`)

	// Happy cases.
	for _, msg := range []string{
		"a hello b",
		"fooa hello bbaz",
	} {
		kw, match := parser.ParseKeyword(msg)
		assert.True(t, match)
		assert.Equal(t, "hello", kw)
	}

	// Sad cases.
	for _, msg := range []string{
		"hello",
		"hello b",
		"a hello",
	} {
		_, match := parser.ParseKeyword(msg)
		assert.False(t, match)
	}
}
