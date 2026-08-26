# rigging-readiness-desk

`rigging-readiness-desk` 是面向舞台机械技术员、演出技术负责人和独立安全复核员的演出前吊挂安全放行工作台。它把设备基线、吊杆与吊点载荷建模、确定性安全计算、现场检查、整改复验、独立批准、场景清单冻结和启用凭据签发收束为一条可追溯流程。

Go 服务直接交付响应式 HTML、CSS 与 JavaScript 页面，并提供同源 JSON HTTP API。首页包含可按状态、场地、经办人、演出时间和标题组合筛选的待办队列，可直接续办临近演出。业务数据保存在本地 SQLite：启动时执行带 `schemaVersion` 的前向迁移，写命令按作业串行化，以 `expectedVersion` 检测并发冲突，并用 `Idempotency-Key` 保证重试不会产生重复业务写入。

## 构建

项目要求 Go 1.22 或更高版本。

```text
go build ./...
```

## 运行

默认只监听高位回环地址 `127.0.0.1:19091`，SQLite 文件为当前目录下的 `riggingdesk.db`：

```text
go run ./cmd/riggingdesk
```

可以显式指定回环地址和数据文件：

```text
go run ./cmd/riggingdesk -addr=127.0.0.1:19191 -data=var/riggingdesk.db
```

也可以设置 `PORT` 为纯端口号，服务将监听 `127.0.0.1:<PORT>`。显式 `-addr` 会覆盖该默认值。服务拒绝非回环地址，正常模式收到 `SIGINT` 或 `SIGTERM` 后会优雅关闭。

浏览器打开服务根路径 `/` 即可完成从建档到凭据校验的全部流程。所有重量使用克、位置使用毫米，页面会以千克和百分比展示计算结果。

## 测试

```text
go test ./...
```

## 真实 HTTP 自检

自检模式使用内存 SQLite，启动真实 TCP 监听，并由 HTTP 客户端完成建档、故意生成超载阻断项、整改修订、重算、五类现场检查、独立批准、冻结、签发与摘要校验；完成后自动关闭：

```text
go run ./cmd/riggingdesk -mode=selfcheck -addr=127.0.0.1:19091 -timeout=20s
```

## API 约定

API 前缀为 `/api/v1`。写请求使用 `application/json`，正文上限为 1 MiB，拒绝未知字段；`expectedVersion` 可由 JSON 字段或 `If-Match` 提供，幂等键由 `Idempotency-Key` 提供。错误响应包含稳定的 `code`、中文 `message` 和可选 `field`。`GET /healthz` 用于存活检查。

作业列表 `GET /api/v1/rigging-sessions` 支持 `status`、`venue`、`operatorId`、`performanceFrom`、`performanceTo`、`keyword`、`dueBefore`、`pendingOnly`、`offset` 和 `limit`。批量构件使用 `/loads/batch`；整改载荷方案通过 `/remediation/load-plan/preview` 预演后再调用 `/remediation/load-plan/apply`。检查完成且阻断项全部关闭后，从 `/review-confirmation` 取得当前版本的复核确认标识，再提交批准；退回决定通过原 `/review` 端点携带 `category` 与 `affectedLineIds`。
