package memebot

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type StringSet map[string]struct{}

func MakeSet(vals ...string) (set StringSet) {
	set = make(StringSet)
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return
}

func (s StringSet) Apply(f func(string) string) (result StringSet) {
	result = make(StringSet)
	for k := range s {
		result[f(k)] = struct{}{}
	}
	return
}

type Memepository interface {
	// Load should be safe to call from multiple goroutines.
	Load() (*MemeIndex, error)
}

type FileSystem interface {
	ReadDirEntries(path string) ([]os.FileInfo, error)
	Open(name string) (*os.File, error)
}

type FileServingMemepositoryConfig struct {
	Path            string      // Path to images directory.
	ImageExtensions StringSet   // Extensions to recognize as image files.
	Router          *mux.Router // Root router to serve image IDs from.

	FileSystem FileSystem // Injectable os wrapper for testing. Zero value delegates to os.
}

type defaultFileSystem struct{}

func (defaultFileSystem) ReadDirEntries(path string) ([]os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// -1 to read all entries.
	return file.Readdir(-1)
}

func (defaultFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// FileServingMemepository is a Memepository that loads images stored on disk,
// and serves them from an HTTP server.
type FileServingMemepository struct {
	FileServingMemepositoryConfig

	server *ObjectServer

	// Used to load memes only the first time Load is called.
	loadOnce  sync.Once
	memes     *MemeIndex
	memesById map[string]*FileMeme
	loadErr   error
}

var _ ObjectRepository = &FileServingMemepository{}

func NewFileServingMemepository(config FileServingMemepositoryConfig) *FileServingMemepository {
	// Convert all extensions to lowercase for matching.
	config.ImageExtensions = config.ImageExtensions.Apply(strings.ToLower)

	if config.FileSystem == nil {
		config.FileSystem = defaultFileSystem{}
	}

	memepository := &FileServingMemepository{
		FileServingMemepositoryConfig: config,
	}
	memepository.server = CreateObjectServer(config.Router, memepository)

	return memepository
}

func (m *FileServingMemepository) Load() (memes *MemeIndex, err error) {
	m.loadOnce.Do(m.load)
	return m.memes, m.loadErr
}

func (m *FileServingMemepository) FindObject(id string) (Object, bool) {
	if _, err := m.Load(); err != nil {
		return nil, false
	}
	meme, found := m.memesById[id]
	return meme, found
}

func (m *FileServingMemepository) load() {
	log.Println("loading memes from", m.Path)

	entries, err := m.FileSystem.ReadDirEntries(m.Path)
	if err != nil {
		log.Println("error reading directory:", err)
		m.loadErr = err
		return
	}

	m.memes = NewMemeIndex()
	m.memesById = make(map[string]*FileMeme)

	for _, entry := range entries {
		if m.isImageFile(entry) {
			meme, err := newFileMeme(entry, m)
			if err != nil {
				log.Println("couldn't load", entry.Name(), err)
			} else {
				m.memes.Add(meme)
				m.memesById[meme.id] = meme
			}
		}
	}

	log.Println("loaded", m.memes.Len(), "memes")
}

func (m *FileServingMemepository) isImageFile(file os.FileInfo) bool {
	if (file.Mode() & os.ModeType) != 0 {
		// Not a regular file.
		return false
	}

	extension := getNormalizedExtensionWithoutDot(file.Name())
	_, found := m.ImageExtensions[extension]
	return found
}

type FileMeme struct {
	owner        *FileServingMemepository
	id           string
	path         string
	lastModified time.Time
	size         int64
	keywords     []string
}

var _ Object = &FileMeme{}

func newFileMeme(file os.FileInfo, owner *FileServingMemepository) (*FileMeme, error) {
	path := filepath.Join(owner.Path, file.Name())

	id, err := generateHashForFile(owner.FileSystem, path)
	if err != nil {
		return nil, err
	}
	// Append the extension to the ID for content-type detection
	id = id + "." + getNormalizedExtensionWithoutDot(file.Name())

	return &FileMeme{
		owner:        owner,
		id:           id,
		path:         path,
		lastModified: file.ModTime(),
		size:         file.Size(),
		keywords:     parseKeywords(file.Name()),
	}, nil
}

func parseKeywords(name string) []string {
	extension := filepath.Ext(name)
	nameWithoutExtension := strings.TrimSuffix(name, extension)
	return strings.Split(nameWithoutExtension, ",")
}

func (m *FileMeme) URL() *url.URL {
	return m.owner.server.URL(m.id)
}

func (m *FileMeme) Keywords() []string {
	return m.keywords
}

func (m *FileMeme) Open() (ReadSeekerCloser, error) {
	return m.owner.FileSystem.Open(m.path)
}

func (m *FileMeme) LastModified() time.Time {
	return m.lastModified
}

func (m *FileMeme) Size() int64 {
	return m.size
}

func getNormalizedExtensionWithoutDot(name string) (extension string) {
	extension = filepath.Ext(name)
	extension = strings.TrimPrefix(extension, ".")
	extension = strings.ToLower(extension)
	return
}

func generateHashForFile(fs FileSystem, name string) (string, error) {
	file, err := fs.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()

	return generateSha1Base64Hash(file)
}

func generateSha1Base64Hash(r io.Reader) (string, error) {
	hasher := sha1.New()
	_, err := io.Copy(hasher, r)
	if err != nil {
		return "", err
	}
	hash := hasher.Sum(nil)

	// Encode as base64.
	var asString bytes.Buffer
	encoder := base64.NewEncoder(base64.URLEncoding, &asString)
	encoder.Write(hash)
	encoder.Close()
	return asString.String(), nil
}
