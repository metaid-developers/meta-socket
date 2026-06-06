# Bot Homepage API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a native metaso-p2p Bot homepage aggregation endpoint at `GET /api/bot-homepage/globalmetaid/:globalMetaId` that returns one render-ready `botHomepage.v1` JSON document for OAC Bot Browser.

**Architecture:** Implement a new read-only `bothomepage` aggregator package that composes existing in-process `userinfo` profile lookup, existing `skillservice.List`, shared asset URL resolution, and optional presence readers. Keep `/api/info/*` compatibility untouched: Bot homepage uses metaso-p2p native `code=0` success envelopes and business error codes `40000`, `40400`, and `50000`.

**Tech Stack:** Go 1.26, Gin, Pebble-backed aggregator registry, `internal/aggregator/userinfo`, `internal/aggregator/skillservice`, `internal/presence`, `testing` with `httptest`, Markdown API specs.

---

## File Structure

- Create `internal/aggregator/bothomepage/types.go`: public response structs with stable JSON tags for `botHomepage.v1`.
- Create `internal/aggregator/bothomepage/query.go`: query option parsing and validation.
- Create `internal/aggregator/bothomepage/module.go`: aggregator skeleton, dependency setters, route registration, and no-op pin handlers.
- Create `internal/aggregator/bothomepage/api.go`: HTTP handler and native response/error envelope mapping.
- Create `internal/aggregator/bothomepage/build.go`: profile, homepage, service, action, proof, source, and warning assembly.
- Create `internal/aggregator/bothomepage/userinfo_adapter.go`: adapter from `userinfo.Aggregator` to homepage `ProfileLookup`.
- Create `internal/aggregator/bothomepage/presence.go`: optional presence lookup over `presence.LocalReader` and `presence.GlobalReader`.
- Create `internal/aggregator/bothomepage/*_test.go`: focused unit and handler tests for query parsing, success, errors, services, proofs, and presence.
- Modify `internal/aggregator/userinfo/module.go`: persist `BackgroundId` so homepage can emit `backgroundPinId`.
- Modify `cmd/metaso-p2p/main.go`: register `bothomepage.Aggregator` and wire userinfo, skillservice, asset base URL, and optional presence readers.
- Modify `internal/api/router_test.go`: include `skillservice` and `bothomepage` in full-router setup and add route-level acceptance coverage.
- Modify `README.md`: list the new Bot Homepage endpoint.
- Create `docs/specs/2026-06-07-bot-homepage-api.md`: metaso-p2p-local API contract derived from the product PRD.

## Implementation Notes

- The endpoint is read-only. `HandleBlockPin` and `HandleMempoolPin` in `bothomepage` return `(nil, nil)`.
- `globalMetaId` validation is intentionally narrow in v1: trim whitespace and reject an empty value. Do not add chain-specific format validation until product defines alias rules.
- `source.contentBaseUrl` is the same configured base URL used by Bot Hub assets: `cfg.BotHub.AssetBaseURL`.
- Proof metadata is partial in current storage. Emit known `pinId` and `protocolPath`; do not fabricate `txid` or `contentHash`.
- Missing proof metadata lowers `proofs.verificationState` to `"partial"` and appends explicit warnings.
- Presence is optional. If no reader can answer, return `presence.state="unknown"` with `updatedAt=null` and do not fail the request.
- Service aggregation must call `skillservice.List` with `ProviderGlobalMetaId=canonical.globalMetaId`, `SortBy="updated"`, `Order="desc"`, and the mapped query flags.
- Do not return subjective service action fields such as `available`, `canOrder`, `disabledReason`, or `availableReason`.

## Task 1: Add Query Parsing And Package Skeleton

**Files:**
- Create: `internal/aggregator/bothomepage/query.go`
- Create: `internal/aggregator/bothomepage/module.go`
- Create: `internal/aggregator/bothomepage/query_test.go`

- [ ] **Step 1: Write failing query parsing tests**

Create `internal/aggregator/bothomepage/query_test.go`:

```go
package bothomepage

import (
	"net/url"
	"testing"
)

func TestParseOptionsDefaults(t *testing.T) {
	got, err := ParseOptions(url.Values{})
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}
	if !got.IncludeServices {
		t.Fatal("IncludeServices default should be true")
	}
	if got.ServiceSize != 20 {
		t.Fatalf("ServiceSize = %d, want 20", got.ServiceSize)
	}
	if got.IncludeInactiveServices {
		t.Fatal("IncludeInactiveServices default should be false")
	}
	if !got.IncludeProofs {
		t.Fatal("IncludeProofs default should be true")
	}
	if !got.IncludePresence {
		t.Fatal("IncludePresence default should be true")
	}
	if got.ChainName != "" {
		t.Fatalf("ChainName = %q, want empty", got.ChainName)
	}
}

func TestParseOptionsClampsServiceSize(t *testing.T) {
	got, err := ParseOptions(url.Values{"serviceSize": {"101"}})
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}
	if got.ServiceSize != 100 {
		t.Fatalf("ServiceSize = %d, want 100", got.ServiceSize)
	}
}

func TestParseOptionsRejectsInvalidValues(t *testing.T) {
	cases := []url.Values{
		{"includeServices": {"maybe"}},
		{"includeProofs": {"sometimes"}},
		{"includePresence": {"unknown"}},
		{"includeInactiveServices": {"wat"}},
		{"serviceSize": {"-1"}},
		{"serviceSize": {"abc"}},
	}
	for _, values := range cases {
		if _, err := ParseOptions(values); err == nil {
			t.Fatalf("ParseOptions(%v) expected error", values)
		}
	}
}

func TestAggregatorInterfaceSkeleton(t *testing.T) {
	agg := &Aggregator{}
	if agg.Name() != "bothomepage" {
		t.Fatalf("Name() = %q, want bothomepage", agg.Name())
	}
	if evt, err := agg.HandleBlockPin(nil); err != nil || evt != nil {
		t.Fatalf("HandleBlockPin(nil) = (%v, %v), want nil nil", evt, err)
	}
	if evt, err := agg.HandleMempoolPin(nil); err != nil || evt != nil {
		t.Fatalf("HandleMempoolPin(nil) = (%v, %v), want nil nil", evt, err)
	}
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestParseOptions|TestAggregatorInterfaceSkeleton' -count=1
```

Expected: FAIL because the package and symbols do not exist.

- [ ] **Step 3: Add the parser and skeleton**

Create `internal/aggregator/bothomepage/query.go`:

```go
package bothomepage

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultServiceSize = 20
	maxServiceSize     = 100
)

type Options struct {
	IncludeServices         bool
	ServiceSize             int
	IncludeInactiveServices bool
	IncludeProofs           bool
	IncludePresence         bool
	ChainName               string
}

func DefaultOptions() Options {
	return Options{
		IncludeServices: true,
		ServiceSize:     defaultServiceSize,
		IncludeProofs:   true,
		IncludePresence: true,
	}
}

func ParseOptions(values url.Values) (Options, error) {
	opts := DefaultOptions()
	var err error
	if raw := values.Get("includeServices"); raw != "" {
		opts.IncludeServices, err = parseBoolFlag(raw)
		if err != nil {
			return Options{}, errors.New("invalid includeServices")
		}
	}
	if raw := values.Get("includeInactiveServices"); raw != "" {
		opts.IncludeInactiveServices, err = parseBoolFlag(raw)
		if err != nil {
			return Options{}, errors.New("invalid includeInactiveServices")
		}
	}
	if raw := values.Get("includeProofs"); raw != "" {
		opts.IncludeProofs, err = parseBoolFlag(raw)
		if err != nil {
			return Options{}, errors.New("invalid includeProofs")
		}
	}
	if raw := values.Get("includePresence"); raw != "" {
		opts.IncludePresence, err = parseBoolFlag(raw)
		if err != nil {
			return Options{}, errors.New("invalid includePresence")
		}
	}
	if raw := strings.TrimSpace(values.Get("serviceSize")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return Options{}, errors.New("invalid serviceSize")
		}
		if n == 0 {
			n = defaultServiceSize
		}
		if n > maxServiceSize {
			n = maxServiceSize
		}
		opts.ServiceSize = n
	}
	opts.ChainName = strings.ToLower(strings.TrimSpace(values.Get("chainName")))
	return opts, nil
}

func parseBoolFlag(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	default:
		return false, errors.New("invalid boolean")
	}
}
```

