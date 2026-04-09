package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"billing-service/internal/config"
	"billing-service/internal/model"
	"billing-service/internal/repository"

	"github.com/google/uuid"
)

type snapshotSource interface {
	FetchLatestSnapshot(ctx context.Context) (model.Snapshot, error)
}

type Service struct {
	cfg    config.Config
	source snapshotSource
	repo   repository.Repository

	mu         sync.Mutex
	lastResult model.JobResult
	lastOK     bool
	lastError  string
}

func New(cfg config.Config, source snapshotSource, repo repository.Repository) *Service {
	return &Service{
		cfg:    cfg,
		source: source,
		repo:   repo,
	}
}

func (s *Service) Start(ctx context.Context) {
	go func() {
		_, _ = s.RunCollectAndRate(ctx, "collect-and-rate")
		ticker := time.NewTicker(s.cfg.CollectInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.RunCollectAndRate(ctx, "collect-and-rate")
			}
		}
	}()
}

func (s *Service) RunCollectAndRate(ctx context.Context, job string) (model.JobResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	startedAt := time.Now().UTC()
	result := model.JobResult{
		Job:       job,
		StartedAt: startedAt,
		Status:    "ok",
	}

	snapshot, err := s.source.FetchLatestSnapshot(ctx)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		result.FinishedAt = time.Now().UTC()
		s.record(result)
		return result, err
	}

	for _, sample := range snapshot.Samples {
		if err := validateSample(sample); err != nil {
			result.Status = "partial"
			result.Error = joinError(result.Error, err.Error())
			continue
		}

		processed, err := s.processSample(ctx, snapshot, sample, &result)
		if err != nil {
			result.Status = "partial"
			result.Error = joinError(result.Error, err.Error())
			continue
		}
		if processed {
			result.ProcessedSamples++
		}
	}

	result.FinishedAt = time.Now().UTC()
	s.record(result)
	if result.Status == "error" {
		return result, errors.New(result.Error)
	}
	return result, nil
}

func (s *Service) Status() model.JobResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastResult
}

func (s *Service) Health() (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastOK, s.lastError
}

func (s *Service) processSample(ctx context.Context, snapshot model.Snapshot, sample model.Sample, result *model.JobResult) (bool, error) {
	storageNodeID := composeStorageNodeID(snapshot.Env, snapshot.NodeID)
	minuteStart := snapshot.CollectedAt.UTC().Truncate(time.Minute)

	checkpoint, err := s.repo.GetCheckpoint(ctx, storageNodeID, sample.UUID)
	if err != nil {
		return false, fmt.Errorf("get checkpoint %s: %w", sample.UUID, err)
	}

	deltaUplink := sample.UplinkBytesTotal
	deltaDownlink := sample.DownlinkBytesTotal
	resetEpoch := int64(0)
	if checkpoint != nil {
		deltaUplink = sample.UplinkBytesTotal - checkpoint.LastUplinkTotal
		deltaDownlink = sample.DownlinkBytesTotal - checkpoint.LastDownlinkTotal
		resetEpoch = checkpoint.ResetEpoch
	}

	if deltaUplink < 0 || deltaDownlink < 0 {
		resetEpoch++
		err := s.repo.UpsertCheckpoint(ctx, model.Checkpoint{
			NodeID:            storageNodeID,
			AccountUUID:       sample.UUID,
			LastUplinkTotal:   sample.UplinkBytesTotal,
			LastDownlinkTotal: sample.DownlinkBytesTotal,
			LastSeenAt:        snapshot.CollectedAt.UTC(),
			XrayRevision:      s.cfg.SourceRevision,
			ResetEpoch:        resetEpoch,
		})
		if err != nil {
			return false, fmt.Errorf("upsert reset checkpoint %s: %w", sample.UUID, err)
		}
		return false, nil
	}

	totalBytes := deltaUplink + deltaDownlink
	profile, err := s.repo.GetBillingProfile(ctx, sample.UUID)
	if err != nil {
		return false, fmt.Errorf("get billing profile %s: %w", sample.UUID, err)
	}
	effectivePricing := resolvePricing(profile, s.cfg)

	bucket := model.MinuteBucket{
		BucketStart:    minuteStart,
		NodeID:         storageNodeID,
		AccountUUID:    sample.UUID,
		Region:         s.cfg.DefaultRegion,
		LineCode:       strings.TrimSpace(sample.InboundTag),
		UplinkBytes:    deltaUplink,
		DownlinkBytes:  deltaDownlink,
		TotalBytes:     totalBytes,
		Multiplier:     effectivePricing.multiplier,
		RatingStatus:   "rated",
		SourceRevision: effectivePricing.pricingRuleVersion,
	}

	minuteExisted, err := s.repo.UpsertMinuteBucket(ctx, bucket)
	if err != nil {
		return false, fmt.Errorf("upsert minute bucket %s: %w", sample.UUID, err)
	}
	if minuteExisted {
		result.ReplayedMinutes++
	} else {
		result.WrittenMinutes++
	}

	amountDelta := -float64(totalBytes) * s.cfg.PricePerByte
	entry := model.LedgerEntry{
		ID:                 deterministicLedgerID(bucket),
		AccountUUID:        sample.UUID,
		BucketStart:        minuteStart,
		BucketEnd:          minuteStart.Add(time.Minute),
		EntryType:          "traffic_charge",
		RatedBytes:         totalBytes,
		AmountDelta:        amountDelta,
		PricingRuleVersion: s.cfg.SourceRevision,
	}

	quota, err := s.repo.GetQuotaState(ctx, sample.UUID)
	if err != nil {
		return false, fmt.Errorf("get quota state %s: %w", sample.UUID, err)
	}
	if quota == nil {
		quota = &model.QuotaState{
			AccountUUID:            sample.UUID,
			RemainingIncludedQuota: effectivePricing.includedQuotaBytes,
			CurrentBalance:         s.cfg.InitialBalance,
			ThrottleState:          "normal",
			SuspendState:           "active",
			EffectiveAt:            snapshot.CollectedAt.UTC(),
		}
	}

	includedApplied := minInt64(quota.RemainingIncludedQuota, totalBytes)
	chargeableBytes := totalBytes - includedApplied
	amountDelta = -float64(chargeableBytes) * effectivePricing.basePricePerByte * effectivePricing.multiplier
	entry.RatedBytes = chargeableBytes
	entry.AmountDelta = amountDelta
	entry.PricingRuleVersion = effectivePricing.pricingRuleVersion
	entry.BalanceAfter = quota.CurrentBalance + amountDelta

	ledgerExisted, err := s.repo.UpsertLedger(ctx, entry)
	if err != nil {
		return false, fmt.Errorf("upsert ledger %s: %w", sample.UUID, err)
	}

	if !ledgerExisted {
		remainingQuota := quota.RemainingIncludedQuota - includedApplied
		if remainingQuota < 0 {
			remainingQuota = 0
		}
		quota.RemainingIncludedQuota = remainingQuota
		quota.CurrentBalance = entry.BalanceAfter
		quota.Arrears = quota.CurrentBalance < 0
		if quota.Arrears {
			quota.ThrottleState = "throttled"
		} else {
			quota.ThrottleState = "normal"
		}
		quota.EffectiveAt = snapshot.CollectedAt.UTC()
		lastRated := minuteStart
		quota.LastRatedBucketAt = &lastRated
		if err := s.repo.UpsertQuotaState(ctx, *quota); err != nil {
			return false, fmt.Errorf("upsert quota state %s: %w", sample.UUID, err)
		}
	} else {
		result.ReplayedMinutes++
	}

	if err := s.repo.UpsertCheckpoint(ctx, model.Checkpoint{
		NodeID:            storageNodeID,
		AccountUUID:       sample.UUID,
		LastUplinkTotal:   sample.UplinkBytesTotal,
		LastDownlinkTotal: sample.DownlinkBytesTotal,
		LastSeenAt:        snapshot.CollectedAt.UTC(),
		XrayRevision:      s.cfg.SourceRevision,
		ResetEpoch:        resetEpoch,
	}); err != nil {
		return false, fmt.Errorf("upsert checkpoint %s: %w", sample.UUID, err)
	}

	return true, nil
}

