# repository 包参考

本文档覆盖 `internal/repository/repository.go` 与 `internal/repository/postgres.go`，说明仓储接口、PostgreSQL 实现及其与 SQL 表的映射关系。

## 包职责

`internal/repository` 是服务层和 PostgreSQL 之间的适配层。它负责：

- 读取 checkpoint、账户配额、计费配置、来源同步状态
- 对分钟桶、账本、checkpoint、配额状态、来源同步状态做幂等写入
- 屏蔽 SQL 细节，让服务层只依赖领域对象

## 表映射

| 领域对象 / 方法 | PostgreSQL 表 |
| --- | --- |
| `Checkpoint` | `traffic_stat_checkpoints` |
| `MinuteBucket` | `traffic_minute_buckets` |
| `LedgerEntry` | `billing_ledger` |
| `QuotaState` | `account_quota_states` |
| `BillingProfile` | `account_billing_profiles` |
| `SourceSyncState` | `billing_source_sync_state` |

参考 DDL：

- [../../sql/billing-service-schema.sql](../../sql/billing-service-schema.sql)

## 接口

### `Repository`

签名：`type Repository interface`

该接口定义了服务层需要的最小持久化能力。

| 方法 | 参数 | 返回 | 作用 |
| --- | --- | --- | --- |
| `GetCheckpoint` | `ctx, nodeID, accountUUID` | `(*model.Checkpoint, error)` | 读取差分基线 |
| `UpsertCheckpoint` | `ctx, checkpoint` | `error` | 更新差分基线 |
| `UpsertMinuteBucket` | `ctx, bucket` | `(bool, error)` | upsert 分钟桶，并返回是否已存在 |
| `UpsertLedger` | `ctx, entry` | `(bool, error)` | upsert 账本，并返回是否已存在 |
| `GetQuotaState` | `ctx, accountUUID` | `(*model.QuotaState, error)` | 读取账户配额状态 |
| `UpsertQuotaState` | `ctx, state` | `error` | 更新账户配额状态 |
| `GetBillingProfile` | `ctx, accountUUID` | `(*model.BillingProfile, error)` | 读取账户定价配置 |
| `GetSourceSyncState` | `ctx, sourceID` | `(*model.SourceSyncState, error)` | 读取来源同步进度 |
| `UpsertSourceSyncState` | `ctx, state` | `error` | 更新来源同步进度 |

## 类型

### `Postgres`