Create `internal/aggregator/bothomepage/module.go`:

```go
package bothomepage

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/metaso-p2p/internal/aggregator"
	"github.com/metaid-developers/metaso-p2p/internal/cache"
	"github.com/metaid-developers/metaso-p2p/internal/storage"
)

type Aggregator struct {
	store    *storage.PebbleStore
	cache    *cache.Cache[[]byte]
	notifyCh chan *aggregator.NotifyEvent
	now      func() int64
}

const (
	namespace       = "bothomepage"
	cacheMaxEntries = 1000
	cacheTTL        = 30 * time.Second
)

func (a *Aggregator) Name() string { return "bothomepage" }

func (a *Aggregator) Init(store *storage.PebbleStore, cacheProvider *cache.CacheProvider) error {
	a.store = store
	if cacheProvider != nil {
		a.cache = cacheProvider.Namespace(namespace, cacheMaxEntries, cacheTTL)
	}
	a.notifyCh = make(chan *aggregator.NotifyEvent, 1)
	if a.now == nil {
		a.now = func() int64 { return time.Now().UnixMilli() }
	}
	return nil
}

func (a *Aggregator) HandleBlockPin(pin *aggregator.PinInscription) (*aggregator.NotifyEvent, error) {
	return nil, nil
}

func (a *Aggregator) HandleMempoolPin(pin *aggregator.PinInscription) (*aggregator.NotifyEvent, error) {
	return nil, nil
}

func (a *Aggregator) NotifyChannel() <-chan *aggregator.NotifyEvent {
	return a.notifyCh
}

func (a *Aggregator) RegisterRoutes(router *gin.RouterGroup) {
}
```

- [ ] **Step 4: Verify GREEN for Task 1**

Run:

```bash
gofmt -w internal/aggregator/bothomepage
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestParseOptions|TestAggregatorInterfaceSkeleton' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

Run:

```bash
git add internal/aggregator/bothomepage/query.go internal/aggregator/bothomepage/module.go internal/aggregator/bothomepage/query_test.go
git commit -m "feat: add bot homepage aggregator skeleton"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing the parser defaults, validation rules, and no-op read-model aggregator skeleton.

## Task 2: Build Profile, Homepage, Source, Actions, And Partial Proofs

**Files:**
- Create: `internal/aggregator/bothomepage/types.go`
- Create: `internal/aggregator/bothomepage/build.go`
- Create: `internal/aggregator/bothomepage/userinfo_adapter.go`
- Create: `internal/aggregator/bothomepage/build_test.go`
- Modify: `internal/aggregator/bothomepage/module.go`
- Modify: `internal/aggregator/userinfo/module.go`
- Modify: `internal/aggregator/userinfo/module_test.go`

- [ ] **Step 1: Write failing profile assembly tests**

Create `internal/aggregator/bothomepage/build_test.go`:

```go
package bothomepage

import (
	"testing"

	"github.com/metaid-developers/metaso-p2p/internal/aggregator/skillservice"
)

type fakeProfileLookup struct {
	byGlobal map[string]*ProfileSnapshot
	err      error
}

func (f fakeProfileLookup) LookupByGlobalMetaId(globalMetaId string) (*ProfileSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byGlobal[globalMetaId], nil
}

type fakeServiceLister struct{}

func (fakeServiceLister) List(p skillservice.ListParams) (*skillservice.ListResult, error) {
	return &skillservice.ListResult{List: []skillservice.ServiceListItem{}}, nil
}

func TestBuildHomepageProfileDefaultModeAndPartialProofs(t *testing.T) {
	agg := &Aggregator{}
	if err := agg.Init(nil, nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	agg.now = func() int64 { return 1780760000000 }
	agg.SetAssetBaseURL("https://file.metaid.io/metafile-indexer/content")
	agg.SetProfileLookup(fakeProfileLookup{byGlobal: map[string]*ProfileSnapshot{
		"idqBot": {
			GlobalMetaId:    "idqBot",
			MetaId:          "metaBot",
			Address:         "18Bot",
			ChainName:       "mvc",
			Name:            "Lisa Hahn Bot",
			NameId:          "name-pin",
			Avatar:          "/content/avatar-pin",
			AvatarId:        "avatar-pin",
			Background:      "/content/background-pin",
			BackgroundId:    "background-pin",
			Bio:             "Builds MetaApp interfaces.",
			BioId:           "bio-pin",
			ChatPublicKey:   "04chat",
			ChatPublicKeyId: "chat-pin",
			NftAvatar:       "",
		},
	}})

	data, err := agg.Build("idqBot", DefaultOptions())
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if data.SchemaVersion != "botHomepage.v1" {
		t.Fatalf("schemaVersion = %q", data.SchemaVersion)
	}
	if data.ResolvedAt != 1780760000000 {
		t.Fatalf("resolvedAt = %d", data.ResolvedAt)
	}
	if data.Canonical.GlobalMetaId != "idqBot" || data.Canonical.MetaId != "metaBot" {
		t.Fatalf("canonical = %+v", data.Canonical)
	}
	if data.Profile.Avatar != "https://file.metaid.io/metafile-indexer/content/avatar-pin" {
		t.Fatalf("avatar = %q", data.Profile.Avatar)
	}
	if data.Profile.BackgroundPinId != "background-pin" {
		t.Fatalf("backgroundPinId = %q", data.Profile.BackgroundPinId)
	}
	if data.Homepage.Mode != "default" || data.Homepage.Title != "Lisa Hahn Bot" {
		t.Fatalf("homepage = %+v", data.Homepage)
	}
	if len(data.Actions) != 3 {
		t.Fatalf("actions length = %d, want 3", len(data.Actions))
	}
	if !data.Actions[0].Enabled {
		t.Fatalf("message action should be enabled when chatPubkey is present")
	}
	if data.Proofs.VerificationState != "partial" {
		t.Fatalf("verificationState = %q, want partial", data.Proofs.VerificationState)
	}
	if len(data.Proofs.Profile) == 0 {
		t.Fatal("expected profile proof summaries from known pin ids")
	}
	if len(data.Warnings) == 0 {
		t.Fatal("expected warnings for missing txid/content hash proof metadata")
	}
}
```

In `internal/aggregator/userinfo/module_test.go`, add coverage for background pin id persistence:

```go
func TestHandleBlockPin_BackgroundStoresPinId(t *testing.T) {
	agg, store, _ := setupTestAggregator(t)
	defer store.Close()

	if _, err := agg.HandleBlockPin(&aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "bg-user",
		Address:   "18Bg",
		ChainName: "mvc",
		Id:        "init-bg:i0",
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := agg.HandleBlockPin(&aggregator.PinInscription{
		Path:      "/info/background",
		Operation: "create",
		MetaId:    "bg-user",
		Address:   "18Bg",
		ChainName: "mvc",
		Id:        "background-pin:i0",
	}); err != nil {
		t.Fatalf("background: %v", err)
	}

	profile, err := agg.LookupByMetaId("bg-user")
	if err != nil {
		t.Fatalf("LookupByMetaId: %v", err)
	}
	if profile.BackgroundId != "background-pin:i0" {
		t.Fatalf("BackgroundId = %q, want background-pin:i0", profile.BackgroundId)
	}
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage ./internal/aggregator/userinfo -run 'TestBuildHomepageProfileDefaultModeAndPartialProofs|TestHandleBlockPin_BackgroundStoresPinId' -count=1
```

Expected: FAIL because response types, `Build`, profile adapter types, and `BackgroundId` do not exist.

- [ ] **Step 3: Add response structs**

