# httpapi 包参考

本文档覆盖 `internal/httpapi/handler.go`，说明 HTTP 路由注册和各处理函数与服务层的映射关系。

## 包职责

`internal/httpapi` 负责把 `service.Service` 暴露成 HTTP 接口。当前只承担：

- 健康检查
- 运行状态查询
- 立即触发任务
- 运行镜像身份查询

它不包含业务逻辑，所有计费行为都委托给 `service.Service`。

## 类型

### `Handler`

签名：`type Handler struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `service` | `*service.Service` | 业务服务实例 |

## 函数与方法

### `New`

- 签名：`func New(svc *service.Service) *Handler`
- 参数：
  - `svc`：业务服务实例
- 返回：
  - `*Handler`
- 职责：
  - 构造 HTTP handler
- 调用位置：
  - `cmd/billing-service/main.go`
- 主要副作用：无
- 错误/边界条件：
  - 不校验 `svc` 是否为 `nil`

### `Routes`

- 签名：`func (h *Handler) Routes() http.Handler`
- 参数：无
- 返回：
  - `http.Handler`：注册好全部路由的 `*http.ServeMux`
- 职责：
  - 注册当前服务支持的全部 HTTP 路径
- 调用位置：
  - `cmd/billing-service/main.go`
- 主要副作用：
  - 构造新的 `ServeMux`
- 错误/边界条件：
  - 当前路由集合固定，不包含版本协商或中间件

当前注册的路由：

| 路径 | 处理函数 | 说明 |
| --- | --- | --- |
| `/api/ping` | `ping` | 返回运行镜像身份 |
| `/healthz` | `healthz` | 返回最近一次任务健康状态 |
| `/v1/status` | `status` | 返回最近一次任务结果快照 |
| `/v1/jobs/collect-and-rate` | `collectAndRate` | 触发立即采集与计费 |
| `/v1/jobs/reconcile` | `reconcile` | 触发同一执行路径，但 job 名不同 |

### `ping`

- 签名：`func (h *Handler) ping(w http.ResponseWriter, r *http.Request)`
- 参数：`w`、`r`
- 返回：无
- 职责：
  - 返回 `service.Ping()` 的结果
- 调用位置：
  - `Routes`
- 主要副作用：
  - 写 HTTP 响应
- 错误/边界条件：
  - 当前不限制 HTTP 方法，任何方法都会返回 `200`

### `healthz`

- 签名：`func (h *Handler) healthz(w http.ResponseWriter, r *http.Request)`
- 参数：`w`、`r`
- 返回：无
- 职责：
  - 调用 `service.Health()`
  - 将布尔健康值映射为 HTTP 状态码与 `status` 文本
- 调用位置：
  - `Routes`
- 主要副作用：
  - 写 HTTP 响应
- 错误/边界条件：
  - `ok == false` 时返回 `503`
  - 当前不限制 HTTP 方法

### `status`

- 签名：`func (h *Handler) status(w http.ResponseWriter, r *http.Request)`
- 参数：`w`、`r`
- 返回：无
- 职责：
  - 返回最近一次 `service.Status()`
- 调用位置：
  - `Routes`
- 主要副作用：
  - 写 HTTP 响应
- 错误/边界条件：
  - 当前不限制 HTTP 方法

### `collectAndRate`

- 签名：`func (h *Handler) collectAndRate(w http.ResponseWriter, r *http.Request)`
- 参数：`w`、`r`
- 返回：无
- 职责：
  - 只接受 `POST`
  - 调用 `service.RunCollectAndRate(r.Context(), "collect-and-rate")`
  - 根据是否返回错误决定 `200` 或 `503`
- 调用位置：
  - `Routes`
- 主要副作用：
  - 触发一次完整计费执行
  - 写 HTTP 响应
- 错误/边界条件：
  - 非 `POST` 返回 `405 method not allowed`
  - 服务层返回错误时仍返回 `JobResult`，但状态码为 `503`

### `reconcile`

- 签名：`func (h *Handler) reconcile(w http.ResponseWriter, r *http.Request)`
- 参数：`w`、`r`
- 返回：无
- 职责：
  - 只接受 `POST`
  - 调用 `service.RunCollectAndRate(r.Context(), "reconcile")`
  - 与 `collectAndRate` 共用同一业务路径，仅 job 名不同
- 调用位置：
  - `Routes`
- 主要副作用：
  - 触发一次完整计费执行
  - 写 HTTP 响应
- 错误/边界条件：
  - 非 `POST` 返回 `405`
  - 服务层返回错误时状态码为 `503`

### `writeJSON`

- 签名：`func writeJSON(w http.ResponseWriter, status int, payload any)`
- 参数：
  - `w`：响应写入器
  - `status`：HTTP 状态码
  - `payload`：任意可 JSON 编码的对象
- 返回：无
- 职责：
  - 统一设置 `Content-Type: application/json`
  - 写入状态码和 JSON 响应体
- 调用位置：
  - `ping`
  - `healthz`
  - `status`
  - `collectAndRate`
  - `reconcile`
- 主要副作用：
  - 写 HTTP Header 和 Body
- 错误/边界条件：
  - JSON 编码错误被忽略，调用方不会收到显式错误

## 接口设计说明

- `collect-and-rate` 和 `reconcile` 在当前实现中只有作业名不同，没有独立 reconciliation 流程
- `/healthz` 反映的是最近一次作业的结果，不是即时数据库探活
- `/api/ping` 用于发布追踪，不依赖数据库或 exporter
