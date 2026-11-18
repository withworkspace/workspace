package cli

import "time"

// closeDoneEvery is how often the daemon looks for crew to close for having
// reported done (ops.CloseDone): a fraction of how long they're left open.
const closeDoneEvery = 30 * time.Second

// closeDoneLoop closes crew who've reported done, once their turn's over
// and nothing's looking at it, until stop closes.
func closeDoneLoop(root string, logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTicker(closeDoneEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			ctx, err := LoadOps(root)
			if err != nil {
				continue
			}
			for _, said := range ctx.CloseDone(now) {
				logf("done crew: %s", said)
			}
		}
	}
}