Create `internal/aggregator/bothomepage/types.go`:

```go
package bothomepage

type Data struct {
	SchemaVersion string            `json:"schemaVersion"`
	ResolvedAt    int64             `json:"resolvedAt"`
	GlobalMetaId  string            `json:"globalMetaId"`
	Canonical     CanonicalIdentity `json:"canonical"`
	Profile       Profile           `json:"profile"`
	Homepage      Homepage          `json:"homepage"`
	Presence      Presence          `json:"presence"`
	Services      []Service         `json:"services"`
	Actions       []Action          `json:"actions"`
	Proofs        Proofs            `json:"proofs"`
	Source        Source            `json:"source"`
	Warnings      []string          `json:"warnings"`
}

type CanonicalIdentity struct {
	GlobalMetaId string `json:"globalMetaId"`
	MetaId       string `json:"metaid"`
	Address      string `json:"address"`
	ChainName    string `json:"chainName"`
}

type Profile struct {
	Name            string `json:"name"`
	Avatar          string `json:"avatar"`
	AvatarPinId     string `json:"avatarPinId"`
	Background      string `json:"background"`
	BackgroundPinId string `json:"backgroundPinId"`
	Bio             string `json:"bio"`
	BioPinId        string `json:"bioPinId"`
	ChatPubkey      string `json:"chatPubkey"`
	ChatPubkeyPinId string `json:"chatPubkeyPinId"`
	NftAvatar       string `json:"nftAvatar"`
	DisplayGlobalId string `json:"displayGlobalMetaId"`
}

type Homepage struct {
	Mode    string          `json:"mode"`
	Title   string          `json:"title"`
	Summary string          `json:"summary"`
	Custom  *CustomHomepage `json:"custom"`
}

type CustomHomepage struct {
	URI          string `json:"uri"`
	PinId        string `json:"pinId"`
	ContentType  string `json:"contentType"`
	Renderer     string `json:"renderer"`
	Txid         string `json:"txid,omitempty"`
	ProtocolPath string `json:"protocolPath"`
}

type Presence struct {
	State     string `json:"state"`
	UpdatedAt *int64 `json:"updatedAt"`
	Source    string `json:"source"`
}

type Service struct {
	Id                 string        `json:"id"`
	CurrentPinId       string        `json:"currentPinId"`
	SourceServicePinId string        `json:"sourceServicePinId"`
	DisplayName        string        `json:"displayName"`
	ServiceName        string        `json:"serviceName"`
	Description        string        `json:"description"`
	ServiceIcon        string        `json:"serviceIcon"`
	ProviderSkill      string        `json:"providerSkill"`
	OutputType         string        `json:"outputType"`
	Price              string        `json:"price"`
	Currency           string        `json:"currency"`
	SettlementKind     string        `json:"settlementKind"`
	PaymentChain       string        `json:"paymentChain"`
	MRC20Ticker        any           `json:"mrc20Ticker"`
	MRC20Id            any           `json:"mrc20Id"`
	PaymentAddress     string        `json:"paymentAddress"`
	RatingAvg          float64       `json:"ratingAvg"`
	RatingCount        int64         `json:"ratingCount"`
	Status             int           `json:"status"`
	Operation          string        `json:"operation"`
	Disabled           bool          `json:"disabled"`
	ChainName          string        `json:"chainName"`
	CreatedAt          int64         `json:"createdAt"`
	UpdatedAt          int64         `json:"updatedAt"`
	Proof              *ServiceProof `json:"proof,omitempty"`
}

type Action struct {
	Id                    string `json:"id"`
	Label                 string `json:"label"`
	Kind                  string `json:"kind"`
	Enabled               bool   `json:"enabled"`
	RequiresUsingIdentity bool   `json:"requiresUsingIdentity"`
	URI                   string `json:"uri,omitempty"`
}

type Proofs struct {
	VerificationState string         `json:"verificationState"`
	Identity          *ProofSummary  `json:"identity"`
	Profile           []ProfileProof `json:"profile"`
	Homepage          *ProofSummary  `json:"homepage"`
	Services          []ServiceProof `json:"services"`
}

type ProofSummary struct {
	Txid                  string `json:"txid,omitempty"`
	PinId                 string `json:"pinId,omitempty"`
	ProtocolPath          string `json:"protocolPath"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId,omitempty"`
	ContentHash           string `json:"contentHash,omitempty"`
	ExplorerURL           string `json:"explorerUrl,omitempty"`
}

type ProfileProof struct {
	Field                 string `json:"field"`
	Txid                  string `json:"txid,omitempty"`
	PinId                 string `json:"pinId,omitempty"`
	ProtocolPath          string `json:"protocolPath"`
	ContentHash           string `json:"contentHash,omitempty"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId,omitempty"`
}

type ServiceProof struct {
	ServiceId             string `json:"serviceId,omitempty"`
	Txid                  string `json:"txid,omitempty"`
	PinId                 string `json:"pinId"`
	SourceServicePinId    string `json:"sourceServicePinId,omitempty"`
	ProtocolPath          string `json:"protocolPath"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId"`
}

type Source struct {
	Resolver        string `json:"resolver"`
	Node            string `json:"node"`
	ProfileEndpoint string `json:"profileEndpoint"`
	ServiceEndpoint string `json:"serviceEndpoint"`
	ContentBaseURL  string `json:"contentBaseUrl"`
	FetchedAt       int64  `json:"fetchedAt"`
	Stale           bool   `json:"stale"`
}
```

- [ ] **Step 4: Add profile lookup interface and userinfo adapter**

Create `internal/aggregator/bothomepage/userinfo_adapter.go`:

```go
package bothomepage

import "github.com/metaid-developers/metaso-p2p/internal/aggregator/userinfo"

type ProfileSnapshot struct {
	GlobalMetaId    string
	MetaId          string
	Address         string
	Name            string
	NameId          string
	Avatar          string
	AvatarId        string
	NftAvatar       string
	Bio             string
	BioId           string
	Background      string
	BackgroundId    string
	ChatPublicKey   string
	ChatPublicKeyId string
	ChainName       string
}

type ProfileLookup interface {
	LookupByGlobalMetaId(globalMetaId string) (*ProfileSnapshot, error)
}

type userInfoLookupAdapter struct {
	ui *userinfo.Aggregator
}

func NewUserInfoLookupAdapter(ui *userinfo.Aggregator) ProfileLookup {
	return &userInfoLookupAdapter{ui: ui}
}

func (a *userInfoLookupAdapter) LookupByGlobalMetaId(globalMetaId string) (*ProfileSnapshot, error) {
	if a == nil || a.ui == nil {
		return nil, nil
	}
	p, err := a.ui.LookupByGlobalMetaId(globalMetaId)
	return profileFromUserInfo(p), err
}

func profileFromUserInfo(p *userinfo.UserProfile) *ProfileSnapshot {
	if p == nil {
		return nil
	}
	return &ProfileSnapshot{
		GlobalMetaId:    p.GlobalMetaID,
		MetaId:          p.MetaID,
		Address:         p.Address,
		Name:            p.Name,
		NameId:          p.NameId,
		Avatar:          p.Avatar,
		AvatarId:        p.AvatarId,
		NftAvatar:       p.NftAvatar,
		Bio:             p.Bio,
		BioId:           p.BioId,
		Background:      p.Background,
		BackgroundId:    p.BackgroundId,
		ChatPublicKey:   p.ChatPublicKey,
		ChatPublicKeyId: p.ChatPublicKeyId,
		ChainName:       p.ChainName,
	}
}
```

- [ ] **Step 5: Add profile lookup and asset setters to the module**

In `internal/aggregator/bothomepage/module.go`, add this import:

```go
	"github.com/metaid-developers/metaso-p2p/internal/aggregator/skillservice"
```

Extend `Aggregator` with these fields:

```go
	profileLookup ProfileLookup
	assetResolver *skillservice.AssetResolver
	assetBaseURL  string
