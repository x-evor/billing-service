package service

import (
	"context"
	"testing"
	"time"

	"billing-service/internal/config"
	"billing-service/internal/model"
	"billing-service/internal/repository"
)

type fakeSource struct {
	snapshot model.Snapshot
	err      error
}

func (f fakeSource) FetchLatestSnapshot(context.Context) (model.Snapshot, error) {
	return f.snapshot, f.err
}

type memoryRepo struct {
	checkpoints map[string]model.Checkpoint
	buckets     map[string]model.MinuteBucket
	ledgers     map[string]model.LedgerEntry
	quotas      map[string]model.QuotaState
	profiles    map[string]model.BillingProfile
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		checkpoints: map[string]model.Checkpoint{},
		buckets:     map[string]model.MinuteBucket{},
		ledgers:     map[string]model.LedgerEntry{},
		quotas:      map[string]model.QuotaState{},
		profiles:    map[string]model.BillingProfile{},
	}
}

func checkpointKey(nodeID, accountUUID string) string {
	return nodeID + "\x00" + accountUUID
}

func bucketKey(bucket model.MinuteBucket) string {
	return bucket.BucketStart.UTC().Format(time.RFC3339) + "\x00" + bucket.NodeID + "\x00" + bucket.AccountUUID + "\x00" + bucket.Region + "\x00" + bucket.LineCode
}

func (m *memoryRepo) GetCheckpoint(ctx context.Context, nodeID, accountUUID string) (*model.Checkpoint, error) {
	if checkpoint, ok := m.checkpoints[checkpointKey(nodeID, accountUUID)]; ok {
		copy := checkpoint
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryRepo) UpsertCheckpoint(ctx context.Context, checkpoint model.Checkpoint) error {
	m.checkpoints[checkpointKey(checkpoint.NodeID, checkpoint.AccountUUID)] = checkpoint
	return nil
}

func (m *memoryRepo) UpsertMinuteBucket(ctx context.Context, bucket model.MinuteBucket) (bool, error) {
	key := bucketKey(bucket)
	_, existed := m.buckets[key]
	m.buckets[key] = bucket
	return existed, nil
}

func (m *memoryRepo) UpsertLedger(ctx context.Context, entry model.LedgerEntry) (bool, error) {
	_, existed := m.ledgers[entry.ID]
	m.ledgers[entry.ID] = entry
	return existed, nil
}

func (m *memoryRepo) GetQuotaState(ctx context.Context, accountUUID string) (*model.QuotaState, error) {
	if quota, ok := m.quotas[accountUUID]; ok {
		copy := quota
		return &copy, nil
	}
	return nil, nil
}

func (m *memoryRepo) UpsertQuotaState(ctx context.Context, state model.QuotaState) error {
	m.quotas[state.AccountUUID] = state
	return nil
}

func (m *memoryRepo) GetBillingProfile(ctx context.Context, accountUUID string) (*model.BillingProfile, error) {
	if profile, ok := m.profiles[accountUUID]; ok {
		copy := profile
		return &copy, nil
	}
	return nil, nil
}

var _ repository.Repository = (*memoryRepo)(nil)

func baseConfig() config.Config {
	return config.Config{
		DefaultRegion:             "",
		SourceRevision:            "billing-service-v1",
		PricePerByte:              0.5,
		InitialIncludedQuotaBytes: 0,
		InitialBalance:            0,
	}
}

func TestDeltaCalculationAndQuotaUpdate(t *testing.T) {
	repo := newMemoryRepo()
	svc := New(baseConfig(), fakeSource{snapshot: model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 30, 15, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "prod",
		Samples: []model.Sample{{
			UUID:               "11111111-1111-1111-1111-111111111111",
			Email:              "user@example.com",
			InboundTag:         "premium",
			UplinkBytesTotal:   100,
			DownlinkBytesTotal: 50,
		}},
	}}, repo)

	result, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate")
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if result.ProcessedSamples != 1 || result.WrittenMinutes != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	quota := repo.quotas["11111111-1111-1111-1111-111111111111"]
	if quota.CurrentBalance != -75 {
		t.Fatalf("expected current balance -75, got %v", quota.CurrentBalance)
	}
}

