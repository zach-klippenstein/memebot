package memebot

import (
	"net/url"
	"os"
	"time"

	"github.com/stretchr/testify/mock"
)

type MockSearcher struct {
	mock.Mock
}

func (m *MockSearcher) FindMeme(keyword string) (Meme, error) {
	args := m.Called(keyword)

	if meme, ok := args.Get(0).(Meme); ok {
		return meme, args.Error(1)
	}
	return nil, args.Error(1)
}

type MockMemepository struct {
	index *MemeIndex
}

func (m *MockMemepository) Load() (*MemeIndex, error) {
	return m.index, nil
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

type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) ReadDirEntries(path string) ([]os.FileInfo, error) {
	args := m.Called(path)
	return args.Get(0).([]os.FileInfo), args.Error(1)
}

func (m *MockFileSystem) Open(name string) (ReadSeekerCloser, error) {
	args := m.Called(name)
	return args.Get(0).(ReadSeekerCloser), args.Error(1)
}

type MockFileInfo struct {
	name    string
	modTime time.Time
}

func (m MockFileInfo) Name() string {
	return m.name
}

func (m MockFileInfo) Size() int64 {
	return 0
}

func (m MockFileInfo) Mode() os.FileMode {
	return 0
}

func (m MockFileInfo) ModTime() time.Time {
	return m.modTime
}

func (m MockFileInfo) IsDir() bool {
	return false
}

func (m MockFileInfo) Sys() interface{} {
	return nil
}
