package push

import (
	"sync"

	socketio "github.com/zishang520/socket.io/v2/socket"

	"github.com/metaid-developers/meta-socket/internal/socket"
)

type roomMembershipStore struct {
	mu              sync.RWMutex
	identityGroups  map[string]map[string]struct{}
	groupIdentities map[string]map[string]struct{}
}

var membershipStore = &roomMembershipStore{
	identityGroups:  make(map[string]map[string]struct{}),
	groupIdentities: make(map[string]map[string]struct{}),
}

func RegisterRoomJoiner() {
	manager := socket.GetManager()
	if manager == nil {
		return
	}
	manager.SetOnClientConnected(joinGroupRoomsForClient)
}

func TrackGroupMembership(metaID, globalMetaID, groupID string, removed bool) {
	if groupID == "" {
		return
	}
	identities := mergeUnique([]string{metaID}, []string{globalMetaID})
	if len(identities) == 0 {
		return
	}

	if removed {
		membershipStore.remove(groupID, identities...)
		for _, identity := range identities {
			LeaveGroupRoomForUser(identity, groupID)
		}
		return
	}

	membershipStore.add(groupID, identities...)
	for _, identity := range identities {
		JoinGroupRoomForUser(identity, groupID)
	}
}

func TrackGroupMembershipBatch(groupID string, metaIDs, globalMetaIDs []string) int {
	if groupID == "" {
		return 0
	}
	identities := mergeUnique(metaIDs, globalMetaIDs)
	if len(identities) == 0 {
		return 0
	}

	membershipStore.add(groupID, identities...)

	joined := 0
	for _, identity := range identities {
		joined += JoinGroupRoomForUser(identity, groupID)
	}
	return joined
}

func HasKnownMembers(groupID string) bool {
	return membershipStore.hasGroupMembers(groupID)
}

func KnownGroupMembers(groupID string) []string {
	return membershipStore.membersForGroup(groupID)
}

func JoinKnownGroupMembers(groupID string) int {
	if groupID == "" {
		return 0
	}
	joined := 0
	for _, identity := range KnownGroupMembers(groupID) {
		joined += JoinGroupRoomForUser(identity, groupID)
	}
	return joined
}

func joinGroupRoomsForClient(metaID string, client *socketio.Socket) {
	if !RoomBroadcastEnabled() {
		return
	}
	if metaID == "" || client == nil {
		return
	}
	for _, groupID := range membershipStore.groupsForIdentity(metaID) {
		room := GroupRoomName(groupID)
		if room == "" {
			continue
		}
		client.Join(socketio.Room(room))
	}
}

func (s *roomMembershipStore) add(groupID string, identities ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, identity := range identities {
		if identity == "" {
			continue
		}
		groupSet, ok := s.identityGroups[identity]
		if !ok {
			groupSet = make(map[string]struct{})
			s.identityGroups[identity] = groupSet
		}
		groupSet[groupID] = struct{}{}

		identitySet, ok := s.groupIdentities[groupID]
		if !ok {
			identitySet = make(map[string]struct{})
			s.groupIdentities[groupID] = identitySet
		}
		identitySet[identity] = struct{}{}
	}
}

func (s *roomMembershipStore) remove(groupID string, identities ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, identity := range identities {
		if identity == "" {
			continue
		}
		if groups, ok := s.identityGroups[identity]; ok {
			delete(groups, groupID)
			if len(groups) == 0 {
				delete(s.identityGroups, identity)
			}
		}
		if members, ok := s.groupIdentities[groupID]; ok {
			delete(members, identity)
			if len(members) == 0 {
				delete(s.groupIdentities, groupID)
			}
		}
	}
}

func (s *roomMembershipStore) groupsForIdentity(identity string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, ok := s.identityGroups[identity]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(set))
	for groupID := range set {
		result = append(result, groupID)
	}
	return result
}

func (s *roomMembershipStore) hasGroupMembers(groupID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, ok := s.groupIdentities[groupID]
	return ok && len(set) > 0
}

func (s *roomMembershipStore) membersForGroup(groupID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, ok := s.groupIdentities[groupID]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(set))
	for identity := range set {
		result = append(result, identity)
	}
	return result
}

func resetMembershipStoreForTest() {
	membershipStore.mu.Lock()
	defer membershipStore.mu.Unlock()

	membershipStore.identityGroups = make(map[string]map[string]struct{})
	membershipStore.groupIdentities = make(map[string]map[string]struct{})
}