```

Add these setters:

```go
func (a *Aggregator) SetProfileLookup(lookup ProfileLookup) {
	a.profileLookup = lookup
}

func (a *Aggregator) SetAssetBaseURL(baseURL string) {
	resolver := skillservice.NewAssetResolver(baseURL)
	a.assetResolver = resolver
	a.assetBaseURL = resolver.BaseURL()
}
```

- [ ] **Step 6: Store `BackgroundId` in userinfo**

In `internal/aggregator/userinfo/module.go`, add the field to `UserProfile`:

```go
BackgroundId string `json:"backgroundId,omitempty"`
```

Change the `/info/background` branch to:

```go
case path == "/info/background":
	profile.Background = "/content/" + pin.Id
	profile.BackgroundId = pin.Id
```

- [ ] **Step 7: Add profile assembly**

Create `internal/aggregator/bothomepage/build.go`:

```go
package bothomepage

import (
	"errors"
	"net/url"
	"strings"

	"github.com/metaid-developers/metaso-p2p/internal/aggregator/skillservice"
)

var (
	ErrInvalidParameter      = errors.New("invalid parameter")
	ErrNotFound              = errors.New("bot homepage not found")
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
	if a.profileLookup == nil {
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
		MetaId:       profile.MetaId,
		Address:      profile.Address,
		ChainName:    profile.ChainName,
	}
	out := &Data{
		SchemaVersion: "botHomepage.v1",
		ResolvedAt:    resolvedAt,
		GlobalMetaId:  requestGlobalMetaId,
		Canonical:     canonical,
		Profile:       a.toProfile(profile, canonical.GlobalMetaId),
		Homepage:      toDefaultHomepage(profile),
		Presence:      unknownPresence(),
		Services:      []Service{},
		Proofs:        Proofs{VerificationState: "unverified", Identity: nil, Profile: []ProfileProof{}, Homepage: nil, Services: []ServiceProof{}},
		Source:        a.source(resolvedAt),
		Warnings:      []string{},
	}
	if opts.IncludeProofs {
		out.Proofs, out.Warnings = buildProfileProofs(profile, canonical.GlobalMetaId, out.Warnings)
	}
	out.Actions = buildActions(out.Profile.ChatPubkey, len(out.Services), canonical.GlobalMetaId)
	return out, nil
}

func (a *Aggregator) currentTime() int64 {
	if a != nil && a.now != nil {
		return a.now()
	}
	return 0
}

func (a *Aggregator) toProfile(p *ProfileSnapshot, canonicalGlobalMetaId string) Profile {
	return Profile{
		Name:            p.Name,
		Avatar:          a.resolveAsset(p.Avatar),
		AvatarPinId:     p.AvatarId,
		Background:      a.resolveAsset(p.Background),
		BackgroundPinId: p.BackgroundId,
		Bio:             p.Bio,
		BioPinId:        p.BioId,
		ChatPubkey:      p.ChatPublicKey,
		ChatPubkeyPinId: p.ChatPublicKeyId,
		NftAvatar:       p.NftAvatar,
		DisplayGlobalId: abbreviateGlobalMetaId(canonicalGlobalMetaId),
	}
}

func toDefaultHomepage(p *ProfileSnapshot) Homepage {
	return Homepage{Mode: "default", Title: p.Name, Summary: p.Bio, Custom: nil}
}

func (a *Aggregator) source(fetchedAt int64) Source {
	return Source{
		Resolver:        "metaso-p2p",
		Node:            contentOrigin(a.assetBaseURL),
		ProfileEndpoint: "/api/info/globalmetaid/:globalMetaId",
		ServiceEndpoint: "/api/bot-hub/skill-service/list",
		ContentBaseURL:  a.assetBaseURL,
		FetchedAt:       fetchedAt,
		Stale:           false,
	}
}

func contentOrigin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func buildActions(chatPubkey string, serviceCount int, canonicalGlobalMetaId string) []Action {
	return []Action{
		{Id: "message", Label: "Message", Kind: "private-chat", Enabled: strings.TrimSpace(chatPubkey) != "", RequiresUsingIdentity: true},
		{Id: "services", Label: "Services", Kind: "service-list", Enabled: serviceCount > 0, RequiresUsingIdentity: true},
		{Id: "copy-uri", Label: "Copy URI", Kind: "copy", Enabled: true, URI: "metaid://" + canonicalGlobalMetaId},
	}
}

func buildProfileProofs(p *ProfileSnapshot, publisherGlobalMetaId string, warnings []string) (Proofs, []string) {
	proofs := Proofs{VerificationState: "partial", Identity: nil, Profile: []ProfileProof{}, Homepage: nil, Services: []ServiceProof{}}
	add := func(field, pinId, path string) {
		if strings.TrimSpace(pinId) == "" {
			return
		}
		proofs.Profile = append(proofs.Profile, ProfileProof{
			Field: field, PinId: pinId, ProtocolPath: path, PublisherGlobalMetaId: publisherGlobalMetaId,
		})
		warnings = append(warnings, "profile proof for "+field+" is missing txid/contentHash metadata")
	}
	add("name", p.NameId, "/info/name")
	add("avatar", p.AvatarId, "/info/avatar")
	add("background", p.BackgroundId, "/info/background")
	add("bio", p.BioId, "/info/bio")
	add("chatPubkey", p.ChatPublicKeyId, "/info/chatpubkey")
	if len(proofs.Profile) == 0 {
		proofs.VerificationState = "unverified"
		warnings = append(warnings, "profile proof metadata is unavailable")
	}
	return proofs, warnings
}

func (a *Aggregator) resolveAsset(asset string) string {
	if a == nil || a.assetResolver == nil {
		return asset
	}
	return a.assetResolver.Resolve(asset)
}

func unknownPresence() Presence {
	return Presence{State: "unknown", UpdatedAt: nil, Source: ""}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func abbreviateGlobalMetaId(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 16 {
		return value
	}
	return value[:8] + "..." + value[len(value)-6:]
}
```

- [ ] **Step 8: Verify GREEN for Task 2**

Run:

```bash
gofmt -w internal/aggregator/bothomepage internal/aggregator/userinfo/module.go internal/aggregator/userinfo/module_test.go
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage ./internal/aggregator/userinfo -run 'TestBuildHomepageProfileDefaultModeAndPartialProofs|TestHandleBlockPin_BackgroundStoresPinId' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 2**

Run:

```bash
git add internal/aggregator/bothomepage/types.go internal/aggregator/bothomepage/build.go internal/aggregator/bothomepage/userinfo_adapter.go internal/aggregator/bothomepage/build_test.go internal/aggregator/bothomepage/module.go internal/aggregator/userinfo/module.go internal/aggregator/userinfo/module_test.go
git commit -m "feat: build bot homepage profile view"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing profile/homepage mapping, asset URL resolution, `BackgroundId`, and partial proof warnings.

## Task 3: Add HTTP Handler And Error Envelopes

**Files:**
- Create: `internal/aggregator/bothomepage/api.go`
- Create: `internal/aggregator/bothomepage/api_test.go`
- Modify: `internal/aggregator/bothomepage/module.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/aggregator/bothomepage/api_test.go`:

```go
package bothomepage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newHandlerFixture(t *testing.T) (*gin.Engine, *Aggregator) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	agg := &Aggregator{}
	if err := agg.Init(nil, nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	agg.now = func() int64 { return 1780760000000 }
	agg.SetAssetBaseURL("https://file.metaid.io/metafile-indexer/content")
	agg.SetProfileLookup(fakeProfileLookup{byGlobal: map[string]*ProfileSnapshot{
		"idqBot": {GlobalMetaId: "idqBot", MetaId: "metaBot", Address: "18Bot", ChainName: "mvc", Name: "Bot", ChatPublicKey: "04chat"},
	}})
	router := gin.New()
	agg.RegisterRoutes(router.Group("/api"))
	return router, agg
}

func callHomepage(t *testing.T, router *gin.Engine, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v raw=%s", err, w.Body.String())
	}
	return w.Code, body
}

