# 晶圆制程批次与配方版本隔离服务

单进程 Go HTTP JSON 服务，使用 `database/sql` + 嵌入式 SQLite（`modernc.org/sqlite`，纯 Go、无 cgo）持久化到 `DB_PATH` 指定的可复用数据库文件，管理晶圆批次、工艺路线版本、设备资质、配方快照、制程运行、量测读数、暂扣处置与完整谱系。

## 功能总览

- 主数据：产品族、站点、工艺路线与修订、配方与版本（不可变快照）、设备与腔体、资质窗口、量测计划；
- 业务链：建档 → 草稿校验与启用 → 批次登记 → 子批拆分 → 首次进站冻结 → 进站排队 → 设备/配方资格校核 → 开工 → 完工 → 量测采集 → 自动判定 → 异常暂扣 → 人工复判 → 返工换版 / 恢复原路线 / 报废 → 下一站放行 → 批次关闭；
- 不变量：冻结快照隔离、站点顺序、晶圆唯一运行、抽样覆盖、暂扣阻断、历史不可改写；
- 可靠性：业务幂等键、乐观锁、真实事务与回滚审计、结构化审计事件；
- 后台作业：资质到期扫描、超时运行检查、暂扣升级、失败作业重试，支持重启恢复；
- 分析查询：过期资质未复判运行、在制批次（冻结路线 + 最近暂扣原因）稳定分页、超时站点队列、重复返工聚合、谱系审计。

## 文档

- [docs/01-领域说明.md](docs/01-领域说明.md)
- [docs/02-状态转换表.md](docs/02-状态转换表.md)
- [docs/03-数据模型.md](docs/03-数据模型.md)
- [docs/04-接口契约.md](docs/04-接口契约.md)

## 构建与运行

要求：Go toolchain go1.26.5（go.mod 语言版本 go 1.21，`GOTOOLCHAIN=local`）。

```bash
# 构建
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go build -o bin/server ./cmd/server

# 测试（真实临时 SQLite 文件）
GOTOOLCHAIN=local go test ./...

# 运行
PORT=8080 DB_PATH=/var/lib/wafer/wafer.db ./bin/server
```

环境变量：

| 变量 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- |
| PORT | 否 | 8080 | HTTP 端口 |
| DB_PATH | 是 | — | SQLite 数据库文件路径（禁止 `:memory:`） |
| SHUTDOWN_TIMEOUT | 否 | 10s | 优雅关闭超时 |
| JOB_INTERVAL | 否 | 5s | 后台作业调度间隔 |
| RUN_TIMEOUT | 否 | 30m | 运行超时阈值 |
| HOLD_ESCALATE_AFTER | 否 | 1h | 暂扣升级阈值 |
| JOB_MAX_ATTEMPTS | 否 | 3 | 作业最大重试次数 |

## Docker

架构无关构建（基础镜像锁定 `golang@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd`）：

```bash
./build_docker.sh wafer-server:amd64 linux/amd64
./build_docker.sh wafer-server:arm64 linux/arm64
```

容器内验证（无网络）：

```bash
docker run --rm --network none wafer-server:amd64 bash -c "go test ./... && go vet ./... && go build ./..."
```

## 示例

```bash
# 健康检查
curl -s localhost:8080/healthz

# 建档产品族
curl -s -X POST localhost:8080/api/v1/product-families \
  -H 'Content-Type: application/json' \
  -d '{"code":"PF-LOGIC","name":"逻辑芯片"}'

# 幂等登记批次（重放返回同一结果，响应头 Idempotent-Replay: true）
curl -s -X POST localhost:8080/api/v1/lots \
  -H 'Idempotency-Key: lot-reg-001' \
  -d '{"code":"LOT-001","product_family_id":"<pf_id>","route_id":"<route_id>","wafers":[{"code":"W1","slot":1},{"code":"W2","slot":2}]}'
```

完整链路示例见 [docs/04-接口契约.md](docs/04-接口契约.md) 末尾。
