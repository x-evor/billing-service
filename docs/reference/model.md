# model 包参考

本文档覆盖 `internal/model/types.go`。该包只定义结构体，不包含行为函数，是 `exporter`、`repository`、`service`、`httpapi` 共享的数据契约层。

## 1. 上游快照模型

### `Sample`

签名：`type Sample struct`

| 字段 | 类型 | JSON 字段 | 含义 |
| --- | --- | --- | --- |
| `UUID` | `string` | `uuid` | 账户 UUID，服务层会校验其为合法 UUID |
| `Email` | `string` | `email` | 上游样本附带的邮箱，当前服务层不使用此字段做计费决策 |
| `InboundTag` | `string` | `inbound_tag` | 线路标签，写入分钟桶的 `line_code` |
| `UplinkBytesTotal` | `int64` | `uplink_bytes_total` | 截止采样时累计上行字节 |
| `DownlinkBytesTotal` | `int64` | `downlink_bytes_total` | 截止采样时累计下行字节 |

### `Snapshot`

签名：`type Snapshot struct`

| 字段 | 类型 | JSON 字段 | 含义 |
| --- | --- | --- | --- |
| `CollectedAt` | `time.Time` | `collected_at` | 快照采集时间，服务层会按分钟截断 |
| `NodeID` | `string` | `node_id` | 上游节点标识 |
| `Env` | `string` | `env` | 环境标识，例如 `prod` |
| `Samples` | `[]Sample` | `samples` | 本次快照中的样本列表 |

### `SnapshotWindowPage`

签名：`type SnapshotWindowPage struct`

| 字段 | 类型 | JSON 字段 | 含义 |
| --- | --- | --- | --- |
| `NodeID` | `string` | `node_id` | 分页级别的节点标识 |
| `Env` | `string` | `env` | 分页级别的环境标识 |
| `Snapshots` | `[]Snapshot` | `snapshots` | 窗口拉取结果中的快照列表 |
| `HasMore` | `bool` | `has_more` | 是否还有下一页 |
| `NextCursor` | `string` | `next_cursor` | 下一页游标，当前实现要求其为 RFC3339 时间字符串 |

## 2. 持久化实体模型

### `Checkpoint`

签名：`type Checkpoint struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `NodeID` | `string` | 存储层节点 ID，实际由 `env:node_id` 组合而成 |
| `AccountUUID` | `string` | 账户 UUID |
| `LastUplinkTotal` | `int64` | 上次记录的累计上行字节 |
| `LastDownlinkTotal` | `int64` | 上次记录的累计下行字节 |
| `LastSeenAt` | `time.Time` | 上次成功处理该样本的采样时间 |
| `XrayRevision` | `string` | 当前写路径版本标识 |
| `ResetEpoch` | `int64` | 检测到累计值回退后的重置计数 |

### `MinuteBucket`

签名：`type MinuteBucket struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `BucketStart` | `time.Time` | 分钟桶起始时间，按 UTC 分钟对齐 |
| `NodeID` | `string` | 存储层节点 ID |
| `AccountUUID` | `string` | 账户 UUID |
| `Region` | `string` | 地域代码，当前来自 `cfg.DefaultRegion` |
| `LineCode` | `string` | 线路代码，当前来自 `sample.InboundTag` |
| `UplinkBytes` | `int64` | 当前分钟增量上行字节 |
| `DownlinkBytes` | `int64` | 当前分钟增量下行字节 |
| `TotalBytes` | `int64` | 当前分钟总字节数 |
| `Multiplier` | `float64` | 最终乘数，当前为地域乘数 * 线路乘数 |
| `RatingStatus` | `string` | 当前实现固定写 `rated` |
| `SourceRevision` | `string` | 写路径版本或计费规则版本 |

### `LedgerEntry`

