package web

import (
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// A mission's need goes where what moves it on is: crew that asked, to
// their conversation; a bundle to verify, to it on Verify; and so on.
func TestAMissionsNeedGoesWhereItsDealtWith(t *testing.T) {
	cases := []struct {
		act  *mission.Act
		want string
	}{
		{nil, "/missions/dark"},
		{&mission.Act{Do: mission.DoReply, Target: "c2"}, "/missions/dark/crew/c2"},
		{&mission.Act{Do: mission.DoPermission, Target: "c2"}, "/missions/dark/crew/c2"},
		{&mission.Act{Do: mission.DoCrew, Target: "c3"}, "/missions/dark/crew/c3"},
		{&mission.Act{Do: mission.DoVerify, Target: "b1"}, "/missions/dark?tab=verify&b=b1"},
		{&mission.Act{Do: mission.DoReview, Target: "web/dark-mode"}, "/missions/dark/review/web/dark-mode"},
		{&mission.Act{Do: mission.DoRead, Target: "docs/plan.md"}, "/missions/dark/artifact?ref=docs%2Fplan.md"},
		{&mission.Act{Do: mission.DoTalk, Target: "t2"}, "/missions/dark/conversations/t2"},
		{&mission.Act{Do: mission.DoBriefPage}, "/missions/dark?tab=brief"},
		{&mission.Act{Do: mission.DoLog}, "/missions/dark?tab=log"},
		// Done on the mission's own page: a button there.
		{&mission.Act{Do: mission.DoAccept}, "/missions/dark"},
		{&mission.Act{Do: mission.DoAnswer, Task: 1}, "/missions/dark"},
		{&mission.Act{Do: mission.DoURL, URL: "https://github.com/x/y/pull/1"}, "/missions/dark"},
	}
	for _, c := range cases {
		got := needsOf([]ops.Need{{Of: ops.NeedMission, Ref: "dark", Act: c.act}})[0].To
		if got != c.want {
			t.Errorf("act %+v goes to %q, want %q", c.act, got, c.want)
		}
	}
}
