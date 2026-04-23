# exporter 包参考

本文档覆盖 `internal/exporter/client.go`，说明与 `xray-exporter` 的 HTTP 调用约定。

## 包职责

`internal/exporter` 目前只提供一个客户端类型 `Client`。它不负责业务校验，也不负责窗口推进，只负责：

- 拼接窗口拉取 URL
- 注入查询参数和认证头
- 发送 HTTP 请求
- 反序列化为 `model.SnapshotWindowPage`

## 类型

### `Client`

签名：`type Client struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `serviceToken` | `string` | 写入 `Authorization: Bearer ...` 的内部服务令牌 |

## 函数

### `NewClient`

- 签名：`func NewClient(serviceToken string) *Client`
- 参数：
  - `serviceToken`：调用 exporter 时使用的内部服务令牌
- 返回：
  - `*Client`：已去掉首尾空白的客户端实例
- 职责：
  - 构造 exporter 客户端
- 调用位置：
  - `cmd/billing-service/main.go`
- 主要副作用：无
- 错误/边界条件：
  - 不做空令牌校验；必填校验在 `config.Load()` 中完成

### `FetchWindow`

- 签名：`func (c *Client) FetchWindow(ctx context.Context, source config.ExporterSource, since, until time.Time, limit int, cursor *time.Time) (model.SnapshotWindowPage, error)`
- 参数：
  - `ctx`：请求上下文
  - `source`：来源配置，提供 `BaseURL`、`SourceID`、`TimeoutSeconds`
  - `since`：窗口起始时间
  - `until`：窗口结束时间
  - `limit`：分页大小
  - `cursor`：可选游标，非空时加入查询参数
- 返回：
  - `model.SnapshotWindowPage`：窗口分页结果
  - `error`：构造请求、发送请求、状态码异常、JSON 解码失败时返回
- 职责：
  - 将 `source.BaseURL` 与 `/v1/snapshots/window` 拼接成请求地址
  - 设置 `since`、`until`、`limit`、`cursor` 查询参数
  - 设置 `Accept: application/json` 和 `Authorization: Bearer <token>` 请求头
  - 基于来源超时时间构造 `http.Client`
- 调用位置：
  - `service.collectSource`
- 主要副作用：
  - 发起外部 HTTP 请求
- 错误/边界条件：
  - `source.TimeoutSeconds <= 0` 时使用 15 秒默认超时
  - 响应状态码不是 `200 OK` 时返回错误
  - 返回体不是合法 JSON 时返回错误

## 请求契约

### URL

- 路径：`/v1/snapshots/window`
- 拼接方式：`url.JoinPath(strings.TrimRight(source.BaseURL, "/"), "/v1/snapshots/window")`

### 查询参数

| 参数 | 含义 |
| --- | --- |
| `since` | RFC3339 格式的窗口起始时间 |
| `until` | RFC3339 格式的窗口结束时间 |
| `limit` | 本页最大快照数量 |
| `cursor` | 非空时追加，用于翻页 |

### 请求头

| Header | 值 |
| --- | --- |
| `Accept` | `application/json` |
| `Authorization` | `Bearer <serviceToken>` |

## 与 service 层的边界

- `Client` 不校验 `snapshot.node_id` / `snapshot.env`
- `Client` 不处理窗口重叠或完成位置推进
- `Client` 不判断 `NextCursor` 是否为空或是否合法

这些行为都在 `service.collectSource` 中完成。
