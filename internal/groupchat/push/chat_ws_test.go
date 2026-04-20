package push

import (
	"testing"

	"github.com/metaid-developers/meta-socket/internal/groupchat/db"
)

func TestBuildGroupFallbackTargetsIncludesKnownMembers(t *testing.T) {
	resetMembershipStoreForTest()
	TrackGroupMembership("member_meta_1", "id_member_1", "group_01", false)
	TrackGroupMembership("member_meta_2", "id_member_2", "group_01", false)

	chat := &db.TalkGroupChatV3{
		GroupID:      "group_01",
		MetaID:       "sender_meta",
		GlobalMetaID: "id_sender",
	}

	metaTargets, globalTargets := buildGroupFallbackTargets(chat)

	assertContains(t, metaTargets, "sender_meta")
	assertContains(t, metaTargets, "member_meta_1")
	assertContains(t, metaTargets, "member_meta_2")
	assertContains(t, globalTargets, "id_sender")
	assertContains(t, globalTargets, "id_member_1")
	assertContains(t, globalTargets, "id_member_2")
}

func assertContains(t *testing.T, values []string, target string) {
	t.Helper()
	for _, value := range values {
		if value == target {
			return
		}
	}
	t.Fatalf("expected %q in %v", target, values)
}
