package bothomepage

import (
	"errors"
	"net/url"
	"strings"

	"github.com/metaid-developers/metaso-p2p/internal/aggregator/skillservice"
)

var (
	ErrInvalidParameter       = errors.New("invalid parameter")
	ErrNotFound               = errors.New("bot homepage not found")
	ErrAggregationUnavailable = errors.New("aggregation unavailable")
)

type ServiceLister interface {
	List(skillservice.ListParams) (*skillservice.ListResult, error)
}

func (a *Aggregator) Build(requestGlobalMetaId string, opts Options) (*Data, error) {
	requestGlobalMetaId = strings.TrimSpace(requestGlobalMetaId)
	if requestGlobalMetaId == "" {
		return nil, ErrInvalidParameter
	}
	if a == nil || a.profileLookup == nil {
		return nil, ErrAggregationUnavailable
	}

	profile, err := a.profileLookup.LookupByGlobalMetaId(requestGlobalMetaId)
	if err != nil {
		return nil, ErrAggregationUnavailable
	}
	if profile == nil {
		return nil, ErrNotFound
	}

	resolvedAt := a.currentTime()
	canonical := CanonicalIdentity{
		GlobalMetaId: firstNonEmpty(profile.GlobalMetaId, requestGlobalMetaId),
		MetaId:       strings.TrimSpace(profile.MetaId),
		Address:      strings.TrimSpace(profile.Address),
		ChainName:    strings.TrimSpace(profile.ChainName),
	}
	outProfile := a.toProfile(profile, canonical.GlobalMetaId)
	services := make([]Service, 0)
	proofs := Proofs{
		VerificationState: "unverified",
		Identity:          nil,
		Profile:           make([]ProfileProof, 0),
		Homepage:          nil,
		Services:          make([]ServiceProof, 0),
	}
	warnings := make([]string, 0)
	if opts.IncludeProofs {
		proofs, warnings = buildProfileProofs(profile, canonical.GlobalMetaId)
	}

	return &Data{
		SchemaVersion: "botHomepage.v1",
		ResolvedAt:    resolvedAt,
		GlobalMetaId:  requestGlobalMetaId,
		Canonical:     canonical,
		Profile:       outProfile,
		Homepage:      toDefaultHomepage(outProfile),
		Presence:      unknownPresence(),
		Services:      services,
		Actions:       buildActions(outProfile.ChatPubkey, len(services), canonical.GlobalMetaId),
		Proofs:        proofs,
		Source:        a.source(resolvedAt),
		Warnings:      warnings,
	}, nil
}

func (a *Aggregator) currentTime() int64 {
	if a == nil || a.now == nil {
		return 0
	}
	return a.now()
}

func (a *Aggregator) toProfile(p *ProfileSnapshot, canonicalGlobalMetaId string) Profile {
	if p == nil {
		return Profile{DisplayGlobalId: abbreviateGlobalMetaId(canonicalGlobalMetaId)}
	}
	return Profile{
		Name:            strings.TrimSpace(p.Name),
		Avatar:          a.resolveAsset(p.Avatar),
		AvatarPinId:     strings.TrimSpace(p.AvatarId),
		Background:      a.resolveAsset(p.Background),
		BackgroundPinId: strings.TrimSpace(p.BackgroundId),
		Bio:             strings.TrimSpace(p.Bio),
		BioPinId:        strings.TrimSpace(p.BioId),
		ChatPubkey:      strings.TrimSpace(p.ChatPublicKey),
		ChatPubkeyPinId: strings.TrimSpace(p.ChatPublicKeyId),
		NftAvatar:       a.resolveAsset(p.NftAvatar),
		DisplayGlobalId: abbreviateGlobalMetaId(canonicalGlobalMetaId),
	}
}

func toDefaultHomepage(profile Profile) Homepage {
	return Homepage{
		Mode:    "default",
		Title:   profile.Name,
		Summary: profile.Bio,
		Custom:  nil,
	}
}

func (a *Aggregator) source(fetchedAt int64) Source {
	baseURL := ""
	if a != nil {
		baseURL = a.assetBaseURL
	}
	return Source{
		Resolver:        "metaso-p2p",
		Node:            contentOrigin(baseURL),
		ProfileEndpoint: "/api/info/globalmetaid/:globalMetaId",
		ServiceEndpoint: "/api/bot-hub/skill-service/list",
		ContentBaseURL:  baseURL,
		FetchedAt:       fetchedAt,
		Stale:           false,
	}
}

func contentOrigin(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func buildActions(chatPubkey string, serviceCount int, canonicalGlobalMetaId string) []Action {
	return []Action{
		{
			Id:                    "message",
			Label:                 "Message",
			Kind:                  "private-chat",
			Enabled:               strings.TrimSpace(chatPubkey) != "",
			RequiresUsingIdentity: true,
		},
		{
			Id:                    "services",
			Label:                 "Services",
			Kind:                  "service-list",
			Enabled:               serviceCount > 0,
			RequiresUsingIdentity: true,
		},
		{
			Id:                    "copy-uri",
			Label:                 "Copy URI",
			Kind:                  "copy",
			Enabled:               true,
			RequiresUsingIdentity: false,
			URI:                   "metaid://" + strings.TrimSpace(canonicalGlobalMetaId),
		},
	}
}

func buildProfileProofs(p *ProfileSnapshot, canonicalGlobalMetaId string) (Proofs, []string) {
	proofs := Proofs{
		VerificationState: "unverified",
		Identity:          nil,
		Profile:           make([]ProfileProof, 0),
		Homepage:          nil,
		Services:          make([]ServiceProof, 0),
	}
	warnings := make([]string, 0)
	if p == nil {
		warnings = append(warnings, "profile proof metadata is unavailable")
		return proofs, warnings
	}

	add := func(field, path, pinID string) {
		pinID = strings.TrimSpace(pinID)
		if pinID == "" {
			return
		}
		proofs.Profile = append(proofs.Profile, ProfileProof{
			Field:                 field,
			ProtocolPath:          path,
			PinId:                 pinID,
			PublisherGlobalMetaId: canonicalGlobalMetaId,
		})
		warnings = append(warnings, field+" proof txid/contentHash metadata is missing")
	}
	add("name", "/info/name", p.NameId)
	add("avatar", "/info/avatar", p.AvatarId)
	add("background", "/info/background", p.BackgroundId)
	add("bio", "/info/bio", p.BioId)
	add("chatPubkey", "/info/chatpubkey", p.ChatPublicKeyId)

	if len(proofs.Profile) > 0 {
		proofs.VerificationState = "partial"
		return proofs, warnings
	}
	warnings = append(warnings, "profile proof metadata is unavailable")
	return proofs, warnings
}

func (a *Aggregator) resolveAsset(asset string) string {
	if a == nil || a.assetResolver == nil {
		return strings.TrimSpace(asset)
	}
	return a.assetResolver.Resolve(asset)
}

func unknownPresence() Presence {
	return Presence{State: "unknown", UpdatedAt: nil, Source: ""}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func abbreviateGlobalMetaId(globalMetaId string) string {
	globalMetaId = strings.TrimSpace(globalMetaId)
	if len(globalMetaId) <= 16 {
		return globalMetaId
	}
	return globalMetaId[:8] + "..." + globalMetaId[len(globalMetaId)-6:]
}