签名：`type LedgerEntry struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `ID` | `string` | 账本主键，基于分钟桶确定性生成 |
| `AccountUUID` | `string` | 账户 UUID |
| `BucketStart` | `time.Time` | 对应分钟桶起始时间 |
| `BucketEnd` | `time.Time` | 对应分钟桶结束时间，当前实现为 `BucketStart + 1 分钟` |
| `EntryType` | `string` | 当前实现固定为 `traffic_charge` |
| `RatedBytes` | `int64` | 真正参与计费的字节数，已扣除包含流量 |
| `AmountDelta` | `float64` | 本次金额变化，当前为负值表示扣费 |
| `BalanceAfter` | `float64` | 扣费后的余额 |
| `PricingRuleVersion` | `string` | 实际应用的定价规则版本 |

### `QuotaState`

签名：`type QuotaState struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `AccountUUID` | `string` | 账户 UUID |
| `RemainingIncludedQuota` | `int64` | 剩余包含流量字节数 |
| `CurrentBalance` | `float64` | 当前余额 |
| `Arrears` | `bool` | 是否欠费 |
| `ThrottleState` | `string` | 节流状态，当前实现用 `normal` / `throttled` |
| `SuspendState` | `string` | 挂起状态，当前新建状态默认 `active` |
| `LastRatedBucketAt` | `*time.Time` | 最近一次成功计费的分钟桶时间 |
| `EffectiveAt` | `time.Time` | 当前状态生效时间 |

### `BillingProfile`

签名：`type BillingProfile struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `AccountUUID` | `string` | 账户 UUID |
| `PackageName` | `string` | 套餐名 |
| `IncludedQuotaBytes` | `int64` | 套餐包含流量 |
| `BasePricePerByte` | `float64` | 基础单价 |
| `RegionMultiplier` | `float64` | 地域乘数 |
| `LineMultiplier` | `float64` | 线路乘数 |
| `PeakMultiplier` | `float64` | 峰时乘数，当前代码未参与计算 |
| `OffPeakMultiplier` | `float64` | 非峰时乘数，当前代码未参与计算 |
| `PricingRuleVersion` | `string` | 计费规则版本号 |

### `SourceSyncState`

签名：`type SourceSyncState struct`

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `SourceID` | `string` | 来源 ID |
| `LastCompletedUntil` | `*time.Time` | 最近一次成功推进到的窗口终点 |
| `LastAttemptedAt` | `*time.Time` | 最近一次开始尝试采集的时间 |
| `LastSucceededAt` | `*time.Time` | 最近一次成功完成采集的时间 |
| `LastError` | `string` | 最近一次失败信息 |

## 3. HTTP 与运行状态模型

### `SourceStatus`

签名：`type SourceStatus struct`

| 字段 | 类型 | JSON 字段 | 含义 |
| --- | --- | --- | --- |
| `SourceID` | `string` | `source_id` | 来源 ID |
| `LastCompletedUntil` | `*time.Time` | `last_completed_until` | 对外暴露的窗口完成位置 |
| `LastAttemptedAt` | `*time.Time` | `last_attempted_at` | 对外暴露的最近尝试时间 |
| `LastSucceededAt` | `*time.Time` | `last_succeeded_at` | 对外暴露的最近成功时间 |
| `LastError` | `string` | `last_error` | 最近错误 |

### `JobResult`

签名：`type JobResult struct`

| 字段 | 类型 | JSON 字段 | 含义 |
| --- | --- | --- | --- |
| `Job` | `string` | `job` | 作业名，当前为 `collect-and-rate` 或 `reconcile` |
| `StartedAt` | `time.Time` | `started_at` | 作业开始时间 |
| `FinishedAt` | `time.Time` | `finished_at` | 作业结束时间 |
| `ProcessedSamples` | `int` | `processed_samples` | 成功进入处理主路径的样本数 |
| `WrittenMinutes` | `int` | `written_minutes` | 新写入的分钟桶数 |
| `ReplayedMinutes` | `int` | `replayed_minutes` | 由于回放或重复写命中的分钟数 |
| `Status` | `string` | `status` | `ok` / `partial` / `error` |
| `Error` | `string` | `error` | 汇总错误信息 |
| `SourceStatuses` | `[]SourceStatus` | `source_statuses` | 各来源状态摘要 |

### `PingInfo`

签名：`type PingInfo struct`

| 字段 | 类型 | JSON 字段 | 含义 |
| --- | --- | --- | --- |
| `Image` | `string` | `image` | 原始镜像引用 |
| `Tag` | `string` | `tag` | 解析出的镜像 tag |
| `Commit` | `string` | `commit` | 解析出的完整 commit |
| `Version` | `string` | `version` | 当前实现等于 commit |

## 4. 使用约束

- 所有 `time.Time` 在服务层和仓储层都按 UTC 处理
- `Snapshot` / `Sample` 是上游输入契约，不应被仓储层直接修改
- `MinuteBucket`、`LedgerEntry`、`QuotaState` 共同构成计费事实链
- `JobResult` 和 `SourceStatus` 只代表最近一次执行的内存快照，不是持久化审计日志
