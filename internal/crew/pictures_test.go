package crew

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/images"
)

type pasted struct{ text []string }

func (p *pasted) RenderScrolled(int) string { return "" }
func (p *pasted) SendKey(uv.KeyPressEvent)  {}
func (p *pasted) Paste(s string)            { p.text = append(p.text, s) }

// A message to crew with a picture pasted in tells them to read it, as a
// conversation's agent is told; one with none is as it was written.
func TestCrewToldOfPicturesPasted(t *testing.T) {
	root := t.TempDir()
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	img, err := images.Save(images.Dir(root), pic.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	said := "This button's off: ![image](<" + img.Path + ">)"
	k := &pasted{}
	r := TermRun{S: Spec{Slug: "m", CrewID: "c1", Harness: "claude"}, T: k, Root: root}
	if _, err := r.Say(said); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Say("carry on"); err != nil {
		t.Fatal(err)
	}
	if len(k.text) != 2 || !strings.HasPrefix(k.text[0], said) || !strings.Contains(k.text[0], "read it with your tool for reading files to see it: "+img.Path) || k.text[1] != "carry on" {
		t.Errorf("pasted: %q", k.text)
	}

	// Resumed with it, over ACP: told, and given the picture.
	text, pics := withPictures(root, said)
	if !strings.Contains(text, img.Path+")") || len(pics) != 1 || pics[0].Path != img.Path || pics[0].Type != "image/png" {
		t.Errorf("withPictures = %q, %+v", text, pics)
	}
}