func TestIncludedQuotaAndMultipliersFromBillingProfile(t *testing.T) {
	repo := newMemoryRepo()
	accountUUID := "11111111-1111-1111-1111-111111111111"
	repo.profiles[accountUUID] = model.BillingProfile{
		AccountUUID:        accountUUID,
		PackageName:        "starter",
		IncludedQuotaBytes: 100,
		BasePricePerByte:   0.5,
		RegionMultiplier:   1.2,
		LineMultiplier:     2.0,
		PricingRuleVersion: "pricing-v2",
	}
	svc := New(baseConfig(), fakeSource{snapshot: model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 30, 15, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "prod",
		Samples: []model.Sample{{
			UUID:               accountUUID,
			InboundTag:         "premium",
			UplinkBytesTotal:   100,
			DownlinkBytesTotal: 50,
		}},
	}}, repo)

	result, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate")
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if result.ProcessedSamples != 1 || result.WrittenMinutes != 1 {
		t.Fatalf("unexpected result %#v", result)
	}

	quota := repo.quotas[accountUUID]
	if quota.RemainingIncludedQuota != 0 {
		t.Fatalf("expected remaining quota 0, got %d", quota.RemainingIncludedQuota)
	}
	if quota.CurrentBalance != -60 {
		t.Fatalf("expected current balance -60, got %v", quota.CurrentBalance)
	}

	bucket := repo.buckets[bucketKey(model.MinuteBucket{
		BucketStart: time.Date(2026, 4, 8, 10, 30, 0, 0, time.UTC),
		NodeID:      composeStorageNodeID("prod", "jp-node"),
		AccountUUID: accountUUID,
		Region:      "",
		LineCode:    "premium",
	})]
	if bucket.Multiplier != 2.4 {
		t.Fatalf("expected multiplier 2.4, got %v", bucket.Multiplier)
	}
	for _, entry := range repo.ledgers {
		if entry.RatedBytes != 50 {
			t.Fatalf("expected rated bytes 50, got %d", entry.RatedBytes)
		}
		if entry.AmountDelta != -60 {
			t.Fatalf("expected amount delta -60, got %v", entry.AmountDelta)
		}
		if entry.PricingRuleVersion != "pricing-v2" {
			t.Fatalf("expected pricing version pricing-v2, got %q", entry.PricingRuleVersion)
		}
	}
}

func TestDuplicateMinuteIsReplaySafe(t *testing.T) {
	repo := newMemoryRepo()
	snapshot := model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 30, 30, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "prod",
		Samples: []model.Sample{{
			UUID:               "11111111-1111-1111-1111-111111111111",
			Email:              "user@example.com",
			InboundTag:         "premium",
			UplinkBytesTotal:   100,
			DownlinkBytesTotal: 50,
		}},
	}
	svc := New(baseConfig(), fakeSource{snapshot: snapshot}, repo)

	if _, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	result, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result.ReplayedMinutes == 0 {
		t.Fatalf("expected replayed minutes, got %#v", result)
	}
	if len(repo.ledgers) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(repo.ledgers))
	}
}

func TestNegativeDeltaProtection(t *testing.T) {
	repo := newMemoryRepo()
	cfg := baseConfig()
	accountUUID := "11111111-1111-1111-1111-111111111111"
	nodeKey := composeStorageNodeID("prod", "jp-node")
	repo.checkpoints[checkpointKey(nodeKey, accountUUID)] = model.Checkpoint{
		NodeID:            nodeKey,
		AccountUUID:       accountUUID,
		LastUplinkTotal:   200,
		LastDownlinkTotal: 200,
		LastSeenAt:        time.Now().UTC(),
		XrayRevision:      "prev",
		ResetEpoch:        0,
	}
	svc := New(cfg, fakeSource{snapshot: model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 31, 0, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "prod",
		Samples: []model.Sample{{
			UUID:               accountUUID,
			InboundTag:         "premium",
			UplinkBytesTotal:   10,
			DownlinkBytesTotal: 20,
		}},
	}}, repo)

	result, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate")
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if result.ProcessedSamples != 0 {
		t.Fatalf("expected negative delta sample to be skipped, got %#v", result)
	}
	if len(repo.buckets) != 0 || len(repo.ledgers) != 0 {
		t.Fatalf("expected no writes on negative delta")
	}
	if repo.checkpoints[checkpointKey(nodeKey, accountUUID)].ResetEpoch != 1 {
		t.Fatalf("expected reset epoch increment")
	}
}

