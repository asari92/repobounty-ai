package http

import (
	"testing"
	"time"
)

func TestNormalizeAutoFinalizeIntervalDefaultsToOneMinute(t *testing.T) {
	if got := normalizeAutoFinalizeInterval(0); got != time.Minute {
		t.Fatalf("normalizeAutoFinalizeInterval(0) = %s, want %s", got, time.Minute)
	}
}

func TestNormalizeAutoFinalizeIntervalDefaultsToOneMinuteForNegativeDuration(t *testing.T) {
	if got := normalizeAutoFinalizeInterval(-time.Second); got != time.Minute {
		t.Fatalf("normalizeAutoFinalizeInterval(-1s) = %s, want %s", got, time.Minute)
	}
}

func TestNormalizeAutoFinalizeIntervalPreservesExplicitValue(t *testing.T) {
	if got := normalizeAutoFinalizeInterval(90 * time.Second); got != 90*time.Second {
		t.Fatalf("normalizeAutoFinalizeInterval(90s) = %s, want %s", got, 90*time.Second)
	}
}
