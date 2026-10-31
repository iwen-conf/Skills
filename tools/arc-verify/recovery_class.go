package main

// RecoveryClass is the independently observed fixture outcome. A product FAIL
// (defect detected) never implies that the fixture was restored.
type RecoveryClass string

const (
	RecoveryRestored     RecoveryClass = "restored"
	RecoveryFailedLocked RecoveryClass = "failed_lock_retained"
	RecoveryUnknown      RecoveryClass = "unknown_reuse_forbidden"
	RecoveryNotAttempted RecoveryClass = "not_attempted"
)

const (
	concurrencySerialized   = "business_serialized"
	concurrencyConflict     = "business_conflict"
	concurrencyScheduleMiss = "schedule_inconclusive"
)

func classifyRecovery(outcome RecoveryOutcome, locksHeld int) RecoveryClass {
	switch outcome.Verdict {
	case "PASS":
		if outcome.Safe {
			return RecoveryRestored
		}
		return RecoveryUnknown
	case "BLOCKED", "FAIL":
		if locksHeld > 0 && outcome.Evidence != "" {
			return RecoveryFailedLocked
		}
		return RecoveryUnknown
	case "NOT_RUN", "":
		return RecoveryNotAttempted
	default:
		return RecoveryUnknown
	}
}

func mustNotReuseFixture(class RecoveryClass) bool {
	return class != RecoveryRestored
}

func classifyConcurrency(records, capacity, scheduleMisses int) string {
	if scheduleMisses > 0 {
		return concurrencyScheduleMiss
	}
	if capacity >= 0 && records > capacity {
		return concurrencyConflict
	}
	return concurrencySerialized
}
