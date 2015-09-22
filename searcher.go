package memebot

import "math/rand"

type MemepositorySearcher struct {
	Memepository
}

var _ MemeSearcher = &MemepositorySearcher{}

func (s *MemepositorySearcher) FindMeme(keyword string) (Meme, error) {
	memes, err := s.Load()
	if err != nil {
		return nil, err
	}

	results := memes.FindByKeyword(keyword)
	if len(results) == 0 {
		return nil, ErrNoMemeFound
	}

	index := rand.Intn(len(results))
	return results[index], nil
}
