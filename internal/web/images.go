package web

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/withworkspace/workspace/internal/images"
)

// Picturing is the pictures you paste for an agent (internal/images):
// one kept as you paste it, and one read back to be drawn.
type Picturing interface {
	SaveImage(data []byte) (images.Image, error)
	Image(name string) (images.Image, error)
}

// Pasted is a picture kept: its path, which what you write names it by
// (the agent reads it there), and where the web UI draws it from.
type Pasted struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

// imageURL is where the web UI reads a pasted picture.
func imageURL(name string) string { return "/api/images/" + name }

// Paste is a picture to keep, in JSON as every write is (the cross-site
// guard): its bytes, base64.
type Paste struct {
	Data []byte `json:"data"`
}

// saveImage keeps the picture pasted.
func (s *Server) saveImage(w http.ResponseWriter, r *http.Request) {
	var in Paste
	// Base64 is a third larger than what it says.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, images.Most/3*4+1024)).Decode(&in); err != nil {
		var big *http.MaxBytesError
		if errors.As(err, &big) {
			jsonError(w, http.StatusRequestEntityTooLarge, images.ErrTooBig.Error())
			return
		}
		jsonError(w, http.StatusBadRequest, "send a picture as {\"data\": base64}")
		return
	}
	img, err := s.App.SaveImage(in.Data)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, Pasted{Name: img.Name, Path: img.Path, URL: imageURL(img.Name)})
}

// image is a pasted picture, to draw. What's in it is its name, so it's
// kept as long as a browser likes.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	img, err := s.App.Image(r.PathValue("name"))
	if err != nil {
		writeError(w, err, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", img.Type)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, img.Path)
}