func TestHandleGlobalMetaIDSuccessEnvelope(t *testing.T) {
	router, _ := newHandlerFixture(t)
	status, body := callHomepage(t, router, "/api/bot-homepage/globalmetaid/idqBot")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("code = %v", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["schemaVersion"] != "botHomepage.v1" {
		t.Fatalf("schemaVersion = %v", data["schemaVersion"])
	}
}

func TestHandleGlobalMetaIDInvalidParameter(t *testing.T) {
	router, _ := newHandlerFixture(t)
	_, body := callHomepage(t, router, "/api/bot-homepage/globalmetaid/%20%20")
	if body["code"].(float64) != 40000 {
		t.Fatalf("code = %v body=%v", body["code"], body)
	}
	if body["message"] != "invalid parameter" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestHandleGlobalMetaIDUnknownBot(t *testing.T) {
	router, _ := newHandlerFixture(t)
	_, body := callHomepage(t, router, "/api/bot-homepage/globalmetaid/idqMissing")
	if body["code"].(float64) != 40400 {
		t.Fatalf("code = %v body=%v", body["code"], body)
	}
	if body["message"] != "bot homepage not found" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestHandleGlobalMetaIDInvalidQuery(t *testing.T) {
	router, _ := newHandlerFixture(t)
	_, body := callHomepage(t, router, "/api/bot-homepage/globalmetaid/idqBot?includeServices=maybe")
	if body["code"].(float64) != 40000 {
		t.Fatalf("code = %v body=%v", body["code"], body)
	}
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestHandleGlobalMetaID' -count=1
```

Expected: FAIL because `handleGlobalMetaID` does not exist.

- [ ] **Step 3: Implement handler**

Create `internal/aggregator/bothomepage/api.go`:

```go
package bothomepage

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/metaso-p2p/internal/api"
)

func (a *Aggregator) handleGlobalMetaID(c *gin.Context) {
	opts, err := ParseOptions(c.Request.URL.Query())
	if err != nil {
		api.RespErr(c, 40000, "invalid parameter")
		return
	}
	data, err := a.Build(c.Param("globalMetaId"), opts)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidParameter):
			api.RespErr(c, 40000, "invalid parameter")
		case errors.Is(err, ErrNotFound):
			api.RespErr(c, 40400, "bot homepage not found")
		default:
			api.RespErr(c, 50000, "aggregation unavailable")
		}
		return
	}
	api.RespSuccess(c, data)
}
```

- [ ] **Step 4: Register the HTTP route**

In `internal/aggregator/bothomepage/module.go`, replace the empty route registration with:

```go
func (a *Aggregator) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/bot-homepage/globalmetaid/:globalMetaId", a.handleGlobalMetaID)
}
```

- [ ] **Step 5: Verify GREEN for Task 3**

Run:

```bash
gofmt -w internal/aggregator/bothomepage
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestHandleGlobalMetaID' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

Run:

```bash
git add internal/aggregator/bothomepage/api.go internal/aggregator/bothomepage/api_test.go internal/aggregator/bothomepage/module.go
git commit -m "feat: serve bot homepage endpoint"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing the endpoint path and native error envelope behavior.

## Task 4: Reuse Skill-Service Aggregation

**Files:**
- Modify: `internal/aggregator/bothomepage/module.go`
- Modify: `internal/aggregator/bothomepage/build.go`
- Modify: `internal/aggregator/bothomepage/build_test.go`

- [ ] **Step 1: Add failing services tests**

Append to `internal/aggregator/bothomepage/build_test.go`:

```go
type recordingServiceLister struct {
	gotParams skillservice.ListParams
	result    *skillservice.ListResult
	err       error
}

