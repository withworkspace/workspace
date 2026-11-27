package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func picture(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, c)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestSaveKeepsOneOfEachPicture(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "images")
	red := picture(t, color.RGBA{R: 255, A: 255})
	a, err := Save(dir, red)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type != "image/png" || !strings.HasSuffix(a.Name, ".png") || a.Path != filepath.Join(dir, a.Name) {
		t.Fatalf("saved as %+v", a)
	}
	again, err := Save(dir, red)
	if err != nil || again != a {
		t.Fatalf("the same picture again: %+v, %v; want %+v", again, err, a)
	}
	blue, err := Save(dir, picture(t, color.RGBA{B: 255, A: 255}))
	if err != nil || blue.Name == a.Name {
		t.Fatalf("another picture: %+v, %v", blue, err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 2 {
		t.Fatalf("%d files kept, want 2 (nothing half written left)", len(files))
	}
}

func TestSaveRefusesWhatIsntAPicture(t *testing.T) {
	dir := t.TempDir()
	for _, data := range [][]byte{nil, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), []byte("hello")} {
		if _, err := Save(dir, data); !errors.Is(err, ErrNotPicture) {
			t.Errorf("Save(%q) = %v, want ErrNotPicture", data, err)
		}
	}
	big := append(picture(t, color.Black), make([]byte, Most)...)
	if _, err := Save(dir, big); !errors.Is(err, ErrTooBig) {
		t.Errorf("a picture over Most: %v", err)
	}
}

func TestFindOnlyByAPicturesName(t *testing.T) {
	dir := t.TempDir()
	img, err := Save(dir, picture(t, color.White))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := Find(dir, img.Name); !ok || got != img {
		t.Fatalf("Find(%s) = %+v, %v", img.Name, got, ok)
	}
	for _, name := range []string{"../ws.db", "x.png", strings.Repeat("0", 32) + ".png", img.Name + "/..", ""} {
		if _, ok := Find(dir, name); ok {
			t.Errorf("Find(%q) found one", name)
		}
	}
}

func TestInFindsThePicturesWritten(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my workspace")
	dir := Dir(root)
	a, _ := Save(dir, picture(t, color.White))
	b, _ := Save(dir, picture(t, color.Black))
	gone := strings.Repeat("a", 32) + ".png"
	text := "Here: ![image](<" + a.Path + ">) and\n![shot](/elsewhere/.ws/images/" + b.Name + ")" +
		"\nagain ![image](<" + a.Path + ">), ![](" + filepath.Join(dir, gone) + "), and a path alone " + b.Path
	got := In(dir, text)
	if len(got) != 2 || got[0] != a || got[1].Name != b.Name || got[1].Path != b.Path {
		t.Fatalf("In = %+v, want %s then %s", got, a.Name, b.Name)
	}
	if In(dir, "no pictures") != nil {
		t.Error("found pictures in words")
	}
}

func TestNote(t *testing.T) {
	if Note(nil) != "" {
		t.Error("a note for no pictures")
	}
	one := Note([]Image{{Path: "/w/.ws/images/a.png"}})
	if !strings.Contains(one, "a picture") || !strings.Contains(one, "/w/.ws/images/a.png") {
		t.Errorf("one: %q", one)
	}
	two := Note([]Image{{Path: "/a.png"}, {Path: "/b.png"}})
	if !strings.Contains(two, "2 pictures") || !strings.Contains(two, "/a.png, /b.png") {
		t.Errorf("two: %q", two)
	}
}
