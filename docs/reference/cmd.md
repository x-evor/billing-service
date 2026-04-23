# cmd 包参考

本文档覆盖 `cmd/billing-service/main.go`。该目录只有一个进程入口函数，用于装配配置、数据库、服务层和 HTTP 层。

## 文件定位

- 路径：`cmd/billing-service/main.go`
- 对外职责：启动 `billing-service` 进程
- 依赖方向：`config` -> `exporter` -> `repository` -> `service` -> `httpapi`

## 函数

### `main`

- 签名：`func main()`
- 参数：无
- 返回：无
- 职责：
  - 读取运行配置
  - 打开 PostgreSQL 连接
  - 构造 `service.Service`
  - 启动后台采集循环
  - 启动 HTTP 服务
  - 监听退出信号并触发优雅关闭
- 调用位置：Go 进程入口，由运行时直接调用
- 主要副作用：
  - 读取环境变量
  - 建立数据库连接
  - 启动 goroutine
  - 监听网络地址
  - 向日志输出启动信息
- 错误/边界条件：
  - `config.Load()` 返回错误时直接 `log.Fatal`
  - `sql.Open` 返回错误时直接 `log.Fatal`
  - `ListenAndServe()` 返回非 `http.ErrServerClosed` 时直接 `log.Fatal`

### 启动流程拆解

| 步骤 | 代码调用 | 目的 |
| --- | --- | --- |
| 1 | `config.Load()` | 组装运行配置与默认值 |
| 2 | `sql.Open("pgx", cfg.DatabaseURL)` | 建立 PostgreSQL 驱动连接 |
| 3 | `signal.NotifyContext(...)` | 统一管理退出信号 |
| 4 | `service.New(...)` | 装配业务服务 |
| 5 | `svc.Start(ctx)` | 启动后台定时采集循环 |
| 6 | `httpapi.New(svc).Routes()` | 注册 HTTP 路由 |
| 7 | `server.ListenAndServe()` | 启动 HTTP 服务器 |
| 8 | `server.Shutdown(...)` | 在信号到达后优雅退出 |

### 依赖装配结果

`main` 中构造出的核心依赖如下：

- 配置对象：`config.Config`
- exporter 客户端：`*exporter.Client`
- 持久化实现：`*repository.Postgres`
- 服务层：`*service.Service`
- HTTP handler：`*httpapi.Handler`

这意味着 `main` 自身不承载业务逻辑，只负责装配与生命周期管理。
