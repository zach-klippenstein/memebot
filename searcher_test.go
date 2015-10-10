package memebot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemepositorySearcher(t *testing.T) {
	mp := &MockMemepository{NewTestMemeIndex(
		NewMockMeme("http://foo.com", "foo", "bar"),
	)}
	searcher := &MemepositorySearcher{mp}

	meme, err := searcher.FindMeme("foo")
	assert.NoError(t, err)
	assert.Equal(t, "foo.com", meme.URL().Host)

	meme, err = searcher.FindMeme("bar")
	assert.NoError(t, err)
	assert.Equal(t, "foo.com", meme.URL().Host)

	meme, err = searcher.FindMeme("baz")
	assert.Equal(t, ErrNoMemeFound, err)
}

func TestMemepositorySearcherPicksRandomMeme(t *testing.T) {
	mp := &MockMemepository{NewTestMemeIndex(
		NewMockMeme("http://foo.com", "foo"),
		NewMockMeme("http://bar.com", "foo"),
	)}
	searcher := &MemepositorySearcher{mp}

	fooCount := 0
	barCount := 0
	for i := 0; i < 100; i++ {
		meme, err := searcher.FindMeme("foo")
		if err != nil {
			panic(err)
		}
		if meme.URL().Host == "foo.com" {
			fooCount++
		} else {
			barCount++
		}
	}

	assert.True(t, fooCount > 0)
	assert.True(t, barCount > 0)
}
