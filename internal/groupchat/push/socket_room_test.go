package push

import "testing"

func TestTrackGroupMembershipBatchAndKnownMembers(t *testing.T) {
	resetMembershipStoreForTest()

	TrackGroupMembershipBatch("group_02", []string{"meta_a", "meta_b"}, []string{"id_a"})
	known := KnownGroupMembers("group_02")

	assertContains(t, known, "meta_a")
	assertContains(t, known, "meta_b")
	assertContains(t, known, "id_a")
}
