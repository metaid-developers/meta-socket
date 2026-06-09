# Bot Homepage V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the grey-release `botHomepage.v2` response with persona fields, section read models, mempool inclusion, and recent two-month MANAPI backfill while preserving the default v1 endpoint.

**Architecture:** Keep `/api/bot-homepage/globalmetaid/:globalMetaId` as the public boundary and add v2 assembly inside `internal/aggregator/bothomepage`. Move expensive chain-to-product shaping into indexed read models: extend `userinfo`, add `publishedcontent`, improve `skillservice` homepage reads, and route mempool pins through the same folding paths as confirmed pins.

**Tech Stack:** Go, Gin, PebbleDB namespaces, existing `internal/aggregator` registry, existing chain adapters, MANAPI path-list HTTP JSON, `CGO_ENABLED=0 go test`.

---

## Execution Protocol

Use subagent-driven development for implementation:

1. The controller reads this plan and dispatches one fresh implementer subagent per task.
2. The implementer completes only that task, runs the task verification, commits only files it changed, and posts one `metabot-post-buzz` development journal entry for the commit.
3. The controller dispatches a spec-compliance reviewer subagent for the completed task.
4. The controller dispatches a code-quality reviewer subagent after spec compliance passes.
5. The controller proceeds to the next task only after both reviews pass.
6. After all tasks, dispatch one final whole-branch reviewer subagent.

Before Task 1 execution, create an isolated branch or worktree:

```bash
git status --short
git switch -c codex/bot-homepage-v2
```

If the branch already exists, switch to it only after verifying the worktree state is safe:

```bash
git status --short
git switch codex/bot-homepage-v2
```

## File Structure

- Modify `internal/aggregator/userinfo/module.go`: store persona/homepage info paths and add reverse indexes for globalMetaId/address lookup.
- Modify `internal/aggregator/userinfo/module_test.go`: cover persona paths, legacy bio parsing inputs, reverse lookup, and mempool handling.
- Create `internal/aggregator/publishedcontent/types.go`: protocol constants, payload shape, record types, list params, section item helpers.
- Create `internal/aggregator/publishedcontent/db.go`: Pebble keys, save/load helpers, identity/time indexes, pin-to-source map.
- Create `internal/aggregator/publishedcontent/process.go`: create/modify/revoke folding, target resolution, non-binary payload extraction.
- Create `internal/aggregator/publishedcontent/list.go`: section read path, chain filtering, cross-chain aggregation, 6-read/5-return helper.
- Create `internal/aggregator/publishedcontent/module.go`: aggregator interface implementation and optional debug route registration.
- Create `internal/aggregator/publishedcontent/backfill.go`: MANAPI path-list client and two-month backfill runner.
- Create `internal/aggregator/publishedcontent/*_test.go`: focused tests for folding, payload, indexes, backfill parsing, and mempool idempotency.
- Modify `internal/indexer/engine.go`: add periodic mempool polling and route parsed mempool pins through `Registry.RouteMempoolPin`.
- Modify `internal/indexer/engine_test.go`: cover mempool polling, duplicate suppression, and route behavior.
- Modify `internal/config/config.go` and `internal/config/config_test.go`: add mempool and backfill configuration knobs with safe defaults.
- Modify `config.example.toml`: document mempool/backfill knobs.
- Modify `cmd/metaso-p2p/main.go`: configure mempool polling, register `publishedcontent`, wire v2 listers, and run optional v2 backfill.
- Modify `internal/aggregator/skillservice/db.go`, `list.go`, and tests: add provider-global-meta-id updated index and homepage read method.
- Modify `internal/aggregator/bothomepage/types.go`, `query.go`, `build.go`, `api.go`, and tests: add v2 options, types, assembler, and section handling.
- Modify `internal/api/router.go` and router tests for endpoint/query handling only if needed; production aggregator registration currently lives in `cmd/metaso-p2p/main.go`, not the router.
- Modify `docs/superpowers/specs/2026-06-09-bot-homepage-v2-design.md` only if implementation reveals a spec contradiction.

## Task 1: Extend UserInfo Persona Read Model

**Files:**
- Modify: `internal/aggregator/userinfo/module.go`
- Modify: `internal/aggregator/userinfo/module_test.go`
- Test: `internal/aggregator/bothomepage/build_test.go` if the adapter shape changes

- [ ] **Step 1: Write failing userinfo tests**

Add tests in `internal/aggregator/userinfo/module_test.go`:

