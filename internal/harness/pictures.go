package harness

// pictureReader is a harness whose crew are let read pasted pictures
// (internal/images) without asking: crew are interactive, and a read
// outside their capsule otherwise waits on your yes in their terminal.
type pictureReader interface {
	readsPictures(l Launch, dir string) Launch
}

// ReadsPictures lets crew's run read the pictures in dir, where its
// harness asks before reading outside where it works.
func ReadsPictures(h Harness, l Launch, dir string) Launch {
	if r, ok := h.(pictureReader); ok && dir != "" && len(l.Args) > 0 {
		return r.readsPictures(l, dir)
	}
	return l
}

// VERIFIED (claude 2.1.332, -p): Read(//<path>/**) in --allowedTools lets
// it read a file under <path> (a path in full is written after a /);
// with none, reading one outside the folder it runs in is refused.
// --allowedTools takes the words after it up to the next option, so it's
// put straight after the program, before the options that follow.
func (c *Claude) readsPictures(l Launch, dir string) Launch {
	args := append([]string{l.Args[0], "--allowedTools", "Read(/" + dir + "/**)"}, l.Args[1:]...)
	l.Args = args
	return l
}
