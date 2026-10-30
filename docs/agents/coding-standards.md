# 编码规范（详情）

> 通用核心规则见 [AGENTS.md](../../AGENTS.md#核心规则始终遵守)。本文档补充数据库、配置、运维端点与基础设施约定。

## 数据库规范

必须使用 Bun 官方组件：

- `github.com/uptrace/bun` - 核心 ORM
- `github.com/uptrace/bun/dialect/mysqldialect` - MySQL 方言
- `github.com/uptrace/bun/dialect/pgdialect` - PostgreSQL 方言
- `github.com/uptrace/bun/dialect/sqlitedialect` - SQLite 方言
- `github.com/uptrace/bun/driver/sqliteshim` - SQLite 驱动
- `github.com/uptrace/bun/driver/pgdriver` - PostgreSQL 驱动
- `github.com/go-sql-driver/mysql` - MySQL 驱动
- `github.com/uptrace/bun/extra/bundebug` - 调试钩子

## 配置管理

加载逻辑见 `internal/config/config.go` 的 `LoadWithMeta` / `LoadWithEnv`。

**覆盖优先级（高覆盖低）**：

1. **环境选择符** `overlayEnv`：`-e/--env` > `APP_ENV` > 空（不 merge overlay）
2. **进程环境变量**：仅敏感项（见 `config.SensitiveBinds` / `configs/.env.example`）
3. **dotenv**（可选）：`.env.{env}` → `configs/.env.{env}` → `.env` → `configs/.env`（不覆盖已有进程环境；dotenv 之间先加载优先）
4. **`config.{env}.yaml` overlay**：仅当 `overlayEnv` 非空且主文件名为 `config.yaml` 时 merge
5. **主配置**：`-c/--config` 指定的文件，或默认 `configs/config.yaml`
6. **代码 `SetDefault`**

**`app.env` 最终值**：`overlayEnv` 非空时锁定为该值；否则保留 YAML；仍为空则 `dev`。环境 overlay 文件不要写 `app.env`。

**推荐用法**：

- 本地：`serve` / `serve -c configs/config.yaml` → 只读主配置（开箱 SQLite）
- 显式环境：`serve -e prod` 或 `APP_ENV=prod`
- 逃生舱：`-c path/to/complete.yaml`（非 `config.yaml`）只加载该文件

支持的环境名：`dev`、`prod`、`test`。CLI：`config validate`、`config show [--check]`。

## 健康检查端点

注册于 `internal/router/system.go`；路径常量与 `SystemPaths` 见 `internal/router/paths.go`：

| 端点 | 语义 | 检查依赖 |
|------|------|----------|
| `GET /health` | 轻量存活探测 | 否（委托 `/liveness`） |
| `GET /liveness` | 存活探测 | 否 |
| `GET /readiness` | 就绪探测 | 是（通过 `health.Checker`） |
| `GET /version` | 应用版本 | 否 |

响应使用统一 `{code, message, data}` envelope；K8s/Docker HTTP 探测仅依赖状态码（2xx/503）。

**扩展依赖检查**：在 `internal/app/http.go` 向 `HealthHandler` 注册 checker，实现 `internal/health.Checker`（`Check(ctx context.Context) error`）。示例：`health.NewDatabaseChecker(db)`。多个 checker 顺序执行并共享 `readinessCheckTimeout`（默认 5s）；依赖较多或较慢时可增大超时，或对单个 checker 使用独立 deadline，必要时用 `errgroup` 并行。

**`/health` 响应变更**：`data.status` 为 `"alive"`（与 `/liveness` 一致），不再返回 `"ok"`。

**运行时指标**：启用 `metrics.enabled` 后使用 `GET /metrics`（Prometheus），不再提供 `/stats`。

**CLI**：`app-tpl health` 默认探测 `/readiness`，供 Docker `HEALTHCHECK` 使用；`--probe` 可选 `health`/`liveness`/`readiness`。

## 缓存和事件系统

- **缓存原语**（`pkg/cache`）：通用 Cache 接口、Memory/Redis/Multi/Nop 实现、Factory、errors、KeyGenerator — 自包含，不依赖 internal/config — 示例见 `pkg/cache/example_test.go`
- **业务缓存管理**（`internal/cache`）：KeySpec（Prefix + TTL）定义业务键，Manager 提供按实体的 typed Get/Set/Delete 方法（如 `GetUserByID/SetUserByID/DelUserByID`），内部用泛型 helper 统一处理 Memory 直存与 Redis JSON 反序列化 — 按业务实体拆文件（`user.go` 等），KeySpec 与 typed 方法同文件存放 — 示例见 `internal/cache/user_test.go`
- **事件机制**（`pkg/event`）：通用事件总线 — 发布/订阅、同步/异步、优先级、通配符、取消订阅函数、哨兵错误 — 自包含，不依赖 internal — 示例见 `pkg/event/example_test.go`
- **业务事件**（`internal/event`）：只放业务相关事件定义（事件名常量、payload、发布函数），机制在 `pkg/event`，按业务领域拆文件（如 `lifecycle.go`）

## 锁 / 调度 / 验证码 / 存储 / 支付（`pkg` 预留能力）

`pkg/*` 是可复用公共能力库，**样例业务不强制接线**。启用时：读 `configs/config.yaml` 对应段，在 `internal/app` 构造并注入到需要的 service/handler。

- **分布式锁**（`pkg/lock`）：Redis 或内存实现；业务需要时用 `pkglock.NewLockerFactory` 创建并注入 service；业务 key 用 `internal/lock` 辅助函数生成
- **调度器**（`pkg/scheduler`）：可复用 cron + 可选 Redis 选主；业务 Job 放 `internal/scheduler`；配置 `scheduler.enabled`，在 `wireScheduler` 中 `New` / `Register`（用法见包内 `README.md`）
- **验证码**（`pkg/captcha`）：接口 + PNG 实现；可注入 cache store；配置 `captcha.*`
- **对象存储**（`pkg/objectstorage`）：Aliyun/Tencent/Qiniu；配置 `object_storage.*`，默认关闭
- **支付**（`pkg/payment`）：厂商 registry + Alipay/WeChat；配置 `payment.enabled`，默认关闭，不强制初始化商户客户端
- **事件**（`pkg/event`）：通用总线；样例仅发布生命周期事件，业务订阅按需扩展

## 模型生成

- 优先 `app-tpl gen model` 生成 `*.gen.go`（见 [commands.md](commands.md)）
- 逻辑关联：`configs/model-relations.yaml`
- 迁移多语句用 `--bun:split` 分隔