```go
func TestHandleBlockPin_StoresPersonaInfoPaths(t *testing.T) {
	agg := setupTestAggregator(t)
	metaid := "meta_persona"
	address := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	global := idaddress.EncodeGlobalMetaId(address, "mvc")

	pins := []*aggregator.PinInscription{
		{Id: "init:i0", Path: "/", Operation: "init", MetaId: metaid, Address: address, ChainName: "mvc"},
		{Id: "role:i0", Path: "/info/role", Operation: "create", MetaId: metaid, Address: address, ChainName: "mvc", ContentBody: []byte("Public role")},
		{Id: "soul:i0", Path: "/info/soul", Operation: "create", MetaId: metaid, Address: address, ChainName: "mvc", ContentBody: []byte("Calm soul")},
		{Id: "goal:i0", Path: "/info/goal", Operation: "create", MetaId: metaid, Address: address, ChainName: "mvc", ContentBody: []byte("Help users")},
		{Id: "skills:i0", Path: "/info/chatSkills", Operation: "create", MetaId: metaid, Address: address, ChainName: "mvc", ContentBody: []byte(`{"allowChatSkills":["metabot-post-buzz"]}`)},
		{Id: "llm:i0", Path: "/info/LLM", Operation: "create", MetaId: metaid, Address: address, ChainName: "mvc", ContentBody: []byte(`{"primaryProvider":"deepseek","displayName":"DeepSeek"}`)},
		{Id: "home:i0", Path: "/info/homepage", Operation: "create", MetaId: metaid, Address: address, ChainName: "mvc", ContentBody: []byte(`{"uri":"metaapp://abc","renderer":"html","contentType":"text/html"}`)},
	}
	for _, pin := range pins {
		if _, err := agg.HandleBlockPin(pin); err != nil {
			t.Fatalf("HandleBlockPin(%s): %v", pin.Path, err)
		}
	}

	profile, err := agg.LookupByGlobalMetaId(global)
	if err != nil {
		t.Fatalf("LookupByGlobalMetaId: %v", err)
	}
	if profile.Role != "Public role" || profile.RoleId != "role:i0" {
		t.Fatalf("role not stored: %#v", profile)
	}
	if profile.Soul != "Calm soul" || profile.Goal != "Help users" {
		t.Fatalf("persona text not stored: %#v", profile)
	}
	if profile.ChatSkills != `{"allowChatSkills":["metabot-post-buzz"]}` || profile.ChatSkillsId != "skills:i0" {
		t.Fatalf("chatSkills not stored: %#v", profile)
	}
	if profile.LLM != `{"primaryProvider":"deepseek","displayName":"DeepSeek"}` || profile.LLMId != "llm:i0" {
		t.Fatalf("llm not stored: %#v", profile)
	}
	if profile.Homepage != `{"uri":"metaapp://abc","renderer":"html","contentType":"text/html"}` || profile.HomepageId != "home:i0" {
		t.Fatalf("homepage not stored: %#v", profile)
	}
}

func TestLookupByGlobalMetaId_UsesReverseIndex(t *testing.T) {
	agg := setupTestAggregator(t)
	address := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	global := idaddress.EncodeGlobalMetaId(address, "mvc")
	if _, err := agg.HandleBlockPin(&aggregator.PinInscription{
		Id: "init:i0", Path: "/", Operation: "init", MetaId: "meta_reverse", Address: address, ChainName: "mvc",
	}); err != nil {
		t.Fatal(err)
	}
	profile, err := agg.LookupByGlobalMetaId(global)
	if err != nil {
		t.Fatalf("LookupByGlobalMetaId: %v", err)
	}
	if profile == nil || profile.MetaID != "meta_reverse" {
		t.Fatalf("reverse lookup returned %#v", profile)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/userinfo -run 'TestHandleBlockPin_StoresPersonaInfoPaths|TestLookupByGlobalMetaId_UsesReverseIndex' -count=1
```

Expected: fails because `UserProfile` lacks persona fields and reverse index lookup is not implemented.

- [ ] **Step 3: Implement userinfo fields and indexes**

In `internal/aggregator/userinfo/module.go`, extend `UserProfile`:

```go
Role          string `json:"role,omitempty"`
RoleId        string `json:"roleId,omitempty"`
Soul          string `json:"soul,omitempty"`
SoulId        string `json:"soulId,omitempty"`
Goal          string `json:"goal,omitempty"`
GoalId        string `json:"goalId,omitempty"`
ChatSkills    string `json:"chatSkills,omitempty"`
ChatSkillsId  string `json:"chatSkillsId,omitempty"`
LLM           string `json:"llm,omitempty"`
LLMId         string `json:"llmId,omitempty"`
Homepage      string `json:"homepage,omitempty"`
HomepageId    string `json:"homepageId,omitempty"`
```

Add key prefixes:

```go
globalMetaIdPrefix = "globalmetaid:"
addressPrefix      = "address:"
```

Add switch cases in `HandleBlockPin`:

```go
case path == "/info/role":
	profile.Role = string(pin.ContentBody)
	profile.RoleId = pin.Id
case path == "/info/soul":
	profile.Soul = string(pin.ContentBody)
	profile.SoulId = pin.Id
case path == "/info/goal":
	profile.Goal = string(pin.ContentBody)
	profile.GoalId = pin.Id
case path == "/info/chatSkills":
	profile.ChatSkills = string(pin.ContentBody)
	profile.ChatSkillsId = pin.Id
case path == "/info/LLM":
	profile.LLM = string(pin.ContentBody)
	profile.LLMId = pin.Id
case path == "/info/homepage":
	profile.Homepage = string(pin.ContentBody)
	profile.HomepageId = pin.Id
```

Update `saveProfileAtKey` so it also writes indexes when values are known:

```go
if profile.GlobalMetaID != "" {
	if err := a.store.Set(namespace, globalMetaIdKey(profile.GlobalMetaID), []byte(profile.MetaID)); err != nil {
		return err
	}
}
if profile.Address != "" {
	if err := a.store.Set(namespace, addressKey(profile.Address), []byte(profile.MetaID)); err != nil {
		return err
	}
}
```

Add key helpers:

```go
func globalMetaIdKey(globalMetaId string) []byte {
	return []byte(globalMetaIdPrefix + strings.ToLower(strings.TrimSpace(globalMetaId)))
}

func addressKey(address string) []byte {
	return []byte(addressPrefix + strings.ToLower(strings.TrimSpace(address)))
}
```

Update `findProfileByGlobalMetaId` and `findProfileByAddress` to consult the index first, then fall back to the existing scan for backwards compatibility.

- [ ] **Step 4: Run focused tests**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/userinfo -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit and post buzz**

```bash
git status --short
git add internal/aggregator/userinfo/module.go internal/aggregator/userinfo/module_test.go internal/aggregator/bothomepage/build_test.go
git commit -m "feat: extend userinfo persona model"
```

Then use `metabot-post-buzz` to post a development journal entry describing the userinfo persona fields, reverse indexes, and test results.

## Task 2: Add Mempool Polling And Routing

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `config.example.toml`
- Modify: `internal/indexer/engine.go`
- Modify: `internal/indexer/engine_test.go`
- Modify: `cmd/metaso-p2p/main.go`

- [ ] **Step 1: Write failing mempool routing tests**

In `internal/indexer/engine_test.go`, add a mock registry aggregator that records mempool pins, and add:

