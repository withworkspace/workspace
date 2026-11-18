package plugin_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
)

// ws speaks only versions the protocol package has types for.
func TestWSSpeaksVersionsThePackageHas(t *testing.T) {
	for _, v := range plugin.Protocols {
		if !slices.Contains(protocol.Versions, v) {
			t.Errorf("ws speaks version %d, and the protocol package has %v", v, protocol.Versions)
		}
	}
}

func TestAgreedSaysWhichSideIsBehind(t *testing.T) {
	for _, c := range []struct {
		name   string
		answer protocol.InitializeResult
		want   string // "" for agreed
	}{
		{"chose what ws speaks", protocol.InitializeResult{Protocol: 1, Protocols: []int{1, 2}}, ""},
		{"from before versions were chosen", protocol.InitializeResult{Protocol: 1}, ""},
		{"newer, from before versions were chosen", protocol.InitializeResult{Protocol: 9}, "jira speaks the plugin protocol's version 9, and this ws speaks version 1. Update ws"},
		{"newer, none in common", protocol.InitializeResult{Protocols: []int{7, 8, 9}}, "jira speaks the plugin protocol's versions 7, 8 and 9, and this ws speaks version 1. Update ws"},
		{"chose what ws doesn't speak", protocol.InitializeResult{Protocol: 2, Protocols: []int{2}}, "Update ws"},
		{"says nothing it speaks", protocol.InitializeResult{}, "jira doesn't say which version of the plugin protocol it speaks, and this ws speaks version 1. Update the plugin"},
	} {
		err := plugin.Agreed("jira", c.answer)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
}