type effectivePricing struct {
	includedQuotaBytes int64
	basePricePerByte   float64
	multiplier         float64
	pricingRuleVersion string
}

func resolvePricing(profile *model.BillingProfile, cfg config.Config) effectivePricing {
	pricing := effectivePricing{
		includedQuotaBytes: cfg.InitialIncludedQuotaBytes,
		basePricePerByte:   cfg.PricePerByte,
		multiplier:         1.0,
		pricingRuleVersion: cfg.SourceRevision,
	}
	if profile == nil {
		return pricing
	}
	if profile.IncludedQuotaBytes > 0 {
		pricing.includedQuotaBytes = profile.IncludedQuotaBytes
	}
	if profile.BasePricePerByte > 0 {
		pricing.basePricePerByte = profile.BasePricePerByte
	}
	regionMultiplier := profile.RegionMultiplier
	if regionMultiplier <= 0 {
		regionMultiplier = 1.0
	}
	lineMultiplier := profile.LineMultiplier
	if lineMultiplier <= 0 {
		lineMultiplier = 1.0
	}
	pricing.multiplier = regionMultiplier * lineMultiplier
	if pricing.multiplier <= 0 {
		pricing.multiplier = 1.0
	}
	if strings.TrimSpace(profile.PricingRuleVersion) != "" {
		pricing.pricingRuleVersion = strings.TrimSpace(profile.PricingRuleVersion)
	}
	return pricing
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func validateSample(sample model.Sample) error {
	if strings.TrimSpace(sample.UUID) == "" {
		return fmt.Errorf("sample uuid is required")
	}
	if _, err := uuid.Parse(strings.TrimSpace(sample.UUID)); err != nil {
		return fmt.Errorf("sample uuid %q is not a valid UUID", sample.UUID)
	}
	return nil
}

func deterministicLedgerID(bucket model.MinuteBucket) string {
	key := fmt.Sprintf("%s|%s|%s|%s|%s", bucket.BucketStart.UTC().Format(time.RFC3339), bucket.NodeID, bucket.AccountUUID, bucket.Region, bucket.LineCode)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(key)).String()
}

func composeStorageNodeID(env, nodeID string) string {
	env = strings.TrimSpace(env)
	nodeID = strings.TrimSpace(nodeID)
	if env == "" {
		return nodeID
	}
	return env + ":" + nodeID
}

func joinError(existing, next string) string {
	if existing == "" {
		return next
	}
	return existing + "; " + next
}

func (s *Service) record(result model.JobResult) {
	s.lastResult = result
	s.lastError = result.Error
	s.lastOK = result.Status != "error"
}
