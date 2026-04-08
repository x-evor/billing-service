package model

import "time"

type Sample struct {
	UUID               string `json:"uuid"`
	Email              string `json:"email"`
	InboundTag         string `json:"inbound_tag"`
	UplinkBytesTotal   int64  `json:"uplink_bytes_total"`
	DownlinkBytesTotal int64  `json:"downlink_bytes_total"`
}

type Snapshot struct {
	CollectedAt time.Time `json:"collected_at"`
	NodeID      string    `json:"node_id"`
	Env         string    `json:"env"`
	Samples     []Sample  `json:"samples"`
}

type Checkpoint struct {
	NodeID            string
	AccountUUID       string
	LastUplinkTotal   int64
	LastDownlinkTotal int64
	LastSeenAt        time.Time
	XrayRevision      string
	ResetEpoch        int64
}

type MinuteBucket struct {
	BucketStart    time.Time
	NodeID         string
	AccountUUID    string
	Region         string
	LineCode       string
	UplinkBytes    int64
	DownlinkBytes  int64
	TotalBytes     int64
	Multiplier     float64
	RatingStatus   string
	SourceRevision string
}

type LedgerEntry struct {
	ID                 string
	AccountUUID        string
	BucketStart        time.Time
	BucketEnd          time.Time
	EntryType          string
	RatedBytes         int64
	AmountDelta        float64
	BalanceAfter       float64
	PricingRuleVersion string
}

type QuotaState struct {
	AccountUUID            string
	RemainingIncludedQuota int64
	CurrentBalance         float64
	Arrears                bool
	ThrottleState          string
	SuspendState           string
	LastRatedBucketAt      *time.Time
	EffectiveAt            time.Time
}

type JobResult struct {
	Job              string    `json:"job"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	ProcessedSamples int       `json:"processed_samples"`
	WrittenMinutes   int       `json:"written_minutes"`
	ReplayedMinutes  int       `json:"replayed_minutes"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
}
