package memebot

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
)

var ErrObjectIdNotFound = errors.New("object id not found")

type ReadSeekerCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Object interface {
	Open() (ReadSeekerCloser, error)
	LastModified() time.Time
	Size() int64
}

type ObjectRepository interface {
	FindObject(id string) (Object, bool)
}

type ObjectServer struct {
	repository ObjectRepository
	route      *mux.Route
}

func CreateObjectServer(router *mux.Router, repository ObjectRepository) *ObjectServer {
	server := &ObjectServer{
		repository: repository,
	}
	server.route = router.Path("/{id}").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		id := vars["id"]

		if id == "" {
			log.Println("bad request: no ID specified")
			http.Error(w, "no ID specified", http.StatusBadRequest)
			return
		}

		object, found := repository.FindObject(id)
		if !found {
			err := fmt.Errorf("id not found: %s", id)
			log.Printf(err.Error())
			http.NotFound(w, req)
			return
		}

		data, err := object.Open()
		if err != nil {
			err := fmt.Sprintf("error opening object %s: %s", id, err)
			log.Printf(err)
			http.Error(w, err, http.StatusInternalServerError)
			return
		}
		defer data.Close()
		log.Printf("loaded object id: %s (%d bytes)", id, object.Size())

		http.ServeContent(w, req, id, object.LastModified(), data)
	})
	return server
}

func (s *ObjectServer) URL(id string) *url.URL {
	url, err := s.route.URL("id", id)
	if err != nil {
		panic(fmt.Errorf("error creating URL for id %s: %s", id, err))
	}
	return url
}
