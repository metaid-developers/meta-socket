# Meta Socket Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 抽离 `show-now-tmp` 的 Socket 实时推送能力到独立开源项目 `meta-socket`，第一阶段保证 `IDBots` 仅修改 `base_url` 即可保持原行为不变。

**Architecture:** 采用“先兼容、后裁剪”策略。先在 `meta-socket` 落地与 `wss://api.idchat.io/socket/socket.io` 对齐的协议与推送链路（Socket.IO 接入 + ZMQ 上游 + group/private/role 分发），再分阶段拆除与 socket 核心无关的历史模块（luckybag、重型 DB API、swagger 等）。通过契约测试与双跑对比保证 I/O 向前兼容。

**Tech Stack:** Go, Gin, Socket.IO (`github.com/zishang520/socket.io/v2/socket`), PebbleDB, ZMQ, multi-chain adapter (BTC/MVC/DOGE), contract test (Go + Node socket.io-client)

---

## Compatibility Baseline (P0 Must Keep)

- Socket 路径兼容：必须支持 `/socket/socket.io`；建议同时支持 `/socket.io`（避免反代差异导致接入失败）。
- 握手参数兼容：`query.metaid`, `query.type`（`app|pc`）语义保持一致。
- 事件通道兼容：客户端统一监听 `message` 事件。
- 消息封装兼容：`{"M":...,"C":...,"D":...}`（当前服务端是字符串 JSON）。
- 关键推送事件兼容：
  - `WS_SERVER_NOTIFY_GROUP_CHAT`
  - `WS_SERVER_NOTIFY_PRIVATE_CHAT`
  - `WS_SERVER_NOTIFY_GROUP_ROLE`
  - `WS_RESPONSE_SUCCESS`
- 心跳兼容：
  - 客户端 `socket.emit('ping')` 后，服务端通过 `message` 回 `{"M":"pong","C":200}`。
  - 兼容旧的 `HEART_BEAT` 包（`M=HEART_BEAT`）。
- 房间广播兼容：`roomBroadcastEnabled=true` 时支持 group room 推送与连接后补 join。
- 连接策略兼容：同用户同端类型连接数上限（`pc/app` 分别 3）与淘汰旧连接逻辑保持一致。

## Non-Goals (P0 明确不做)

- 不提供现有 `group-chat` 全量 REST API（尤其是 DB 运维类接口、swagger 大体量文档接口）。
- 不在首发实现 luckybag/grpc 相关能力（仅在后续有明确兼容需求时再补）。
- 不在 P0 重构协议模型定义，不改客户端解析逻辑。

## Current Code Facts (用于边界确认)

- Socket 核心：`common/socket_util/socket_manager.go`, `common/socket_util/socket_data.go`
- 业务推送组装：`basicprotocols/group_chat/service/chat_ws.go`, `.../socket_service/socket_service.go`, `.../socket_room.go`
- 上游入口：`man/man.go` 的 `doZmqRun -> group_chat.ProcessGroupChatPin(...)`
- pin 分发：`basicprotocols/group_chat/indexer/indexer.go`
- 持久化与回调：`basicprotocols/group_chat/db/*`
- 客户端真实依赖（IDBots）：
  - `path: '/socket/socket.io'`
  - `query: { metaid, type }`
  - 监听 `message`，解析 `M/C/D`
  - 处理 `WS_SERVER_NOTIFY_*` + `WS_RESPONSE_SUCCESS` 分支

## Scope Sizing (粗略量化)

- 全仓 Go 代码约 `93,256` 行。
- `group_chat` 约 `56,332` 行；其中直接 socket 文件仅约 `1,769` 行。
- 结合上游闭包后，P0 最小可运行子集预计约 `20,113` 行（不含 adapter/common 进一步裁剪）。
- 结论：明显冗余较多，按代码量估算“非 socket 核心”占比超过 60%。

## File Structure (meta-socket 目标结构)

- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/cmd/meta-socket/main.go`
  - 进程入口，加载配置，启动 socket 与上游 indexer
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/config/config.go`
  - 统一配置（socket/chain/zmq/pebble/profile）
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/socket/server.go`
  - SocketManager（从旧项目迁移并去历史耦合）
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/socket/contract.go`
  - `SocketData`、事件常量、兼容协议定义
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/socket/routes.go`
  - 同时挂载 `/socket/socket.io/*f` 与 `/socket.io/*f`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/pipeline/zmq_runner.go`
  - ZMQ 消费与 pin 入口
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/pipeline/pin_router.go`
  - 协议分发（group/private/role）
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/db/*.go`
  - chat/private/group/community/global-block/socket-info 的最小 DB 子集
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/push/*.go`
  - `chat_ws` + `socket_service` + room join
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/profile/user_info_service.go`
  - 本地优先 + 远端可选回源（可配置）
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/tests/contract/socket_compat_test.go`
  - 协议契约测试
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/tests/fixtures/*.json`
  - group/private/role 事件夹具
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/docs/compat/compat-matrix.md`
  - 客户端兼容矩阵

### Task 1: 建立 P0 兼容契约与回归基线

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/docs/compat/compat-matrix.md`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/tests/fixtures/README.md`

- [ ] **Step 1: 固化现网契约**
  - 写清路径、握手参数、消息 envelope、事件名、心跳行为、错误码。
- [ ] **Step 2: 生成最小夹具**
  - 准备 group/private/role 三类标准 payload + `WS_RESPONSE_SUCCESS` 包裹场景。
- [ ] **Step 3: 明确验收门槛**
  - 定义“IDBots 仅改 base_url 后通过”的验收标准。

### Task 2: 搭建 meta-socket 最小运行骨架

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/go.mod`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/cmd/meta-socket/main.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/config/config.go`

- [x] **Step 1: 初始化工程与配置模型**
- [x] **Step 2: 预留 socket/zmq/pebble/profile 配置节**
- [x] **Step 3: 启动空服务并通过健康检查**

### Task 3: 迁移 SocketManager 并做路径兼容

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/socket/server.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/socket/contract.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/socket/routes.go`

- [x] **Step 1: 迁移连接管理、设备限流、room、extra push 能力**
- [x] **Step 2: 同时挂载 `/socket/socket.io` 与 `/socket.io`**
- [x] **Step 3: 保持 `message` 事件与 `M/C/D` 字符串化输出兼容**
- [x] **Step 4: 保持 `ping -> pong(message)` 行为兼容**

### Task 4: 迁移上游 ZMQ + pin 分发闭包

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/pipeline/zmq_runner.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/pipeline/pin_router.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/adapter/*`

- [ ] **Step 1: 接入 BTC/MVC/DOGE 的 `ZmqRun` 能力**
- [ ] **Step 2: 打通 `pin -> group/private/role` 最小分发链路**
- [ ] **Step 3: 保留 `globalMetaId` 计算与映射兼容逻辑**

### Task 5: 迁移 group/private/role 推送链路

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/push/chat_ws.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/push/socket_service.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/push/socket_room.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/db/*.go`

- [ ] **Step 1: 迁移 db 回调钩子（`dealGroupChatItem/dealPrivateChatItem/dealGroupRoleInfoChangeList`）**
- [ ] **Step 2: 迁移 payload 组装，保证字段与旧服务一致**
- [ ] **Step 3: 完成 room 广播与 fallback 单播逻辑**

### Task 6: 裁剪非核心模块（首轮收口）

**Files:**
- Modify: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/db/init_db.go`
- Modify: `/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/service/init.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/docs/compat/p0-excluded-modules.md`

- [ ] **Step 1: 从初始化链路移除 luckybag/grpc/重型 DB API 依赖**
- [ ] **Step 2: 将迁移/备份做成可开关（默认开启迁移，备份可选）**
- [ ] **Step 3: 明确记录 P0 排除项与后续补齐策略**

### Task 7: 完成契约测试与双跑对比

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/tests/contract/socket_compat_test.go`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/tests/contract/replay_compare_test.go`

- [ ] **Step 1: 契约测试（连接/握手/心跳/事件/字段）**
- [ ] **Step 2: 双跑对比（旧服务 vs meta-socket）同输入比对输出**
- [ ] **Step 3: 针对 `WS_RESPONSE_SUCCESS` 包裹场景单测**

### Task 8: IDBots 兼容验收（只改 base_url）

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/docs/compat/idbots-smoke-checklist.md`

- [ ] **Step 1: 在 IDBots / IDBots-indev / IDBots_cursor 分别进行 smoke test**
- [ ] **Step 2: 验证 group/private/role 三类消息入库与解密流程不回退**
- [ ] **Step 3: 输出兼容报告与遗留差异清单**

### Task 9: 开源首发准备

**Files:**
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/README.md`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/config.example.toml`
- Create: `/Users/tusm/Documents/MetaID_Projects/meta-socket/LICENSE`

- [ ] **Step 1: 编写“只改 base_url 即可接入”的文档与示例**
- [ ] **Step 2: 明确自托管部署说明（chain/zmq/pebble 依赖）**
- [ ] **Step 3: 发布 v0.x（P0 兼容版）并冻结契约**

## P0 验收矩阵（必须全部通过）

- 连接验收：
  - `/socket/socket.io` 可连
  - `/socket.io` 可连
  - `metaid/type` 缺失时行为与旧服务一致
- 协议验收：
  - `message` 事件中 `M/C/D` 结构一致
  - `WS_SERVER_NOTIFY_GROUP_CHAT` 字段一致
  - `WS_SERVER_NOTIFY_PRIVATE_CHAT` 字段一致
  - `WS_SERVER_NOTIFY_GROUP_ROLE` 字段一致
  - `WS_RESPONSE_SUCCESS` 可被 IDBots 旧逻辑正确消费
- 心跳验收：
  - `emit('ping')` 后收到 `M:'pong'`
  - `HEART_BEAT` 请求收到兼容回包
- 行为验收：
  - 同用户多端连接限制与旧服务一致
  - room 广播与单播 fallback 行为一致

## Risk & Decision Gates

- 风险1：旧服务实际通过网关把 `/socket/socket.io` 转到 `/socket.io`。  
  Gate：meta-socket 直接双路径支持，避免部署方必须配反代。

- 风险2：用户资料回源默认会落到 `file.metaid.io`，与去中心化目标冲突。  
  Gate：P0 保兼容可回源，P1 增加“本地优先 + 可禁远程回源”强开关。

- 风险3：`group_chat` 初始化链路包含大量 luckybag/备份/迁移副作用。  
  Gate：P0 先保迁移，禁用非必要处理器；P1 再做彻底解耦。

- 风险4：缺少现成 socket 自动化测试。  
  Gate：先建契约测试，再执行迁移代码，任何行为差异先在测试中固化。

## Recommended Execution Order

1. Task 1 → Task 2 → Task 3（先把“可连接且协议兼容”跑通）
2. Task 4 → Task 5（打通上游真实数据）
3. Task 7 → Task 8（双跑对比 + IDBots 验收）
4. Task 6 → Task 9（收口与开源发布）