func (r *recordingServiceLister) List(p skillservice.ListParams) (*skillservice.ListResult, error) {
	r.gotParams = p
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

func TestBuildHomepageIncludesProviderServices(t *testing.T) {
	agg := &Aggregator{}
	if err := agg.Init(nil, nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	agg.now = func() int64 { return 1780760000000 }
	agg.SetAssetBaseURL("https://file.metaid.io/metafile-indexer/content")
	agg.SetProfileLookup(fakeProfileLookup{byGlobal: map[string]*ProfileSnapshot{
		"idqBot": {GlobalMetaId: "idqBot", MetaId: "metaBot", Address: "18Bot", ChainName: "mvc", Name: "Bot", ChatPublicKey: "04chat"},
	}})
	lister := &recordingServiceLister{result: &skillservice.ListResult{List: []skillservice.ServiceListItem{
		{
			Id: "svc-current", CurrentPinId: "svc-current", SourceServicePinId: "svc-source",
			DisplayName: "Review MetaApp manifest", ServiceName: "review-metaapp-manifest", Description: "Review manifests",
			ServiceIcon: "https://cdn.example/icon.png", ProviderSkill: "review-metaapp-manifest", OutputType: "text",
			Price: "0", Currency: "SPACE", SettlementKind: "native", PaymentChain: "mvc", PaymentAddress: "18Bot",
			RatingAvg: 4.8, RatingCount: 12, Status: 0, Operation: "modify", Disabled: false,
			ChainName: "mvc", CreatedAt: 1780700000000, UpdatedAt: 1780750000000,
		},
	}}}
	agg.SetServiceLister(lister)

	opts := DefaultOptions()
	opts.ServiceSize = 7
	opts.IncludeInactiveServices = true
	opts.ChainName = "mvc"
	data, err := agg.Build("idqBot", opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if lister.gotParams.ProviderGlobalMetaId != "idqBot" {
		t.Fatalf("ProviderGlobalMetaId = %q", lister.gotParams.ProviderGlobalMetaId)
	}
	if lister.gotParams.Size != 7 || !lister.gotParams.IncludeInactive || lister.gotParams.ChainName != "mvc" {
		t.Fatalf("ListParams = %+v", lister.gotParams)
	}
	if len(data.Services) != 1 {
		t.Fatalf("services length = %d", len(data.Services))
	}
	if data.Services[0].Proof == nil || data.Services[0].Proof.ProtocolPath != skillservice.PathSkillService {
		t.Fatalf("service proof = %+v", data.Services[0].Proof)
	}
	if !data.Actions[1].Enabled {
		t.Fatal("services action should be enabled when services are present")
	}
	if len(data.Proofs.Services) != 1 {
		t.Fatalf("proof services length = %d", len(data.Proofs.Services))
	}
}

func TestBuildHomepageSkipsServicesWhenDisabled(t *testing.T) {
	agg := &Aggregator{}
	if err := agg.Init(nil, nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	agg.SetProfileLookup(fakeProfileLookup{byGlobal: map[string]*ProfileSnapshot{
		"idqBot": {GlobalMetaId: "idqBot", MetaId: "metaBot"},
	}})
	lister := &recordingServiceLister{result: &skillservice.ListResult{List: []skillservice.ServiceListItem{{Id: "svc"}}}}
	agg.SetServiceLister(lister)

	opts := DefaultOptions()
	opts.IncludeServices = false
	data, err := agg.Build("idqBot", opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if lister.gotParams.Size != 0 {
		t.Fatalf("service lister should not be called, got params %+v", lister.gotParams)
	}
	if len(data.Services) != 0 {
		t.Fatalf("services length = %d", len(data.Services))
	}
}

func TestBuildHomepageSuppressesProofsWhenDisabled(t *testing.T) {
	agg := &Aggregator{}
	if err := agg.Init(nil, nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	agg.SetProfileLookup(fakeProfileLookup{byGlobal: map[string]*ProfileSnapshot{
		"idqBot": {GlobalMetaId: "idqBot", MetaId: "metaBot", NameId: "name-pin"},
	}})
	agg.SetServiceLister(&recordingServiceLister{result: &skillservice.ListResult{List: []skillservice.ServiceListItem{
		{Id: "svc-current", CurrentPinId: "svc-current", SourceServicePinId: "svc-source"},
	}}})

	opts := DefaultOptions()
	opts.IncludeProofs = false
	data, err := agg.Build("idqBot", opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(data.Proofs.Profile) != 0 || len(data.Proofs.Services) != 0 {
		t.Fatalf("proofs should be empty when includeProofs=false: %+v", data.Proofs)
	}
	if len(data.Services) != 1 || data.Services[0].Proof != nil {
		t.Fatalf("service proof should be nil when includeProofs=false: %+v", data.Services)
	}
	if len(data.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none for suppressed proofs", data.Warnings)
	}
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestBuildHomepageIncludesProviderServices|TestBuildHomepageSkipsServicesWhenDisabled|TestBuildHomepageSuppressesProofsWhenDisabled' -count=1
```

Expected: FAIL because `Build` does not call `ServiceLister`.

- [ ] **Step 3: Add service lister wiring to the module**

In `internal/aggregator/bothomepage/module.go`, extend `Aggregator` with:

```go
	serviceLister ServiceLister
```

Add this setter:

```go
func (a *Aggregator) SetServiceLister(lister ServiceLister) {
	a.serviceLister = lister
}
```

- [ ] **Step 4: Add service mapping**

In `internal/aggregator/bothomepage/build.go`, after profile proof assembly and before actions:

```go
if opts.IncludeServices {
	services, serviceProofs, warnings, err := a.loadServices(canonical.GlobalMetaId, opts, opts.IncludeProofs, out.Warnings)
	if err != nil {
		return nil, ErrAggregationUnavailable
	}
	out.Services = services
	out.Proofs.Services = serviceProofs
	out.Warnings = warnings
}
out.Actions = buildActions(out.Profile.ChatPubkey, len(out.Services), canonical.GlobalMetaId)
```

Add these helpers to `build.go`:

```go
func (a *Aggregator) loadServices(canonicalGlobalMetaId string, opts Options, includeProofs bool, warnings []string) ([]Service, []ServiceProof, []string, error) {
	if a.serviceLister == nil {
		return []Service{}, []ServiceProof{}, warnings, nil
	}
	result, err := a.serviceLister.List(skillservice.ListParams{
		Size:                 opts.ServiceSize,
		ProviderGlobalMetaId: canonicalGlobalMetaId,
		ChainName:            opts.ChainName,
		SortBy:               "updated",
		Order:                "desc",
		IncludeInactive:      opts.IncludeInactiveServices,
	})
	if err != nil {
		return nil, nil, warnings, err
	}
	if result == nil || result.List == nil {
		return []Service{}, []ServiceProof{}, warnings, nil
	}
	services := make([]Service, 0, len(result.List))
	proofs := make([]ServiceProof, 0, len(result.List))
	for _, item := range result.List {
		var proof *ServiceProof
		if includeProofs {
			p := ServiceProof{
				ServiceId:             item.Id,
				PinId:                 item.CurrentPinId,
				SourceServicePinId:    item.SourceServicePinId,
				ProtocolPath:          skillservice.PathSkillService,
				PublisherGlobalMetaId: canonicalGlobalMetaId,
			}
			proof = &p
			proofs = append(proofs, p)
			warnings = append(warnings, "service proof for "+item.Id+" is missing txid/contentHash metadata")
		}
		services = append(services, serviceFromListItem(item, proof))
	}
	return services, proofs, warnings, nil
}

func serviceFromListItem(item skillservice.ServiceListItem, proof *ServiceProof) Service {
	return Service{
		Id:                 item.Id,
		CurrentPinId:       item.CurrentPinId,
		SourceServicePinId: item.SourceServicePinId,
		DisplayName:        item.DisplayName,
		ServiceName:        item.ServiceName,
		Description:        item.Description,
		ServiceIcon:        item.ServiceIcon,
		ProviderSkill:      item.ProviderSkill,
		OutputType:         item.OutputType,
		Price:              item.Price,
		Currency:           item.Currency,
		SettlementKind:     item.SettlementKind,
		PaymentChain:       item.PaymentChain,
		MRC20Ticker:        item.MRC20Ticker,
		MRC20Id:            item.MRC20Id,
		PaymentAddress:     item.PaymentAddress,
		RatingAvg:          item.RatingAvg,
		RatingCount:        item.RatingCount,
		Status:             item.Status,
		Operation:          item.Operation,
		Disabled:           item.Disabled,
		ChainName:          item.ChainName,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		Proof:              proof,
	}
}
```

- [ ] **Step 5: Verify GREEN for Task 4**

Run:

```bash
gofmt -w internal/aggregator/bothomepage
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestBuildHomepageIncludesProviderServices|TestBuildHomepageSkipsServicesWhenDisabled|TestBuildHomepageSuppressesProofsWhenDisabled|TestBuildHomepageProfileDefaultModeAndPartialProofs' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

Run:

```bash
git add internal/aggregator/bothomepage/module.go internal/aggregator/bothomepage/build.go internal/aggregator/bothomepage/build_test.go
git commit -m "feat: include bot homepage services"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing how homepage reuses `skillservice.List` and avoids subjective availability fields.

## Task 5: Add Optional Presence Resolution

**Files:**
- Modify: `internal/aggregator/bothomepage/module.go`
- Create: `internal/aggregator/bothomepage/presence.go`
- Create: `internal/aggregator/bothomepage/presence_test.go`
- Modify: `internal/aggregator/bothomepage/build.go`

- [ ] **Step 1: Write failing presence tests**

Create `internal/aggregator/bothomepage/presence_test.go`:

```go
package bothomepage

import (
	"testing"

	"github.com/metaid-developers/metaso-p2p/internal/presence"
)

type fakeLocalPresence struct {
	items []presence.OnlineEntry
}

func (f fakeLocalPresence) OnlineEntries() []presence.OnlineEntry { return f.items }

type fakeGlobalPresence struct {
	enabled bool
	items   []presence.OnlineEntry
}

func (f fakeGlobalPresence) Enabled() bool { return f.enabled }
func (f fakeGlobalPresence) DefaultScope() string { return "global" }
func (f fakeGlobalPresence) OnlineList(local []presence.OnlineEntry, page int, size int) []presence.OnlineEntry {
	if f.enabled {
		return f.items
	}
	return local
}
func (f fakeGlobalPresence) Stats(local []presence.OnlineEntry) presence.GlobalStats { return presence.GlobalStats{} }

func TestResolvePresenceUnknownWithoutReaders(t *testing.T) {
	agg := &Aggregator{}
	got := agg.resolvePresence(ProfileSnapshot{GlobalMetaId: "idqBot", MetaId: "metaBot"}, true)
	if got.State != "unknown" || got.UpdatedAt != nil || got.Source != "" {
		t.Fatalf("presence = %+v", got)
	}
}

func TestResolvePresenceOnlineFromGlobalReader(t *testing.T) {
	agg := &Aggregator{}
	agg.SetPresenceReaders(fakeLocalPresence{}, fakeGlobalPresence{
		enabled: true,
		items: []presence.OnlineEntry{{
			MetaId: "idqBot", LastSeenAt: 1780760000000, SourceNodeIds: []string{"node-a"},
		}},
	})
	got := agg.resolvePresence(ProfileSnapshot{GlobalMetaId: "idqBot", MetaId: "metaBot"}, true)
	if got.State != "online" {
		t.Fatalf("state = %q", got.State)
	}
	if got.UpdatedAt == nil || *got.UpdatedAt != 1780760000000 {
		t.Fatalf("updatedAt = %v", got.UpdatedAt)
	}
	if got.Source != "federated-presence" {
		t.Fatalf("source = %q", got.Source)
	}
}

func TestResolvePresenceDisabledByQuery(t *testing.T) {
	agg := &Aggregator{}
	agg.SetPresenceReaders(fakeLocalPresence{items: []presence.OnlineEntry{{MetaId: "idqBot"}}}, nil)
	got := agg.resolvePresence(ProfileSnapshot{GlobalMetaId: "idqBot"}, false)
	if got.State != "unknown" {
		t.Fatalf("state = %q", got.State)
	}
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestResolvePresence' -count=1
```

Expected: FAIL because `resolvePresence` does not exist.

- [ ] **Step 3: Add presence readers to the module**

In `internal/aggregator/bothomepage/module.go`, add this import:

```go
	"github.com/metaid-developers/metaso-p2p/internal/presence"
```

Extend `Aggregator` with:

```go
	localPresence  presence.LocalReader
	globalPresence presence.GlobalReader
```

Add this setter:

```go
func (a *Aggregator) SetPresenceReaders(local presence.LocalReader, global presence.GlobalReader) {
	a.localPresence = local
	a.globalPresence = global
}
```

- [ ] **Step 4: Implement presence helper**

Create `internal/aggregator/bothomepage/presence.go`:

```go
package bothomepage

import (
	"strings"

	"github.com/metaid-developers/metaso-p2p/internal/presence"
)

func (a *Aggregator) resolvePresence(profile ProfileSnapshot, include bool) Presence {
	if !include {
		return unknownPresence()
	}
	candidates := identityCandidates(profile)
	local := []presence.OnlineEntry(nil)
	if a != nil && a.localPresence != nil {
		local = a.localPresence.OnlineEntries()
	}
	if a != nil && a.globalPresence != nil && a.globalPresence.Enabled() {
		if hit, ok := findPresence(a.globalPresence.OnlineList(local, 1, 100), candidates); ok {
			return presenceFromEntry(hit, "federated-presence")
		}
	}
	if hit, ok := findPresence(local, candidates); ok {
		return presenceFromEntry(hit, "local-presence")
	}
	return unknownPresence()
}

func identityCandidates(profile ProfileSnapshot) []string {
	values := []string{profile.GlobalMetaId, profile.MetaId, profile.Address}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func findPresence(items []presence.OnlineEntry, candidates []string) (presence.OnlineEntry, bool) {
	for _, item := range items {
		for _, candidate := range candidates {
			if strings.EqualFold(strings.TrimSpace(item.MetaId), candidate) {
				return item, true
			}
		}
	}
	return presence.OnlineEntry{}, false
}

func presenceFromEntry(item presence.OnlineEntry, source string) Presence {
	updatedAt := item.LastSeenAt
	if updatedAt == 0 {
		updatedAt = item.ConnectedAt
	}
	return Presence{State: "online", UpdatedAt: &updatedAt, Source: source}
}
```

In `Build`, replace the initial `Presence: unknownPresence()` behavior after profile conversion with:

```go
out.Presence = a.resolvePresence(*profile, opts.IncludePresence)
```

- [ ] **Step 5: Verify GREEN for Task 5**

Run:

```bash
gofmt -w internal/aggregator/bothomepage
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'TestResolvePresence|TestBuildHomepageProfileDefaultModeAndPartialProofs' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 5**

Run:

```bash
git add internal/aggregator/bothomepage/module.go internal/aggregator/bothomepage/presence.go internal/aggregator/bothomepage/presence_test.go internal/aggregator/bothomepage/build.go
git commit -m "feat: add bot homepage presence hints"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing optional local/federated presence and the non-fatal unknown fallback.

## Task 6: Wire Main And Full Router Acceptance Tests

**Files:**
- Modify: `cmd/metaso-p2p/main.go`
- Modify: `internal/api/router_test.go`

- [ ] **Step 1: Write failing full-router acceptance test**

In `internal/api/router_test.go`, add the import:

```go
	"github.com/metaid-developers/metaso-p2p/internal/aggregator/bothomepage"
	"github.com/metaid-developers/metaso-p2p/internal/aggregator/skillservice"
```

In `fullRouterFixture`, add:

```go
	userAgg        *userinfo.Aggregator
	botHomepageAgg *bothomepage.Aggregator
	skillAgg       *skillservice.Aggregator
```

In `setupFullRouterFixture`, replace the inline userinfo registration with a retained variable:

```go
userAgg := &userinfo.Aggregator{}
if err := reg.Register(userAgg); err != nil {
	t.Fatalf("register userinfo: %v", err)
}
```

Then register skillservice and bothomepage after privatechat:

```go
skillAgg := &skillservice.Aggregator{}
if err := reg.Register(skillAgg); err != nil {
	t.Fatalf("register skillservice: %v", err)
}
botHomepageAgg := &bothomepage.Aggregator{}
if err := reg.Register(botHomepageAgg); err != nil {
	t.Fatalf("register bothomepage: %v", err)
}
skillAgg.SetProfileLookup(skillservice.NewUserInfoLookupAdapter(userAgg))
skillAgg.SetAssetBaseURL("https://file.metaid.io/metafile-indexer/content")
botHomepageAgg.SetProfileLookup(bothomepage.NewUserInfoLookupAdapter(userAgg))
botHomepageAgg.SetServiceLister(skillAgg)
botHomepageAgg.SetAssetBaseURL("https://file.metaid.io/metafile-indexer/content")
```

Return the new fields in `fullRouterFixture`:

```go
return &fullRouterFixture{
	router:          api.SetupRouter(cfg, store, cacheProvider, reg, nil, "test"),
	store:           store,
	userAgg:         userAgg,
	groupAgg:        groupAgg,
	privateAgg:      privateAgg,
	skillAgg:        skillAgg,
	botHomepageAgg: botHomepageAgg,
}
```

Add this test:

```go
func TestRouter_BotHomepageGlobalMetaIDAcceptance(t *testing.T) {
	fixture := setupFullRouterFixture(t)

	if _, err := fixture.botHomepageAgg.HandleBlockPin(nil); err != nil {
		t.Fatalf("bothomepage HandleBlockPin should be no-op: %v", err)
	}
	if _, err := fixture.userAgg.HandleBlockPin(&aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "bot-meta",
		Address:   "18BotHomepage",
		ChainName: "mvc",
		Id:        "init-bot:i0",
	}); err != nil {
		t.Fatalf("userinfo init: %v", err)
	}
	if _, err := fixture.userAgg.HandleBlockPin(&aggregator.PinInscription{
		Path:        "/info/name",
		Operation:   "create",
		MetaId:      "bot-meta",
		Address:     "18BotHomepage",
		ChainName:   "mvc",
		Id:          "name-bot:i0",
		ContentBody: []byte("Homepage Bot"),
	}); err != nil {
		t.Fatalf("userinfo name: %v", err)
	}

	profile, err := fixture.userAgg.LookupByMetaId("bot-meta")
	if err != nil {
		t.Fatalf("LookupByMetaId: %v", err)
	}
	w, body := get(t, fixture.router, "/api/bot-homepage/globalmetaid/"+profile.GlobalMetaID)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", w.Code, w.Body.String())
	}
	data := assertResponseDataKeys(t, body, "schemaVersion", "canonical", "profile", "homepage", "services", "actions", "proofs", "source", "warnings")
	if data["schemaVersion"] != "botHomepage.v1" {
		t.Fatalf("schemaVersion = %v", data["schemaVersion"])
	}
}
```

- [ ] **Step 2: Run focused router test and verify RED**

Run:

```bash
CGO_ENABLED=0 go test ./internal/api -run TestRouter_BotHomepageGlobalMetaIDAcceptance -count=1
```

Expected: FAIL because imports/wiring/main registration are not complete.

- [ ] **Step 3: Wire main**

In `cmd/metaso-p2p/main.go`, add the import:

```go
	"github.com/metaid-developers/metaso-p2p/internal/aggregator/bothomepage"
```

Near the existing aggregator variables, keep `botHomepageAgg` visible for later presence wiring:

```go
var aggRegistry *aggregator.Registry
var userinfoAgg *userinfo.Aggregator
var botHomepageAgg *bothomepage.Aggregator
```

In the aggregator registry block, create and register homepage after skillservice:

```go
botHomepageAgg = &bothomepage.Aggregator{}
if err := aggRegistry.Register(botHomepageAgg); err != nil {
	log.Printf("WARNING: bothomepage aggregator init failed: %v", err)
}
```

After skillservice/privatechat dependency wiring:

```go
botHomepageAgg.SetProfileLookup(bothomepage.NewUserInfoLookupAdapter(userinfoAgg))
botHomepageAgg.SetServiceLister(skillserviceAgg)
botHomepageAgg.SetAssetBaseURL(cfg.BotHub.AssetBaseURL)
```

After federation setup, add:

```go
if botHomepageAgg != nil && socketServer != nil {
	var globalReader presence.GlobalReader
	if federationService != nil {
		globalReader = federationService.GlobalReader()
	}
	botHomepageAgg.SetPresenceReaders(socketServer.Manager(), globalReader)
}
```

Add the import required by that snippet:

```go
	"github.com/metaid-developers/metaso-p2p/internal/presence"
```

- [ ] **Step 4: Verify GREEN for Task 6**

Run:

```bash
gofmt -w cmd/metaso-p2p/main.go internal/api/router_test.go
CGO_ENABLED=0 go test ./internal/api ./cmd/metaso-p2p -run 'TestRouter_BotHomepageGlobalMetaIDAcceptance|TestEnabledBlockIndexChainNames' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 6**

Run:

```bash
git add cmd/metaso-p2p/main.go internal/api/router_test.go
git commit -m "feat: wire bot homepage aggregator"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing real registry wiring and full-router acceptance coverage.

## Task 7: Add API Documentation

**Files:**
- Create: `docs/specs/2026-06-07-bot-homepage-api.md`
- Modify: `README.md`

- [ ] **Step 1: Add docs spec**

Create `docs/specs/2026-06-07-bot-homepage-api.md` with this content:

```markdown
# Bot Homepage Aggregation API

Date: 2026-06-07

## Endpoint

`GET /api/bot-homepage/globalmetaid/:globalMetaId`

Returns one render-ready Bot homepage document for OAC Bot Browser and other MetaID clients.

## Envelope

Success:

```json
{"code":0,"message":"","data":{}}
```

Business errors:

| Code | Meaning |
| --- | --- |
| `40000` | invalid parameter |
| `40400` | bot homepage not found |
| `50000` | aggregation unavailable |

`/api/info/*` keeps its meta-file-system-compatible `code=1` success convention. This endpoint uses native metaso-p2p `code=0`.

## Query

| Parameter | Default | Description |
| --- | --- | --- |
| `includeServices` | `true` | Include provider skill services. |
| `serviceSize` | `20` | Service count cap, maximum `100`. |
| `includeInactiveServices` | `false` | Include revoked, disabled, or status-abnormal services. |
| `includeProofs` | `true` | Include proof summaries when indexed. |
| `includePresence` | `true` | Include online state when a presence reader can answer. |
| `chainName` | empty | Optional service chain filter. |

## Data Contract

`data.schemaVersion` is `botHomepage.v1`.

Required top-level fields:

- `resolvedAt`
- `globalMetaId`
- `canonical`
- `profile`
- `homepage`
- `presence`
- `services`
- `actions`
- `proofs`
- `source`
- `warnings`

## Service Rules

Services reuse `/api/bot-hub/skill-service/list` semantics with:

- `providerGlobalMetaId=<canonical.globalMetaId>`
- `sortBy=updated`
- `order=desc`
- `size=<serviceSize>`
- `includeInactive=<includeInactiveServices>`
- `chainName=<chainName>`

The endpoint does not return subjective fields such as `available`, `canOrder`, `disabledReason`, or `availableReason`.

## Proof Rules

The endpoint emits known `pinId` and `protocolPath` values. It does not fabricate `txid` or `contentHash`. Missing proof metadata returns `proofs.verificationState="partial"` or `"unverified"` and adds warnings.

## Presence Rules

Presence is a hint. If local or federated presence cannot answer confidently, the endpoint returns:

```json
{"state":"unknown","updatedAt":null,"source":""}
```

Presence failure does not fail the homepage response.
```

- [ ] **Step 2: Update README endpoints**

In `README.md`, under API endpoints, add:

```markdown
### Bot Homepage
- `GET /api/bot-homepage/globalmetaid/:globalMetaId` - render-ready Bot homepage aggregation for OAC Bot Browser

See [`docs/specs/2026-06-07-bot-homepage-api.md`](docs/specs/2026-06-07-bot-homepage-api.md) for the full v1 contract.
```

- [ ] **Step 3: Verify docs mention the required endpoint and no forbidden typo**

Run:

```bash
rg -n "/api/bot-homepage/globalmetaid|botHomepage.v1" README.md docs/specs/2026-06-07-bot-homepage-api.md
if rg -n "TSID" README.md docs/specs/2026-06-07-bot-homepage-api.md; then exit 1; fi
```

Expected: endpoint and schemaVersion appear; `TSID` does not appear.

- [ ] **Step 4: Commit Task 7**

Run:

```bash
git add docs/specs/2026-06-07-bot-homepage-api.md README.md
git commit -m "docs: document bot homepage api"
```

After the commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing the API contract, error codes, query flags, and proof/presence rules.

## Task 8: Final Verification And Integration Checks

**Files:**
- All files changed by Tasks 1-7.

- [ ] **Step 1: Run package tests**

Run:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage ./internal/aggregator/userinfo ./internal/aggregator/skillservice ./internal/api ./cmd/metaso-p2p -count=1
```

Expected: PASS.

- [ ] **Step 2: Run whole-repo tests**

Run:

```bash
CGO_ENABLED=0 go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Build binary**

Run:

```bash
CGO_ENABLED=0 go build -o /tmp/metaso-p2p ./cmd/metaso-p2p
```

Expected: PASS and `/tmp/metaso-p2p` exists.

- [ ] **Step 4: Check formatting and forbidden terms**

Run:

```bash
gofmt -w cmd/metaso-p2p/main.go internal/api/router_test.go internal/aggregator/bothomepage internal/aggregator/userinfo/module.go internal/aggregator/userinfo/module_test.go
git diff --check
if rg -n '"available":|"canOrder":|"disabledReason":|"availableReason":|TSID' internal/aggregator/bothomepage; then exit 1; fi
if rg -n "TSID" docs/specs/2026-06-07-bot-homepage-api.md README.md; then exit 1; fi
```

Expected: `git diff --check` passes. The `rg` checks find no forbidden homepage response keys and no `TSID` hits.

- [ ] **Step 5: Inspect final changed files**

Run:

```bash
git status --short
git diff --stat
```

Expected: only the Bot homepage implementation, tests, and docs are changed.

- [ ] **Step 6: Commit any final verification-only cleanup**

If Task 8 required formatting or documentation cleanup, commit only those cleanup files:

```bash
git add <files changed during Task 8>
git commit -m "chore: finalize bot homepage verification"
```

After any Task 8 cleanup commit succeeds, invoke the `metabot-post-buzz` skill and post a development journal entry describing final verification results.

## Acceptance Mapping

- PRD AC 1: Task 3 handler tests and Task 6 router acceptance verify `code=0` and `schemaVersion="botHomepage.v1"`.
- PRD AC 2: Task 3 invalid globalMetaId/query tests verify `code=40000`.
- PRD AC 3: Task 3 unknown Bot test verifies `code=40400`.
- PRD AC 4: Task 2 and Task 6 verify required top-level fields.
- PRD AC 5: Task 4 verifies service filtering by `ProviderGlobalMetaId` through `skillservice.List`.
- PRD AC 6: Task 2 verifies asset URL resolution and `source.contentBaseUrl`.
- PRD AC 7: Task 7 and Task 8 forbid `TSID`.
- PRD AC 8: Task 2 verifies partial proof state plus warnings without fabricated TXIDs.
- PRD AC 9: Task 5 verifies unknown presence fallback and non-fatal presence behavior.
- PRD AC 10: Tasks 1-6 add unit and router tests for success, invalid parameter, not found, services included/excluded, partial proofs, and presence.
- PRD AC 11: Task 7 adds API docs under `docs/specs/`.