func TestRestartRecoveryFromCheckpoint(t *testing.T) {
	repo := newMemoryRepo()
	accountUUID := "11111111-1111-1111-1111-111111111111"
	nodeKey := composeStorageNodeID("prod", "jp-node")
	repo.checkpoints[checkpointKey(nodeKey, accountUUID)] = model.Checkpoint{
		NodeID:            nodeKey,
		AccountUUID:       accountUUID,
		LastUplinkTotal:   100,
		LastDownlinkTotal: 100,
		LastSeenAt:        time.Now().UTC(),
	}
	svc := New(baseConfig(), fakeSource{snapshot: model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 32, 0, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "prod",
		Samples: []model.Sample{{
			UUID:               accountUUID,
			InboundTag:         "premium",
			UplinkBytesTotal:   130,
			DownlinkBytesTotal: 140,
		}},
	}}, repo)

	result, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate")
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if result.ProcessedSamples != 1 || result.WrittenMinutes != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	bucket := repo.buckets[bucketKey(model.MinuteBucket{
		BucketStart: time.Date(2026, 4, 8, 10, 32, 0, 0, time.UTC),
		NodeID:      nodeKey,
		AccountUUID: accountUUID,
		Region:      "",
		LineCode:    "premium",
	})]
	if bucket.TotalBytes != 70 {
		t.Fatalf("expected recovered delta 70, got %d", bucket.TotalBytes)
	}
}

func TestMultiEnvIsolation(t *testing.T) {
	repo := newMemoryRepo()
	accountUUID := "11111111-1111-1111-1111-111111111111"
	cfg := baseConfig()

	prodSvc := New(cfg, fakeSource{snapshot: model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 33, 0, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "prod",
		Samples:     []model.Sample{{UUID: accountUUID, InboundTag: "premium", UplinkBytesTotal: 10, DownlinkBytesTotal: 10}},
	}}, repo)
	previewSvc := New(cfg, fakeSource{snapshot: model.Snapshot{
		CollectedAt: time.Date(2026, 4, 8, 10, 33, 0, 0, time.UTC),
		NodeID:      "jp-node",
		Env:         "preview",
		Samples:     []model.Sample{{UUID: accountUUID, InboundTag: "premium", UplinkBytesTotal: 10, DownlinkBytesTotal: 10}},
	}}, repo)

	if _, err := prodSvc.RunCollectAndRate(context.Background(), "collect-and-rate"); err != nil {
		t.Fatalf("prod run: %v", err)
	}
	if _, err := previewSvc.RunCollectAndRate(context.Background(), "collect-and-rate"); err != nil {
		t.Fatalf("preview run: %v", err)
	}
	if len(repo.buckets) != 2 {
		t.Fatalf("expected isolated buckets per env, got %d", len(repo.buckets))
	}
}

func TestLateMinuteReconcileUsesSameMinuteKey(t *testing.T) {
	repo := newMemoryRepo()
	accountUUID := "11111111-1111-1111-1111-111111111111"
	cfg := baseConfig()
	collectedAt := time.Date(2026, 4, 8, 10, 34, 50, 0, time.UTC)
	snapshot := model.Snapshot{
		CollectedAt: collectedAt,
		NodeID:      "jp-node",
		Env:         "prod",
		Samples: []model.Sample{{
			UUID:               accountUUID,
			InboundTag:         "premium",
			UplinkBytesTotal:   20,
			DownlinkBytesTotal: 20,
		}},
	}
	svc := New(cfg, fakeSource{snapshot: snapshot}, repo)

	if _, err := svc.RunCollectAndRate(context.Background(), "collect-and-rate"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	result, err := svc.RunCollectAndRate(context.Background(), "reconcile")
	if err != nil {
		t.Fatalf("reconcile run: %v", err)
	}
	if result.ReplayedMinutes == 0 {
		t.Fatalf("expected reconcile to report replayed minute, got %#v", result)
	}
	if len(repo.buckets) != 1 {
		t.Fatalf("expected single logical minute bucket, got %d", len(repo.buckets))
	}
}
