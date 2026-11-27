package ops

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/images"
	"github.com/withworkspace/workspace/internal/spend"
)

// heard is a Converser that keeps what it was asked to run, and runs nothing.
type heard struct{ run *harness.ConverseRun }

func (heard) Name() string     { return "heard" }
func (heard) Available() error { return nil }
func (heard) Start(harness.Brief) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (heard) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (heard) ConverseModel() string { return "" }
func (h heard) Converse(r harness.ConverseRun) (harness.Launch, error) {
	*h.run = r
	return harness.Launch{}, errors.New("heard")
}

// A picture you pasted into what an agent's sent is a file it's told to
// read, and given (where its harness can take one).
func TestAgentToldOfPicturesPasted(t *testing.T) {
	root := t.TempDir()
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	img, err := images.Save(images.Dir(root), pic.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	said := "What's wrong here? ![image](<" + img.Path + ">)"
	var got harness.ConverseRun
	_, _ = RunHeadless(context.Background(), root, "test", spend.For{}, nil, "", 0, heard{&got}, harness.ConverseRun{Prompt: said})
	if !strings.HasPrefix(got.Prompt, said) || !strings.Contains(got.Prompt, "read it with your tool for reading files to see it: "+img.Path) {
		t.Errorf("prompt: %q", got.Prompt)
	}
	if len(got.Images) != 1 || got.Images[0].Path != img.Path || got.Images[0].Type != "image/png" {
		t.Errorf("images: %+v", got.Images)
	}

	// A run with no tools can't read one: it's told nothing of it.
	_, _ = RunHeadless(context.Background(), root, "test", spend.For{}, nil, "", 0, heard{&got}, harness.ConverseRun{Prompt: said, Alone: true})
	if got.Prompt != said || got.Images != nil {
		t.Errorf("alone: %q, %+v", got.Prompt, got.Images)
	}
}
