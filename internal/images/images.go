// Package images keeps the pictures you paste for an agent: a screenshot
// in a conversation, a crew's transcript, anywhere you write to one. Each
// is a file in the workspace's .ws/images, named for what's in it, so
// pasting the same picture twice keeps one, and what you wrote names it
// by its path in full: the agent is given what you wrote, and reads the
// picture from there, as it would any file.
//
// They're kept with the workspace's records, not in a temporary folder,
// so a conversation read later (or an agent's session taken up again)
// still has its pictures.
package images

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Most is the largest picture kept: a screenshot of a large screen at
// full resolution is a few megabytes.
const Most = 20 << 20

// ErrNotPicture is something pasted that isn't a picture an agent can see.
var ErrNotPicture = errors.New("not a picture: paste a PNG, JPEG, GIF or WebP")

// ErrTooBig is a picture over Most.
var ErrTooBig = fmt.Errorf("too large: maximum %d MB", Most>>20)

// kinds are the pictures an agent's model reads, and what each is saved as.
var kinds = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// named is a picture's name: what's in it, and what it is.
var named = regexp.MustCompile(`^[0-9a-f]{32}\.(?:png|jpg|gif|webp)$`)

// written is a picture where it's named in what someone wrote: a markdown
// image whose address ends in a picture's place (.ws/images/<name>),
// perhaps in <> (a path with a space in it).
var written = regexp.MustCompile(`!\[[^\]]*\]\(<?[^)>]*?[/\\]\.ws[/\\]images[/\\]([0-9a-f]{32}\.(?:png|jpg|gif|webp))>?\)`)

// Dir is where a workspace keeps them.
func Dir(root string) string { return filepath.Join(root, ".ws", "images") }

// Image is a picture as it's kept.
type Image struct {
	Name string `json:"name"` // its file's name in Dir
	Path string `json:"path"` // its file, in full
	Type string `json:"type"` // image/png, …
}

// Save keeps data in dir, if it's a picture, and says where.
func Save(dir string, data []byte) (Image, error) {
	if len(data) == 0 {
		return Image{}, ErrNotPicture
	}
	if len(data) > Most {
		return Image{}, ErrTooBig
	}
	kind := http.DetectContentType(data)
	ext, ok := kinds[kind]
	if !ok {
		return Image{}, ErrNotPicture
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:16]) + ext
	img := Image{Name: name, Path: filepath.Join(dir, name), Type: kind}
	if _, err := os.Stat(img.Path); err == nil {
		return img, nil // pasted before: what's in it is the same
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Image{}, err
	}
	// Written whole before it's there by its name, so it's never read
	// half written.
	tmp, err := os.CreateTemp(dir, ".paste-*")
	if err != nil {
		return Image{}, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return Image{}, err
	}
	if err := tmp.Close(); err != nil {
		return Image{}, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return Image{}, err
	}
	if err := os.Rename(tmp.Name(), img.Path); err != nil {
		return Image{}, err
	}
	return img, nil
}

// Find is the picture kept in dir by that name, if there's one: a name
// that isn't one of theirs (a path, say) never is.
func Find(dir, name string) (Image, bool) {
	if !named.MatchString(name) {
		return Image{}, false
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return Image{}, false
	}
	return Image{Name: name, Path: path, Type: typeOf(name)}, true
}

func typeOf(name string) string {
	ext := filepath.Ext(name)
	for kind, e := range kinds {
		if e == ext {
			return kind
		}
	}
	return ""
}

// In are the pictures kept in dir that text names, each once, in the
// order it names them. A picture named by a path from elsewhere (another
// workspace's) is found by its name here, or not at all.
func In(dir, text string) []Image {
	if !strings.Contains(text, "images") {
		return nil
	}
	var out []Image
	seen := map[string]bool{}
	for _, m := range written.FindAllStringSubmatch(text, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		if img, ok := Find(dir, m[1]); ok {
			out = append(out, img)
		}
	}
	return out
}

// Note is what an agent's told of the pictures in what it's given: that
// each is a file it reads to see. "" for none.
func Note(imgs []Image) string {
	if len(imgs) == 0 {
		return ""
	}
	var b strings.Builder
	if len(imgs) == 1 {
		b.WriteString("\n\n(You were given a picture, pasted as a file: read it with your tool for reading files to see it: ")
	} else {
		fmt.Fprintf(&b, "\n\n(You were given %d pictures, pasted as files: read each with your tool for reading files to see it: ", len(imgs))
	}
	for i, img := range imgs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(img.Path)
	}
	b.WriteString(")")
	return b.String()
}