签名：`type Postgres struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `db` | `*sql.DB` | PostgreSQL 连接池 |

### `NewPostgres`

- 签名：`func NewPostgres(db *sql.DB) *Postgres`
- 参数：
  - `db`：PostgreSQL 连接池
- 返回：
  - `*Postgres`：仓储实现
- 职责：
  - 构造 `Repository` 的 PostgreSQL 适配器
- 调用位置：
  - `cmd/billing-service/main.go`
- 主要副作用：无
- 错误/边界条件：
  - 不校验 `db` 是否为 `nil`

## 读取方法

### `GetCheckpoint`

- 签名：`func (p *Postgres) GetCheckpoint(ctx context.Context, nodeID, accountUUID string) (*model.Checkpoint, error)`
- 参数：`ctx`、`nodeID`、`accountUUID`
- 返回：
  - 命中时返回 `*model.Checkpoint`
  - 未命中时返回 `nil, nil`
  - 查询失败时返回错误
- 职责：
  - 从 `traffic_stat_checkpoints` 读取指定节点和账户的累计值基线
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 读取数据库
- 错误/边界条件：
  - `sql.ErrNoRows` 被转换为 `nil, nil`

### `GetQuotaState`

- 签名：`func (p *Postgres) GetQuotaState(ctx context.Context, accountUUID string) (*model.QuotaState, error)`
- 参数：`ctx`、`accountUUID`
- 返回：
  - 命中时返回 `*model.QuotaState`
  - 未命中时返回 `nil, nil`
  - 查询失败时返回错误
- 职责：
  - 读取 `account_quota_states`
  - 把 `last_rated_bucket_at` 的 `sql.NullTime` 转成 `*time.Time`
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 读取数据库
- 错误/边界条件：
  - `sql.ErrNoRows` 被转换为 `nil, nil`

### `GetBillingProfile`

- 签名：`func (p *Postgres) GetBillingProfile(ctx context.Context, accountUUID string) (*model.BillingProfile, error)`
- 参数：`ctx`、`accountUUID`
- 返回：
  - 命中时返回 `*model.BillingProfile`
  - 未命中时返回 `nil, nil`
  - 查询失败时返回错误
- 职责：
  - 从 `account_billing_profiles` 读取账户级定价配置
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 读取数据库
- 错误/边界条件：
  - `sql.ErrNoRows` 被转换为 `nil, nil`

### `GetSourceSyncState`

- 签名：`func (p *Postgres) GetSourceSyncState(ctx context.Context, sourceID string) (*model.SourceSyncState, error)`
- 参数：`ctx`、`sourceID`
- 返回：
  - 命中时返回 `*model.SourceSyncState`
  - 未命中时返回 `nil, nil`
  - 查询失败时返回错误
- 职责：
  - 从 `billing_source_sync_state` 读取来源同步状态
  - 把 3 个 `sql.NullTime` 字段转换为可选时间指针
- 调用位置：
  - `service.collectSource`
- 主要副作用：
  - 读取数据库
- 错误/边界条件：
  - `sql.ErrNoRows` 被转换为 `nil, nil`

## 写入方法

### `UpsertCheckpoint`

- 签名：`func (p *Postgres) UpsertCheckpoint(ctx context.Context, checkpoint model.Checkpoint) error`
- 参数：`ctx`、`checkpoint`
- 返回：`error`
- 职责：
  - 把最新累计值基线写入 `traffic_stat_checkpoints`
  - 以 `(node_id, account_uuid)` 为冲突键更新旧记录
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 写数据库
- 错误/边界条件：
  - SQL 执行失败时直接返回错误

### `UpsertMinuteBucket`

- 签名：`func (p *Postgres) UpsertMinuteBucket(ctx context.Context, bucket model.MinuteBucket) (bool, error)`
- 参数：`ctx`、`bucket`
- 返回：
  - `bool`：写入前该分钟桶是否已经存在
  - `error`：查询或写入失败时返回
- 职责：
  - 先调用 `minuteBucketExists`
  - 再对 `traffic_minute_buckets` 执行 upsert
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 先读后写数据库
- 错误/边界条件：
  - 幂等性依赖主键 `(bucket_start, node_id, account_uuid, region, line_code)`

### `UpsertLedger`

- 签名：`func (p *Postgres) UpsertLedger(ctx context.Context, entry model.LedgerEntry) (bool, error)`
- 参数：`ctx`、`entry`
- 返回：
  - `bool`：写入前该账本是否已存在
  - `error`：查询或写入失败时返回
- 职责：
  - 先调用 `ledgerExists`
  - 再对 `billing_ledger` 执行 upsert
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 先读后写数据库
- 错误/边界条件：
  - 幂等性依赖 `entry.ID`

### `UpsertQuotaState`

- 签名：`func (p *Postgres) UpsertQuotaState(ctx context.Context, state model.QuotaState) error`
- 参数：`ctx`、`state`
- 返回：`error`
- 职责：
  - 将账户状态写入 `account_quota_states`
  - 根据 `account_uuid` 做 upsert
- 调用位置：
  - `service.processSample`
- 主要副作用：
  - 写数据库
- 错误/边界条件：
  - `LastRatedBucketAt == nil` 时按 SQL `NULL` 写入

### `UpsertSourceSyncState`

- 签名：`func (p *Postgres) UpsertSourceSyncState(ctx context.Context, state model.SourceSyncState) error`
- 参数：`ctx`、`state`
- 返回：`error`
- 职责：
  - 将来源同步状态写入 `billing_source_sync_state`
  - 根据 `source_id` 做 upsert
- 调用位置：
  - `service.collectSource`
  - `service.recordSourceFailure`
- 主要副作用：
  - 写数据库
- 错误/边界条件：
  - 各时间字段为 `nil` 时按 SQL `NULL` 写入

## 内部辅助函数

这些函数不属于 `Repository` 接口，但仍是当前生产代码的一部分。

| 函数 | 签名 | 参数 | 返回 | 职责 | 调用位置 | 副作用 / 边界条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `minuteBucketExists` | `func (p *Postgres) minuteBucketExists(ctx context.Context, bucket model.MinuteBucket) (bool, error)` | `ctx`、`bucket` | 是否存在、错误 | 检查分钟桶主键是否已存在 | `UpsertMinuteBucket` | `sql.ErrNoRows` 转为 `false, nil` |
| `ledgerExists` | `func (p *Postgres) ledgerExists(ctx context.Context, id string) (bool, error)` | `ctx`、`id` | 是否存在、错误 | 检查账本主键是否已存在 | `UpsertLedger` | `sql.ErrNoRows` 转为 `false, nil` |
| `ensureUTC` | `func ensureUTC(ts time.Time) time.Time` | `ts` | `time.Time` | 返回 UTC 时间 | 当前未被调用 | 无副作用；目前是保留的时间规范化辅助函数 |
| `unexpectedStatus` | `func unexpectedStatus(name string) error` | `name` | `error` | 构造统一错误消息 | 当前未被调用 | 无副作用；目前是保留的错误构造辅助函数 |

## 设计说明

- 仓储层当前没有显式事务封装，`processSample` 的多次写入由服务层按固定顺序驱动
- `UpsertMinuteBucket` 和 `UpsertLedger` 的“是否已存在”采用“先查再写”的接口语义，便于服务层统计 `WrittenMinutes` / `ReplayedMinutes`
- `ensureUTC` 和 `unexpectedStatus` 当前未进入主路径；阅读代码时不要误判为关键业务流程的一部分