```go
func TestEnginePollsMempoolAndRoutesPins(t *testing.T) {
	store := newTestPebbleStore(t)
	cacheProvider := cache.NewProvider()
	reg := aggregator.NewRegistry(store, cacheProvider)
	rec := &recordingAggregator{name: "recorder"}
	if err := reg.Register(rec); err != nil {
		t.Fatal(err)
	}

	chain := &mockChain{name: "mvc", mempoolTxs: []any{"tx1"}}
	indexer := &mockIndexer{
		mempoolPins: []*aggregator.PinInscription{{
			Id: "tx1i0", Path: "/protocols/simplebuzz", Operation: "create", ChainName: "mvc",
			GlobalMetaId: "idq1", MetaId: "meta1", Address: "addr1", ContentBody: []byte(`{"content":"hello"}`),
		}},
	}
	engine := NewEngine(store, reg)
	if err := engine.RegisterChain(chain, indexer, 0); err != nil {
		t.Fatal(err)
	}

	engine.pollMempoolOnce()

	if len(rec.mempoolPins) != 1 {
		t.Fatalf("mempool pins routed = %d, want 1", len(rec.mempoolPins))
	}
	if rec.mempoolPins[0].Id != "tx1i0" {
		t.Fatalf("unexpected pin routed: %#v", rec.mempoolPins[0])
	}
}

func TestEngineMempoolPollDeduplicatesTransactionIDs(t *testing.T) {
	store := newTestPebbleStore(t)
	cacheProvider := cache.NewProvider()
	reg := aggregator.NewRegistry(store, cacheProvider)
	rec := &recordingAggregator{name: "recorder"}
	if err := reg.Register(rec); err != nil {
		t.Fatal(err)
	}
	chain := &mockChain{name: "mvc", mempoolTxs: []any{"tx1"}}
	indexer := &mockIndexer{mempoolPins: []*aggregator.PinInscription{{Id: "tx1i0", Path: "/protocols/simplebuzz", Operation: "create", ChainName: "mvc"}}}
	engine := NewEngine(store, reg)
	if err := engine.RegisterChain(chain, indexer, 0); err != nil {
		t.Fatal(err)
	}

	engine.pollMempoolOnce()
	engine.pollMempoolOnce()

	if len(rec.mempoolPins) != 1 {
		t.Fatalf("duplicate mempool routes = %d, want 1", len(rec.mempoolPins))
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/indexer -run 'TestEnginePollsMempoolAndRoutesPins|TestEngineMempoolPollDeduplicatesTransactionIDs' -count=1
```

Expected: fails because `pollMempoolOnce` and dedupe state do not exist.

- [ ] **Step 3: Add config knobs**

Add config fields to the existing `ZMQConfig`. Do not introduce a new `IndexerConfig`; current code has `BlockIndexConfig` and `ZMQConfig`, and `config.example.toml` already describes `[zmq]` as mempool monitoring.

```go
type ZMQConfig struct {
	Enabled               bool           `json:"enabled"`
	MempoolPollingEnabled bool           `json:"mempoolPollingEnabled"`
	MempoolPollInterval   time.Duration  `json:"mempoolPollInterval"`
	MempoolDedupeTTL      time.Duration  `json:"mempoolDedupeTTL"`
	BTC                   ChainZMQConfig `json:"btc"`
	MVC                   ChainZMQConfig `json:"mvc"`
	DOGE                  ChainZMQConfig `json:"doge"`
	OPCAT                 ChainZMQConfig `json:"opcat"`
}
```

Defaults:

```go
MempoolPollingEnabled: true,
MempoolPollInterval:   10 * time.Second,
MempoolDedupeTTL:      30 * time.Minute,
```

Update the existing `[zmq]` table in `config.example.toml`; do not create a second `[zmq]` table. Clarify that `[zmq].enabled` controls future ZMQ subscription behavior, while `mempoolPollingEnabled` controls the RPC polling loop implemented in this task.

```toml
[zmq]
mempoolPollingEnabled = true
mempoolPollInterval = "10s"
mempoolDedupeTTL = "30m"
```

Add env loading:

```go
applyBoolEnv("METASO_P2P_ZMQ_MEMPOOL_POLLING_ENABLED", &cfg.ZMQ.MempoolPollingEnabled)
applyDurationEnv("METASO_P2P_ZMQ_MEMPOOL_POLL_INTERVAL", &cfg.ZMQ.MempoolPollInterval)
applyDurationEnv("METASO_P2P_ZMQ_MEMPOOL_DEDUPE_TTL", &cfg.ZMQ.MempoolDedupeTTL)
```

- [ ] **Step 4: Implement polling in `internal/indexer/engine.go`**

Add engine fields:

```go
mempoolPollingEnabled bool
mempoolPollInterval   time.Duration
mempoolDedupeTTL      time.Duration
mempoolSeen           map[string]time.Time
mempoolMu             sync.Mutex
```

Initialize them in `NewEngine`:

```go
mempoolPollingEnabled: true,
mempoolPollInterval:   10 * time.Second,
mempoolDedupeTTL:      30 * time.Minute,
mempoolSeen:           make(map[string]time.Time),
```

Add a configuration method because the current `NewEngine(store, registry)` signature does not accept config:

```go
func (e *Engine) ConfigureMempoolPolling(enabled bool, pollInterval, dedupeTTL time.Duration) {
	e.mempoolPollingEnabled = enabled
	if pollInterval > 0 {
		e.mempoolPollInterval = pollInterval
	}
	if dedupeTTL > 0 {
		e.mempoolDedupeTTL = dedupeTTL
	}
}
```

In `cmd/metaso-p2p/main.go`, call it immediately after `indexer.NewEngine(store, aggRegistry)`:

```go
idxEngine.ConfigureMempoolPolling(
	cfg.ZMQ.MempoolPollingEnabled,
	cfg.ZMQ.MempoolPollInterval,
	cfg.ZMQ.MempoolDedupeTTL,
)
```

Add:

```go
func (e *Engine) pollMempoolOnce() {
	e.mu.RLock()
	entries := make([]*chainEntry, 0, len(e.chains))
	for _, entry := range e.chains {
		entries = append(entries, entry)
	}
	e.mu.RUnlock()

	now := time.Now()
	for _, entry := range entries {
		txList, err := entry.chain.GetMempoolTransactionList()
		if err != nil {
			log.Printf("[indexer] %s mempool: list error: %v", entry.chain.Name(), err)
			continue
		}
		if len(txList) == 0 {
			continue
		}
		pins, txIDs, err := entry.indexer.CatchMempoolPins(txList)
		if err != nil {
			log.Printf("[indexer] %s mempool: parse error: %v", entry.chain.Name(), err)
			continue
		}
		if len(txIDs) > 0 {
			pins = e.filterSeenMempoolPins(entry.chain.Name(), pins, txIDs, now)
		}
		for _, pin := range pins {
			e.registry.RouteMempoolPin(pin)
		}
	}
}
```

Add `filterSeenMempoolPins` keyed by `chainName + ":" + txID`, with pruning by `e.mempoolDedupeTTL`.

