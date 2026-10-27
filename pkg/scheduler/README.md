# scheduler

可复用的定时任务调度包：封装 [robfig/cron/v3](https://github.com/robfig/cron)，统一管理 Job 的 `Start` / `Stop`，并可选通过 Redis 选主，保证多实例下只有一个进程跑 cron。

本包不依赖应用的 `internal` 包。业务 Job 放在应用侧（如 `internal/scheduler`），在组装层（如 `internal/app`）构造并注册。

## 安装

模块路径与主工程一致时直接引用：

```go
import "github.com/ilaziness/app-tpl/pkg/scheduler"
```

## 单机使用

默认使用 `NoopLockProvider`（不做互斥）：

```go
sched := scheduler.New(scheduler.WithLogger(log))
sched.Register(myJob)

if err := sched.Start(ctx); err != nil {
    return err
}
defer sched.Stop(ctx)
```

## 多机选主（Redis）

由应用指定锁 key；续期间隔应小于锁 TTL：

```go
sched := scheduler.New(
    scheduler.WithLogger(log),
    scheduler.WithLock(scheduler.NewRedisLock(rdb, "myapp:scheduler:lock")),
    // 可选：scheduler.WithLockRenewInterval(10*time.Second),
)
```

`NewRedisLock` 可选 `scheduler.WithTTL(30*time.Second)` 设置锁 TTL（默认 30s）。

抢锁失败时 `Start` 返回 `nil` 且不启动 cron，其它服务（如 HTTP）可继续运行。

## 实现 Job

Job 只依赖 `Cron`（`AddFunc` / `Remove`），不要依赖完整 `Scheduler`：

```go
type MyJob struct{}

func (j *MyJob) Init(ctx context.Context, c scheduler.Cron) error {
    _, err := c.AddFunc("@every 1m", func() {
        // 定时逻辑
    })
    return err
}
```

在 `Start` 之前 `Register`。某个 Job 的 `Init` 失败时：停止 cron、释放锁并返回错误。

## Cron 表达式

`DefaultParser` 使用 **5 字段**（分 时 日 月 周）并支持 descriptor，例如：

- `0 * * * *` — 每小时
- `@every 1m` / `@hourly`

业务侧校验表达式时请使用同一 parser：`scheduler.DefaultParser()`。

## 与 pkg/lock 的区别

| | `pkg/lock` | `pkg/scheduler.LockProvider` |
|--|------------|------------------------------|
| 场景 | 短临界区互斥 | 调度选主 |
| Renew | 无 | 有（心跳续期） |
| Acquire | 失败返回 error | 失败返回 `(false, nil)` |

## 在本模板中的接线

1. 业务 Job 写在 `internal/scheduler`
2. 在 `internal/app` 中按配置构造：

```go
opts := []pkgscheduler.Option{pkgscheduler.WithLogger(a.log)}
if a.redisClient != nil {
    opts = append(opts, pkgscheduler.WithLock(
        pkgscheduler.NewRedisLock(a.redisClient, "app-tpl:scheduler:lock"),
    ))
}
sched := pkgscheduler.New(opts...)
sched.Register(scheduler.NewDemoJob(a.log)) // 业务 Job 同理 Register

a.addHook(Hook{
    Name:    "scheduler",
    OnStart: sched.Start,
    OnStop:  sched.Stop,
})
```

配置项见 `configs/config.yaml` 的 `scheduler.enabled`。

## 主要 API

| API | 说明 |
|-----|------|
| `New(opts...)` | 创建调度器 |
| `WithLogger` / `WithLock` / `WithLockRenewInterval` | 构造选项 |
| `NewRedisLock(client, key, opts...)` | Redis 选主锁 |
| `Register` / `Start` / `Stop` | 注册与生命周期 |
| `Cron` / `Job` | 任务接口 |
| `DefaultParser` | 标准 cron 解析器 |
