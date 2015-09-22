package memebot

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStringSet(t *testing.T) {
	set := MakeSet("a", "b")
	assert.True(t, set.Contains("a"))
	assert.True(t, set.Contains("b"))
	assert.False(t, set.Contains("c"))
}

func TestStringSetApply(t *testing.T) {
	set := MakeSet("a", "b").Apply(strings.ToUpper)
	assert.True(t, set.Contains("A"))
	assert.True(t, set.Contains("B"))
	assert.False(t, set.Contains("a"))
	assert.False(t, set.Contains("b"))
}

func TestParseKeywords(t *testing.T) {
	kw := parseKeywords("foo bar, foobar ,  ,,.jpg")
	assert.Equal(t, []string{"foo bar", "foobar"}, kw)
}

func TestGetNormalizedExtensionWithoutDot(t *testing.T) {
	ext := getNormalizedExtensionWithoutDot("foo.BAr")
	assert.Equal(t, "bar", ext)
}

func TestGenerateHashForFile(t *testing.T) {
	data := Buffer{bytes.NewBufferString("hello world")}

	fs := new(MockFileSystem)
	fs.On("Open", "foo").Return(data, nil)

	hash, err := generateHashForFile(fs, "foo")
	assert.NoError(t, err)
	assert.True(t, len(hash) > 0)
}

type Buffer struct {
	*bytes.Buffer
}

func (b Buffer) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (b Buffer) Close() error {
	return nil
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