Update `zmqLoop` or the engine lifecycle loop so mempool polling runs periodically while the engine context is active. The loop should return immediately when `e.mempoolPollingEnabled` is false. It should use `e.mempoolPollInterval`, run one immediate `pollMempoolOnce()` on start, then tick until context cancellation.

- [ ] **Step 5: Run focused tests**

```bash
CGO_ENABLED=0 go test ./internal/indexer ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit and post buzz**

```bash
git status --short
git add internal/config/config.go internal/config/config_test.go config.example.toml internal/indexer/engine.go internal/indexer/engine_test.go cmd/metaso-p2p/main.go
git commit -m "feat: route mempool pins through aggregators"
```

Then use `metabot-post-buzz` to post a development journal entry describing mempool polling, dedupe behavior, and tests.

## Task 3: Add PublishedContent Aggregator

**Files:**
- Create: `internal/aggregator/publishedcontent/types.go`
- Create: `internal/aggregator/publishedcontent/db.go`
- Create: `internal/aggregator/publishedcontent/process.go`
- Create: `internal/aggregator/publishedcontent/list.go`
- Create: `internal/aggregator/publishedcontent/module.go`
- Create: `internal/aggregator/publishedcontent/process_test.go`
- Create: `internal/aggregator/publishedcontent/list_test.go`

- [ ] **Step 1: Write failing folding and payload tests**

In `process_test.go`, cover:

```go
func TestProcessCreateModifyRevokeFoldsCurrentRecord(t *testing.T) {
	agg := setupTestAggregator(t)
	create := makeContentPin("create:i0", "/protocols/metaapp", "create", "", `{"appName":"A","intro":"one"}`)
	modify := makeContentPin("modify:i0", "/protocols/metaapp@create:i0", "modify", "", `{"appName":"A2","intro":"two"}`)
	revoke := makeContentPin("revoke:i0", "/protocols/metaapp@modify:i0", "revoke", "", "")

	mustProcess(t, agg, create, false)
	mustProcess(t, agg, modify, false)
	rec := mustLoadRecord(t, agg, "mvc", PathMetaApp, "create:i0")
	if rec.SourcePinId != "create:i0" || rec.CurrentPinId != "modify:i0" {
		t.Fatalf("version chain not folded: %#v", rec)
	}
	if rec.PayloadJSON["appName"] != "A2" {
		t.Fatalf("payload not updated: %#v", rec.PayloadJSON)
	}

	mustProcess(t, agg, revoke, false)
	rec = mustLoadRecord(t, agg, "mvc", PathMetaApp, "create:i0")
	if rec.Operation != OperationRevoke || !rec.Hidden {
		t.Fatalf("revoke not hidden: %#v", rec)
	}
}

func TestPayloadFallsBackToContentSummary(t *testing.T) {
	agg := setupTestAggregator(t)
	pin := makeContentPin("skill:i0", PathMetaBotSkill, "create", "", "")
	pin.ContentSummary = `{"name":"weather","description":"Query weather"}`
	mustProcess(t, agg, pin, false)
	rec := mustLoadRecord(t, agg, "mvc", PathMetaBotSkill, "skill:i0")
	if rec.PayloadJSON["name"] != "weather" {
		t.Fatalf("contentSummary fallback failed: %#v", rec.PayloadJSON)
	}
}

