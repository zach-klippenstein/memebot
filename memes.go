package memebot

import (
	"net/url"
	"sort"
	"strings"
)

type Meme interface {
	URL() *url.URL
	Keywords() []string
}

type MemeIndex struct {
	byKeyword map[string][]Meme
	all       []Meme
}

func NewMemeIndex() *MemeIndex {
	return &MemeIndex{
		byKeyword: make(map[string][]Meme),
	}
}

func (mi *MemeIndex) Add(meme Meme) {
	mi.all = append(mi.all, meme)
	for _, keyword := range meme.Keywords() {
		keyword = normalizeKeyword(keyword)
		mi.byKeyword[keyword] = append(mi.byKeyword[keyword], meme)
	}
}

// Find performs a case-insensitive search.
func (mi *MemeIndex) FindByKeyword(keyword string) []Meme {
	keyword = normalizeKeyword(keyword)
	return mi.byKeyword[keyword]
}

func (mi *MemeIndex) Len() int {
	return len(mi.all)
}

func (mi *MemeIndex) Memes() []Meme {
	return mi.all
}

func normalizeKeyword(kw string) string {
	return strings.ToLower(kw)
}

func (mi *MemeIndex) Keywords() (keywords []string) {
	for k := range mi.byKeyword {
		keywords = append(keywords, k)
	}
	sort.Sort(sort.StringSlice(keywords))
	return
}
