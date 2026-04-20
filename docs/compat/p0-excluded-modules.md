# P0 Excluded Modules (Task 6)

本文档定义 `meta-socket` 在 P0 版本中**明确不包含**的模块，以及后续补齐策略。

## P0 排除项

- `luckybag` 相关处理链路（发包、抢包、结算、过期清理）
- `grpc` 服务能力
- 重型 DB API（运维类/全量查询类接口）

这些模块不会进入 `meta-socket` 的默认初始化链路，P0 只保留 socket 推送核心闭包（连接管理、group/private/role 推送）。

## 初始化收口策略

`groupchat` 初始化通过以下两个收口点执行：

- [internal/groupchat/db/init_db.go](/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/db/init_db.go)
- [internal/groupchat/service/init.go](/Users/tusm/Documents/MetaID_Projects/meta-socket/internal/groupchat/service/init.go)

默认行为：

- migration：开启
- backup：关闭
- luckybag/grpc/heavy-db-api：关闭

## 配置开关

- `META_SOCKET_GROUPCHAT_MIGRATION_ENABLED`（默认 `true`）
- `META_SOCKET_GROUPCHAT_BACKUP_ENABLED`（默认 `false`）
- `META_SOCKET_GROUPCHAT_LUCKYBAG_ENABLED`（默认 `false`）
- `META_SOCKET_GROUPCHAT_GRPC_ENABLED`（默认 `false`）
- `META_SOCKET_GROUPCHAT_HEAVY_API_ENABLED`（默认 `false`）

## 后续补齐策略

- P1：按“能力开关 + 契约测试”方式增量恢复非核心能力，不直接回滚为旧项目全量初始化。
- P1：恢复任一排除项时，必须先补对应契约测试并完成 `Task 7` 双跑对比。