func TestBinaryPayloadIsNotExposed(t *testing.T) {
	agg := setupTestAggregator(t)
	pin := makeContentPin("bin:i0", PathSimpleBuzz, "create", "", "\x00\x01\x02")
	pin.ContentType = "application/octet-stream"
	mustProcess(t, agg, pin, false)
	rec := mustLoadRecord(t, agg, "mvc", PathSimpleBuzz, "bin:i0")
	if rec.PayloadText != "" || rec.PayloadJSON != nil {
		t.Fatalf("binary payload exposed: %#v", rec)
	}
}
```

In `list_test.go`, cover cross-chain and buzz sorting:

```go
func TestListByIdentityCrossChainReadsSixReturnsFive(t *testing.T) {
	agg := setupTestAggregator(t)
	for i := 0; i < 7; i++ {
		chain := "mvc"
		if i%2 == 1 {
			chain = "btc"
		}
		pin := makeContentPin(fmt.Sprintf("buzz-%d:i0", i), PathSimpleBuzz, "create", "", fmt.Sprintf(`{"content":"%d"}`, i))
		pin.ChainName = chain
		pin.Timestamp = int64(100 + i)
		mustProcess(t, agg, pin, false)
	}
	result, err := agg.List(ListParams{ProtocolPath: PathSimpleBuzz, PublisherGlobalMetaId: "idq-publisher", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 5 || !result.HasMore {
		t.Fatalf("list result = len %d hasMore %v", len(result.Items), result.HasMore)
	}
}

func TestBuzzModifyDoesNotChangeCreatedSort(t *testing.T) {
	agg := setupTestAggregator(t)
	old := makeContentPin("old:i0", PathSimpleBuzz, "create", "", `{"content":"old"}`)
	old.Timestamp = 100
	newer := makeContentPin("new:i0", PathSimpleBuzz, "create", "", `{"content":"new"}`)
	newer.Timestamp = 200
	mod := makeContentPin("mod:i0", PathSimpleBuzz+"@old:i0", "modify", "", `{"content":"old updated"}`)
	mod.Timestamp = 300
	mustProcess(t, agg, old, false)
	mustProcess(t, agg, newer, false)
	mustProcess(t, agg, mod, false)

	result, err := agg.List(ListParams{ProtocolPath: PathSimpleBuzz, PublisherGlobalMetaId: "idq-publisher", Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Items[0].SourcePinId != "new:i0" || result.Items[1].SourcePinId != "old:i0" {
		t.Fatalf("buzz sorting changed after modify: %#v", result.Items)
	}
	if result.Items[1].PayloadJSON["content"] != "old updated" {
		t.Fatalf("modified payload not shown: %#v", result.Items[1])
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/publishedcontent -count=1
```

Expected: package does not exist.

- [ ] **Step 3: Implement types and module**

Create `types.go` with constants:

```go
const (
	PathSimpleBuzz   = "/protocols/simplebuzz"
	PathMetaApp      = "/protocols/metaapp"
	PathMetaBotSkill = "/protocols/metabot-skill"

	OperationCreate = "create"
	OperationModify = "modify"
	OperationRevoke = "revoke"
)
```

Define `Record`, `ListParams`, `ListResult`, and `SectionItem` with source/current pin ids, identity fields, timestamps, content type, payload text/JSON, hidden state, and proof metadata.

Create `module.go` with `Name() == "publishedcontent"`, `HandleBlockPin`, `HandleMempoolPin`, and `RegisterRoutes`. Both block and mempool handlers call the same `processPin(pin, isMempool bool)` path.

- [ ] **Step 4: Implement Pebble keys and folding**

Use keys:

```text
record:<chain>:<protocol>:<sourcePinId>
pin_to_source:<chain>:<pinId>
by_global:<protocol>:<globalMetaId>:<sortKey>:<chain>:<sourcePinId>
by_metaid:<protocol>:<metaid>:<sortKey>:<chain>:<sourcePinId>
by_address:<protocol>:<address>:<sortKey>:<chain>:<sourcePinId>
```

Use inverted 64-bit timestamps for newest-first prefix scans. For buzzes, index by `CreatedAt`; for metaapps and metabot-skill, index by `UpdatedAt` with `CreatedAt` fallback.

Target resolution:

```go
func targetPinID(pin *aggregator.PinInscription) string {
	if id := pinTargetFromPath(pin.Path); id != "" && id != pin.Id {
		return id
	}
	return strings.TrimPrefix(strings.TrimSpace(pin.OriginalId), "@")
}
```

If modify/revoke targets another modify pin, resolve through `pin_to_source` until the source record is found.

- [ ] **Step 5: Implement payload extraction**

Add `ContentSummary string` to `aggregator.PinInscription`. MANAPI path-list data often carries displayable protocol payload in `contentSummary` when `contentBody` is empty, and both live indexing tests and backfill use the same field. Implement:

```go
func extractPayload(pin *aggregator.PinInscription) (text string, jsonPayload map[string]any, exposed bool) {
	raw := bytes.TrimSpace(pin.ContentBody)
	if len(raw) == 0 {
		raw = []byte(strings.TrimSpace(pin.ContentSummary))
	}
	if len(raw) == 0 || isBinaryPayload(pin.ContentType, raw) {
		return "", nil, false
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) == nil {
		return "", payload, true
	}
	return string(raw), nil, true
}
```

Treat content types containing `octet-stream`, `zip`, `image/`, `audio/`, `video/`, or NUL bytes as binary.

- [ ] **Step 6: Run focused tests**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/publishedcontent ./internal/aggregator -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit and post buzz**

```bash
git status --short
git add internal/aggregator/aggregator.go internal/aggregator/publishedcontent
git commit -m "feat: add published content aggregator"
```

Then use `metabot-post-buzz` to post a development journal entry describing protocol paths, folding, payload rules, cross-chain reads, and tests.

## Task 4: Add MANAPI Two-Month Backfill

**Files:**
- Create: `internal/aggregator/publishedcontent/backfill.go`
- Create: `internal/aggregator/publishedcontent/backfill_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `config.example.toml`

- [ ] **Step 1: Write failing backfill tests**

Add tests:

```go
func TestBackfillStopsAtTwoMonthCutoff(t *testing.T) {
	newer := manapiPinForTest("new:i0", PathSimpleBuzz, "create", time.Now().AddDate(0, -1, 0).Unix(), `{"content":"new"}`)
	older := manapiPinForTest("old:i0", PathSimpleBuzz, "create", time.Now().AddDate(0, -3, 0).Unix(), `{"content":"old"}`)
	server := newBackfillMANAPIServer(t, map[string][]manapiPin{PathSimpleBuzz: {newer, older}})
	agg := setupTestAggregator(t)
	client := NewBackfillClient(server.URL, server.Client())

	err := agg.Backfill(context.Background(), BackfillOptions{
		Client: client,
		Paths: []string{PathSimpleBuzz},
		Since: time.Now().AddDate(0, -2, 0),
		PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := agg.List(ListParams{ProtocolPath: PathSimpleBuzz, PublisherGlobalMetaId: "idq-publisher", Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].SourcePinId != "new:i0" {
		t.Fatalf("cutoff not respected: %#v", result.Items)
	}
}

func TestBackfillUsesContentSummaryFallback(t *testing.T) {
	pin := manapiPinForTest("skill:i0", PathMetaBotSkill, "create", time.Now().Unix(), "")
	pin.ContentSummary = `{"name":"writer","description":"Write text"}`
	server := newBackfillMANAPIServer(t, map[string][]manapiPin{PathMetaBotSkill: {pin}})
	agg := setupTestAggregator(t)
	client := NewBackfillClient(server.URL, server.Client())

	err := agg.Backfill(context.Background(), BackfillOptions{
		Client: client,
		Paths: []string{PathMetaBotSkill},
		Since: time.Now().AddDate(0, -2, 0),
		PageSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := agg.List(ListParams{ProtocolPath: PathMetaBotSkill, PublisherGlobalMetaId: "idq-publisher", Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Items[0].PayloadJSON["name"] != "writer" {
		t.Fatalf("summary fallback missing: %#v", result.Items[0])
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/publishedcontent -run Backfill -count=1
```

Expected: fails because backfill client is missing.

- [ ] **Step 3: Implement MANAPI client**

Create a client that calls:

```text
GET {baseURL}/pin/path/list?cursor={cursor}&size={size}&path={path}
```

Decode both `data.list` and top-level `list` response shapes. Decode fields used by this feature:

```go
type manapiPin struct {
	ID             string             `json:"id"`
	Path           string             `json:"path"`
	OriginalPath   string             `json:"originalPath"`
	Operation      string             `json:"operation"`
	ContentType    string             `json:"contentType"`
	ContentBody    manapiContentBytes `json:"contentBody"`
	ContentSummary string             `json:"contentSummary"`
	MetaId         string             `json:"metaId"`
	GlobalMetaId   string             `json:"globalMetaId"`
	Address        string             `json:"address"`
	CreateMetaId   string             `json:"createMetaId"`
	CreateAddress  string             `json:"createAddress"`
	ChainName      string             `json:"chainName"`
	Timestamp      int64              `json:"timestamp"`
	GenesisHeight  int64              `json:"genesisHeight"`
	OriginalId     string             `json:"originalId"`
}
```

Use the existing `internal/federation/discovery.go` `manapiContentBytes` behavior as the local pattern.

- [ ] **Step 4: Implement two-month processing**

Backfill options:

```go
type BackfillOptions struct {
	Client   *BackfillClient
	Paths    []string
	Since    time.Time
	PageSize int
}
```

Default paths:

```go
[]string{PathSimpleBuzz, PathMetaApp, PathMetaBotSkill}
```

Process newest-first pages until every returned pin on a page is older than `Since` or the page is empty. Convert each MANAPI pin into `aggregator.PinInscription` and call `processPin(pin, false)`.

- [ ] **Step 5: Add config knobs**

Add config:

```toml
[botHomepageV2Backfill]
enabled = false
lookback = "1440h"
timeout = "2m"
pageSize = 100
manapiBaseUrl = "https://manapi.metaid.io"
```

Default `enabled=false` so normal test/server startup does not unexpectedly hit MANAPI.

- [ ] **Step 6: Run focused tests**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/publishedcontent ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit and post buzz**

```bash
git status --short
git add internal/aggregator/publishedcontent/backfill.go internal/aggregator/publishedcontent/backfill_test.go internal/config/config.go internal/config/config_test.go config.example.toml
git commit -m "feat: add bot homepage v2 backfill"
```

Then use `metabot-post-buzz` to post a development journal entry describing two-month cutoff, MANAPI fields, contentSummary fallback, and tests.

## Task 5: Add SkillService Homepage Read Path

**Files:**
- Modify: `internal/aggregator/skillservice/db.go`
- Modify: `internal/aggregator/skillservice/list.go`
- Modify: `internal/aggregator/skillservice/types.go`
- Modify: `internal/aggregator/skillservice/list_test.go`

- [ ] **Step 1: Write failing homepage list tests**

Add:

```go
func TestListHomepageByProviderGlobalMetaIdReadsNewestSix(t *testing.T) {
	f := setupListFixture(t)
	for i := 0; i < 7; i++ {
		pin := makeServicePin(fmt.Sprintf("svc-%d:i0", i), OperationCreate)
		pin.GlobalMetaId = "idq-provider"
		pin.Timestamp = int64(100 + i)
		mustProcessServicePin(t, f.agg, pin)
	}
	result, err := f.agg.ListHomepageByProvider(skillservice.HomepageListParams{
		ProviderGlobalMetaId: "idq-provider",
		Size: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.List) != 5 || !result.HasMore {
		t.Fatalf("homepage result len=%d hasMore=%v", len(result.List), result.HasMore)
	}
}

func TestListHomepageByProviderCrossChain(t *testing.T) {
	f := setupListFixture(t)
	mvc := makeServicePin("mvc:i0", OperationCreate)
	mvc.ChainName = "mvc"
	mvc.GlobalMetaId = "idq-provider"
	mvc.Timestamp = 100
	btc := makeServicePin("btc:i0", OperationCreate)
	btc.ChainName = "btc"
	btc.GlobalMetaId = "idq-provider"
	btc.Timestamp = 200
	mustProcessServicePin(t, f.agg, mvc)
	mustProcessServicePin(t, f.agg, btc)

	result, err := f.agg.ListHomepageByProvider(skillservice.HomepageListParams{
		ProviderGlobalMetaId: "idq-provider",
		Size: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.List) != 2 || result.List[0].ChainName != "btc" {
		t.Fatalf("cross-chain order wrong: %#v", result.List)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/skillservice -run 'HomepageByProvider' -count=1
```

Expected: fails because homepage list API/index does not exist.

- [ ] **Step 3: Add global provider indexes**

Add Pebble keys:

```text
service_by_provider_global:<providerGlobalMetaId>:<invertedUpdatedAt>:<chainName>:<sourceServicePinId>
service_by_provider_global_chain:<providerGlobalMetaId>:<chainName>:<invertedUpdatedAt>:<sourceServicePinId>
```

Update `saveService` to delete stale old index entries and write new index entries when `ProviderGlobalMetaId` is non-empty.

- [ ] **Step 4: Implement homepage read method**

Add:

```go
type HomepageListParams struct {
	ProviderGlobalMetaId string
	ChainName            string
	Size                 int
	IncludeInactive      bool
}

type HomepageListResult struct {
	List    []ServiceListItem
	HasMore bool
}
```

Implement `ListHomepageByProvider` to scan provider-global indexes, read `Size+1`, filter inactive unless requested, sort by updated desc from key order, return `Size` and `HasMore`.

- [ ] **Step 5: Run focused tests**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/skillservice -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit and post buzz**

```bash
git status --short
git add internal/aggregator/skillservice/db.go internal/aggregator/skillservice/list.go internal/aggregator/skillservice/types.go internal/aggregator/skillservice/list_test.go
git commit -m "feat: add skill service homepage index"
```

Then use `metabot-post-buzz` to post a development journal entry describing provider-global homepage reads and tests.

## Task 6: Build Bot Homepage V2 Assembler

**Files:**
- Modify: `internal/aggregator/bothomepage/types.go`
- Modify: `internal/aggregator/bothomepage/query.go`
- Modify: `internal/aggregator/bothomepage/build.go`
- Modify: `internal/aggregator/bothomepage/api.go`
- Modify: `internal/aggregator/bothomepage/module.go`
- Modify: `internal/aggregator/bothomepage/userinfo_adapter.go`
- Modify: `internal/aggregator/bothomepage/build_test.go`
- Modify: `internal/aggregator/bothomepage/api_test.go`

- [ ] **Step 1: Write failing v2 query and build tests**

Add tests:

```go
func TestParseOptionsVersionV2(t *testing.T) {
	opts := ParseOptions(url.Values{"version": {"v2"}, "chainName": {""}})
	if opts.Version != "v2" || opts.ChainName != "" {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestBuildV2ParsesLegacyBioIntoPersona(t *testing.T) {
	legacy := `{"role":"Agent role","soul":"Warm","goal":"Help","llm":"deepseek","allowChatSkills":["metabot-post-buzz"]}`
	agg := newTestHomepageAggregator(t, &ProfileSnapshot{
		GlobalMetaId: "idq-bot",
		MetaId: "meta-bot",
		Address: "addr-bot",
		ChainName: "mvc",
		Name: "Bot",
		Bio: legacy,
		BioId: "bio:i0",
	})
	out, err := agg.Build("idq-bot", Options{Version: "v2", IncludeSections: true, IncludeProofs: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.SchemaVersion != "botHomepage.v2" {
		t.Fatalf("schema = %s", out.SchemaVersion)
	}
	if out.Profile.Bio != "" || out.Persona.Role != "Agent role" {
		t.Fatalf("legacy bio not split: profile=%#v persona=%#v", out.Profile, out.Persona)
	}
	if out.Homepage.Summary != "Agent role" {
		t.Fatalf("summary fallback = %q", out.Homepage.Summary)
	}
}

func TestBuildV2SectionsAreOptional(t *testing.T) {
	agg := newTestHomepageAggregator(t, profileForGlobal("idq-bot"))
	agg.SetPublishedContentLister(&failingPublishedContentLister{})
	out, err := agg.Build("idq-bot", Options{Version: "v2", IncludeSections: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Sections) == 0 {
		t.Fatal("sections missing")
	}
	if !containsWarning(out.Warnings, "metaapps section unavailable") {
		t.Fatalf("warning missing: %#v", out.Warnings)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -run 'V2|VersionV2' -count=1
```

Expected: fails because v2 types and assembly do not exist.

- [ ] **Step 3: Add v2 types**

Add or extend types:

```go
type Persona struct {
	Role            string       `json:"role"`
	RolePinId       string       `json:"rolePinId"`
	Soul            string       `json:"soul"`
	SoulPinId       string       `json:"soulPinId"`
	Goal            string       `json:"goal"`
	GoalPinId       string       `json:"goalPinId"`
	ChatSkills      ChatSkills   `json:"chatSkills"`
	LLM             LLM          `json:"llm"`
	LegacyBioParsed bool         `json:"legacyBioParsed"`
}

type Section struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Kind     string        `json:"kind"`
	Items    []SectionItem `json:"items"`
	Limit    int           `json:"limit"`
	Returned int           `json:"returned"`
	HasMore  bool          `json:"hasMore"`
	More     MoreLink      `json:"more"`
}
```

Keep v1 fields stable. If the existing `Data` type becomes crowded, introduce `DataV2` and let the API handler write it for v2 while v1 keeps `Data`.

- [ ] **Step 4: Add lister interfaces**

Add interfaces in `build.go` or a new focused file:

```go
type HomepageServiceLister interface {
	ListHomepageByProvider(skillservice.HomepageListParams) (*skillservice.HomepageListResult, error)
}

type PublishedContentLister interface {
	List(publishedcontent.ListParams) (*publishedcontent.ListResult, error)
}
```

Add setters to `module.go`:

```go
func (a *Aggregator) SetHomepageServiceLister(lister HomepageServiceLister) { a.homepageServiceLister = lister }
func (a *Aggregator) SetPublishedContentLister(lister PublishedContentLister) { a.publishedContentLister = lister }
```

- [ ] **Step 5: Implement persona and homepage summary**

Rules:

1. Plain `/info/bio` goes to `profile.bio`.
2. JSON `/info/bio` maps behavior fields to persona and leaves `profile.bio=""`.
3. Preferred `/info/role`, `/info/soul`, `/info/goal`, `/info/chatSkills`, and `/info/LLM` override legacy bio fields.
4. `homepage.summary` uses `profile.bio`, then `persona.role`, then `persona.goal`, then empty.

Implement JSON parsing with `map[string]any` and typed helper functions so malformed legacy JSON simply remains plain bio text.

- [ ] **Step 6: Implement section assembly**

For each section:

```go
const homepageSectionLimit = 5
readSize := homepageSectionLimit + 1
```

Services use `HomepageServiceLister`. MetaAPPs, Bot skills, and Buzzes use `PublishedContentLister` with protocol path, canonical identity, chainName, and size 6.

On one section error, return an empty section:

```json
{
  "id": "metaapps",
  "title": "MetaAPPs",
  "kind": "metaapps",
  "items": [],
  "limit": 5,
  "returned": 0,
  "hasMore": false,
  "more": {"label":"More","enabled":false}
}
```

Add warning text exactly:

```text
metaapps section unavailable
skills section unavailable
buzzes section unavailable
services section unavailable
```

- [ ] **Step 7: Update API handler**

`version=v2` or `schemaVersion=botHomepage.v2` returns v2. Empty/default version returns existing v1. Business errors remain HTTP 200 JSON envelopes.

- [ ] **Step 8: Run focused tests**

```bash
CGO_ENABLED=0 go test ./internal/aggregator/bothomepage -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit and post buzz**

```bash
git status --short
git add internal/aggregator/bothomepage
git commit -m "feat: add bot homepage v2 assembler"
```

Then use `metabot-post-buzz` to post a development journal entry describing v2 schema selection, persona parsing, sections, warnings, and tests.

## Task 7: Wire Aggregators And Router Acceptance

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/router_test.go`
- Modify: `cmd/metaso-p2p/main.go`
- Add: `docs/specs/2026-06-09-bot-homepage-v2-api.md`

- [ ] **Step 1: Write failing full-router acceptance tests**

Add router test:

```go
func TestRouterBotHomepageV2IncludesSections(t *testing.T) {
	fixture := setupRouterFixture(t)
	seedBotProfile(t, fixture.userAgg, "idq-bot")
	seedPublishedContent(t, fixture.publishedAgg, "idq-bot")

	w, body := get(t, fixture.router, "/api/bot-homepage/globalmetaid/idq-bot?version=v2")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if body["code"] != float64(0) {
		t.Fatalf("code = %#v body=%s", body["code"], w.Body.String())
	}
	data := body["data"].(map[string]any)
	if data["schemaVersion"] != "botHomepage.v2" {
		t.Fatalf("schema = %#v", data["schemaVersion"])
	}
	if _, ok := data["sections"].([]any); !ok {
		t.Fatalf("sections missing: %#v", data)
	}
}

func TestRouterBotHomepageDefaultStillV1(t *testing.T) {
	fixture := setupRouterFixture(t)
	seedBotProfile(t, fixture.userAgg, "idq-bot")
	w, body := get(t, fixture.router, "/api/bot-homepage/globalmetaid/idq-bot")
	if w.Code != http.StatusOK || body["code"] != float64(0) {
		t.Fatalf("bad response status=%d body=%s", w.Code, w.Body.String())
	}
	data := body["data"].(map[string]any)
	if data["schemaVersion"] != "botHomepage.v1" {
		t.Fatalf("default schema = %#v", data["schemaVersion"])
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
CGO_ENABLED=0 go test ./internal/api -run 'BotHomepageV2|DefaultStillV1' -count=1
```

Expected: fails because publishedcontent is not wired.

- [ ] **Step 3: Register publishedcontent, listers, and optional backfill**

In `cmd/metaso-p2p/main.go`, import `internal/aggregator/publishedcontent`, register `publishedcontent.Aggregator` alongside existing aggregators, and wire it into `bothomepage`. This is the production registration point; do not assume `internal/api/router.go` owns aggregator construction.

```go
var publishedAgg *publishedcontent.Aggregator
publishedCandidate := &publishedcontent.Aggregator{}
if err := aggRegistry.Register(publishedCandidate); err != nil {
	log.Printf("WARNING: publishedcontent aggregator init failed: %v", err)
} else {
	publishedAgg = publishedCandidate
}
```

Keep existing:

```go
botHomepageAgg.SetProfileLookup(bothomepage.NewUserInfoLookupAdapter(userinfoAgg))
skillserviceAgg.SetProfileLookup(skillservice.NewUserInfoLookupAdapter(userinfoAgg))
```

Then wire:

```go
botHomepageAgg.SetHomepageServiceLister(skillserviceAgg)
if publishedAgg != nil {
	botHomepageAgg.SetPublishedContentLister(publishedAgg)
}
```

If Task 4 added `cfg.BotHomepageV2Backfill`, start the two-month backfill only when `cfg.BotHomepageV2Backfill.Enabled` is true. Default must remain false so normal startup and tests do not hit MANAPI.

```go
if cfg.BotHomepageV2Backfill.Enabled && publishedAgg != nil {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.BotHomepageV2Backfill.Timeout)
		defer cancel()
		err := publishedAgg.Backfill(ctx, publishedcontent.BackfillOptions{
			Client:   publishedcontent.NewBackfillClient(cfg.BotHomepageV2Backfill.MANAPIBaseURL, http.DefaultClient),
			Paths:    publishedcontent.DefaultBackfillPaths(),
			Since:    time.Now().Add(-cfg.BotHomepageV2Backfill.Lookback),
			PageSize: cfg.BotHomepageV2Backfill.PageSize,
		})
		if err != nil {
			log.Printf("WARNING: bot homepage v2 backfill failed: %v", err)
		}
	}()
}
```

If Task 4 chose different exact config or helper names, adapt this snippet to those exact names without changing the behavior.

- [ ] **Step 4: Add API docs**

Add a concise docs page for the v2 endpoint with:

```text
GET /api/bot-homepage/globalmetaid/:globalMetaId?version=v2
GET /api/bot-homepage/globalmetaid/:globalMetaId?schemaVersion=botHomepage.v2
```

Document that default remains v1, `chainName=""` aggregates all chains, each section returns five items, mempool data is included, and payload appears under `data.payload` when non-binary.

- [ ] **Step 5: Run router tests**

```bash
CGO_ENABLED=0 go test ./internal/api -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit and post buzz**

```bash
git status --short
git add internal/api/router.go internal/api/router_test.go cmd/metaso-p2p/main.go docs/specs
git commit -m "feat: wire bot homepage v2 endpoint"
```

Then use `metabot-post-buzz` to post a development journal entry describing route wiring, v1 compatibility, v2 acceptance tests, and docs.

## Task 8: Final Verification And Whole-Branch Review

**Files:**
- All files changed by Tasks 1-7

- [ ] **Step 1: Run full local verification**

```bash
git status --short
CGO_ENABLED=0 go test ./... -count=1
git diff --check
```

Expected:

```text
go test ./... PASS
git diff --check exits 0
```

- [ ] **Step 2: Smoke representative v2 build path in tests**

Run targeted package tests again for easier failure localization:

```bash
CGO_ENABLED=0 go test ./internal/aggregator/userinfo ./internal/aggregator/skillservice ./internal/aggregator/publishedcontent ./internal/aggregator/bothomepage ./internal/indexer ./internal/api -count=1
```

Expected: PASS.

- [ ] **Step 3: Dispatch final reviewer subagent**

Ask the reviewer to inspect:

- v1 default compatibility.
- v2 schema and section response shape.
- Persona/legacy bio handling.
- Published-content create/modify/revoke folding.
- Buzz sorting by source `createdAt`.
- `chainName=""` cross-chain aggregation.
- Mempool polling/routing and idempotency.
- Two-month MANAPI backfill behavior.
- Test coverage and any missing production wiring.

- [ ] **Step 4: Fix reviewer findings**

If the reviewer finds issues, make narrow fixes, rerun the relevant tests, and commit with one of:

```bash
git commit -m "fix: address bot homepage v2 review findings"
git commit -m "docs: clarify bot homepage v2 behavior"
```

Use `metabot-post-buzz` after each fix commit.

- [ ] **Step 5: Final development journal**

If no code/doc changes were needed after Task 8 verification, post a `metabot-post-buzz` journal entry summarizing final verification without creating a new commit.

## Spec Coverage Checklist

- v2 endpoint selection: Task 6, Task 7.
- Default v1 compatibility: Task 6, Task 7, Task 8.
- Persona paths and legacy bio parsing: Task 1, Task 6.
- Sections for services, metaapps, skills, buzzes: Task 3, Task 5, Task 6.
- Five items plus `hasMore`: Task 3, Task 5, Task 6.
- Direct non-binary payload output: Task 3, Task 4, Task 6.
- MetaAPP/Bot Skill/Buzz modify/revoke folding: Task 3.
- Buzz modify display update without sort bump: Task 3.
- `chainName=""` cross-chain aggregation: Task 3, Task 5, Task 6.
- Mempool inclusion: Task 2, Task 3, Task 8.
- Recent two-month MANAPI backfill: Task 4.
- Optional section failures: Task 6.
- Proof summaries: Task 6.
- No new pagination endpoints: Task 6, Task 7.
