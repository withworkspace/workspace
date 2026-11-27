package crew

import (
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/images"
)

// withPictures is a message to crew as they're given it: told to read each
// picture pasted into it (internal/images), and the pictures, for crew
// whose agent takes them with a message (over ACP).
func withPictures(root, text string) (string, []harness.Image) {
	pics := images.In(images.Dir(root), text)
	var out []harness.Image
	for _, p := range pics {
		out = append(out, harness.Image{Path: p.Path, Type: p.Type})
	}
	return text + images.Note(pics), out
}
