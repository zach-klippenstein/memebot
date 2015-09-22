package memebot

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemes(t *testing.T) {
	foo := NewMockMeme("http://foo.com", "foo", "bar")
	bar := NewMockMeme("http://bar.com", "bar", "baz")
	memes := NewMemeIndex()
	memes.Add(foo)
	memes.Add(bar)

	assert.Equal(t, 2, memes.Len())
	assert.Equal(t, []string{"bar", "baz", "foo"}, memes.Keywords())
	assert.Equal(t, []Meme{foo}, memes.FindByKeyword("foo"))
	assert.Len(t, memes.FindByKeyword("bar"), 2)
}

func NewMockMeme(url string, keywords ...string) Meme {
	return MockMeme{mustParseURL(url), keywords}
}

type MockMeme struct {
	url      *url.URL
	keywords []string
}

func (m MockMeme) URL() *url.URL {
	return m.url
}

func (m MockMeme) Keywords() []string {
	return m.keywords
}

func NewTestMemeIndex(memes ...Meme) *MemeIndex {
	index := NewMemeIndex()
	for _, meme := range memes {
		index.Add(meme)
	}
	return index
}

func mustParseURL(rawurl string) *url.URL {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return u
}
