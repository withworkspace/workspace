package ops

import (
	"errors"

	"github.com/withworkspace/workspace/internal/images"
)

// SaveImage keeps a picture you pasted for an agent (internal/images),
// for what you're writing to name by its path.
func (w *Workspace) SaveImage(data []byte) (images.Image, error) {
	img, err := images.Save(images.Dir(w.Root()), data)
	if errors.Is(err, images.ErrNotPicture) || errors.Is(err, images.ErrTooBig) {
		return images.Image{}, invalid("%s", err)
	}
	return img, err
}

// Image is a picture pasted before, by its name, to be drawn.
func (w *Workspace) Image(name string) (images.Image, error) {
	img, ok := images.Find(images.Dir(w.Root()), name)
	if !ok {
		return images.Image{}, ErrNotFound
	}
	return img, nil
}
