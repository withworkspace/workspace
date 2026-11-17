package cli

import (
	"testing"
	"time"
)

func TestInWords(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{time.Minute, "a minute"},
		{45 * time.Minute, "45 minutes"},
		{90 * time.Minute, "90 minutes"},
		{time.Hour, "an hour"},
		{2 * time.Hour, "2 hours"},
	} {
		if got := inWords(tc.d); got != tc.want {
			t.Errorf("inWords(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// With no idle timeout (the default) the daemon stays open until it's
// closed; with one, it says how long it idles first.
func TestIdleLine(t *testing.T) {
	for _, tc := range []struct {
		timeout time.Duration
		want    string
	}{
		{0, "it stays open until it's closed (ws daemon stop)"},
		{5 * time.Minute, "it exits after 5 minutes idle"},
	} {
		if got := idleLine(tc.timeout); got != tc.want {
			t.Errorf("idleLine(%v) = %q, want %q", tc.timeout, got, tc.want)
		}
	}
}
