package memebot

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode"

	regen "github.com/zach-klippenstein/goregen"
)

var DefaultHelpParser = func(msg string) bool {
	return strings.ToLower(msg) == "help"
}

type MessageParser struct {
	KeywordParser KeywordParser

	// Defaults to SlackPrefixMentionParser.
	MentionParser MentionParser

	// Defaults to DefaultHelpParser.
	HelpParser func(string) bool
}

func (p *MessageParser) Validate() error {
	if p.KeywordParser == nil {
		return errors.New("KeywordParser must be specified")
	}
	if p.MentionParser == nil {
		p.MentionParser = SlackPrefixMentionParser{}
	}
	if p.HelpParser == nil {
		p.HelpParser = DefaultHelpParser
	}
	return nil
}

func (p *MessageParser) ParseMessage(mentionedUser, userId, msg string) (keyword string, mentioned bool, help bool) {
	if err := p.Validate(); err != nil {
		panic(err)
	}

	msg, mentioned = p.MentionParser.ParseMention(mentionedUser, userId, msg)

	if p.HelpParser(msg) && mentioned {
		// Only look for help if mentioned.
		help = true
		return
	}

	if kw, matched := p.KeywordParser.ParseKeyword(msg); matched {
		keyword = kw
		return
	}

	return
}

// GenerateSample generates a sample message.
// If userName is non-empty, formats the message with a mention.
func (p *MessageParser) GenerateSample(userName string) string {
	sample := p.KeywordParser.GenerateSample()

	if userName != "" {
		sample = p.MentionParser.FormatMention(userName, sample)
	}

	return sample
}

// If msg contains a mention of mentionedUser (e.g. "mentionedUser foo bar" or "@mentionedUser: foo bar"),
// returns ("foo bar", true). If it doesn't contain the username, returns (msg, false).
type MentionParser interface {
	ParseMention(mentionedUserName, userId, msg string) (cleanMsg string, mentioned bool)

	// FormatMentions a mention to msg as it would be displayed in a Slack client.
	FormatMention(userName, msg string) string
}

type SlackPrefixMentionParser struct{}

func (SlackPrefixMentionParser) ParseMention(mentionedUserName, userId, msg string) (cleanMsg string, mentioned bool) {
	for _, prefix := range []string{mentionedUserName, "<@" + userId + ">"} {
		if mentioned = strings.HasPrefix(msg, prefix); mentioned {
			cleanMsg = strings.TrimPrefix(msg, prefix)

			// Find the end of the current word.
			i := strings.IndexFunc(cleanMsg, unicode.IsSpace)
			if i < 0 {
				// Message contains only a mention, with no other text.
				return
			}
			cleanMsg = cleanMsg[i:]

			cleanMsg = strings.TrimSpace(cleanMsg)
			return
		}
	}

	cleanMsg = msg
	return
}

func (SlackPrefixMentionParser) FormatMention(userName, msg string) string {
	return fmt.Sprintf("@%s %s", userName, msg)
}

type KeywordParser interface {
	// If msg contains a keyword, returns the keyword and true, else empty and false.
	ParseKeyword(msg string) (keyword string, matched bool)

	// Returns an example that shows the syntax accepted by ParseKeyword.
	GenerateSample() string
}

type RegexpKeywordParser struct {
	*regexp.Regexp
	exampleGenerator regen.Generator
}

/*
NewRegexpKeywordParser creates a KeywordParser that parses keywords as the first capture group
in pattern.

keywords is used to generate sample phrases.
*/
func NewRegexpKeywordParser(pattern string, keywords []string) (parser RegexpKeywordParser, err error) {
	// Make the regexp case-insensitive.
	compiledPattern, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return
	}

	if compiledPattern.NumSubexp() < 1 {
		err = fmt.Errorf("keyword pattern must have at least 1 capturing group: /%s/", pattern)
		return
	}

	// Setup the sample generator.
	generator, err := regen.NewGenerator(pattern, &regen.GeneratorArgs{
		Flags: syntax.Perl, // regexp.Compile uses this flag too.
		MinUnboundedRepeatCount: 5,
		MaxUnboundedRepeatCount: 5,
		CaptureGroupHandler: func(index int, name string, group *syntax.Regexp, generator regen.Generator, args *regen.GeneratorArgs) string {
			// Only use a keyword for the first capture group.
			if index != 0 || len(keywords) == 0 {
				return generator.Generate()
			}

			keywordIndex := args.Rng().Intn(len(keywords))
			return keywords[keywordIndex]
		}})
	if err != nil {
		err = errors.New("error creating sample phrase generator: " + err.Error())
		return
	}

	parser = RegexpKeywordParser{
		Regexp:           compiledPattern,
		exampleGenerator: generator,
	}
	return
}

func (p RegexpKeywordParser) ParseKeyword(msg string) (string, bool) {
	matches := p.FindStringSubmatch(msg)
	if len(matches) < 2 {
		return "", false
	}
	// Ignore the full-string match.
	matches = matches[1:]

	// Find the first non-empty capture group value (submatch).
	// First element is the entire matched string, we only care about capture groups.
	for _, match := range matches {
		if match != "" {
			return match, true
		}
	}

	return "", false
}

func (p RegexpKeywordParser) GenerateSample() string {
	return p.exampleGenerator.Generate()
}
