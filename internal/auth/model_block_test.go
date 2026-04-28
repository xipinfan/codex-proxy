package auth

import (
	"testing"
	"time"
)

func TestAccountModelBlockAfterThreeQualifyingFailures(t *testing.T) {
	acc := &Account{}
	base := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 2; i++ {
		blocked := acc.RecordModelAccessFailure("gpt-5.5", base.Add(time.Duration(i)*time.Second))
		if blocked {
			t.Fatalf("failure %d blocked too early", i+1)
		}
		if acc.IsModelBlocked("gpt-5.5", base.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("model blocked before threshold")
		}
	}
	if !acc.RecordModelAccessFailure("gpt-5.5", base.Add(3*time.Second)) {
		t.Fatalf("third qualifying failure should create a new block")
	}
	if !acc.IsModelBlocked("gpt-5.5", base.Add(24*time.Hour)) {
		t.Fatalf("model should be blocked during 7 day window")
	}
	if acc.IsModelBlocked("gpt-5.5", base.Add(8*24*time.Hour+time.Second)) {
		t.Fatalf("model block should expire after 7 days")
	}
}

func TestAccountModelBlockIsPerModelAndSuccessClears(t *testing.T) {
	acc := &Account{}
	now := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		acc.RecordModelAccessFailure("gpt-5.5", now.Add(time.Duration(i)*time.Second))
	}
	if !acc.IsModelBlocked("gpt-5.5", now.Add(time.Hour)) {
		t.Fatalf("gpt-5.5 should be blocked")
	}
	if acc.IsModelBlocked("gpt-5.4", now.Add(time.Hour)) {
		t.Fatalf("different model should remain available")
	}
	acc.ClearModelAccessFailure("gpt-5.5")
	if acc.IsModelBlocked("gpt-5.5", now.Add(time.Hour)) {
		t.Fatalf("success clear should unblock the model")
	}
}

func TestPickExcludingSkipsBlockedAccountOnlyForRequestedModel(t *testing.T) {
	m := newPolicyTestManager(t)
	m.selector = NewFillFirstSelector()
	blocked := addPolicyTestAccount(m, "blocked@example.com")
	other := addPolicyTestAccount(m, "other@example.com")
	now := time.Now()
	for i := 0; i < 3; i++ {
		blocked.RecordModelAccessFailure("gpt-5.5", now.Add(time.Duration(i)*time.Second))
	}

	picked, err := m.PickExcluding("gpt-5.5", nil)
	if err != nil {
		t.Fatalf("pick blocked model: %v", err)
	}
	if picked != other {
		t.Fatalf("expected other account for blocked model, got %s", picked.GetEmail())
	}

	picked, err = m.PickExcluding("gpt-5.4", nil)
	if err != nil {
		t.Fatalf("pick different model: %v", err)
	}
	if picked != blocked {
		t.Fatalf("blocked account should remain eligible for other models")
	}
}

func TestPickRecentlySuccessfulSkipsModelBlockedAccount(t *testing.T) {
	m := newPolicyTestManager(t)
	blocked := addPolicyTestAccount(m, "recent-blocked@example.com")
	other := addPolicyTestAccount(m, "recent-other@example.com")
	blocked.lastSuccessUnixMs.Store(time.Now().UnixMilli())
	other.lastSuccessUnixMs.Store(time.Now().Add(-time.Minute).UnixMilli())
	for i := 0; i < 3; i++ {
		blocked.RecordModelAccessFailure("gpt-5.5", time.Now().Add(time.Duration(i)*time.Second))
	}

	picked, err := m.PickRecentlySuccessful("gpt-5.5", nil)
	if err != nil {
		t.Fatalf("pick recently successful: %v", err)
	}
	if picked != other {
		t.Fatalf("expected non-blocked recent account, got %s", picked.GetEmail())
	}
}

func TestRecordModelFailureIfAccessErrorOnlyBlocksQualifyingErrors(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "qualifying@example.com")
	body := []byte(`{"error":{"message":"You do not have access to model gpt-5.5"}}`)
	for i := 0; i < 3; i++ {
		m.RecordModelFailureIfAccessError(acc, "gpt-5.5", 403, body)
	}
	if !acc.IsModelBlocked("gpt-5.5", time.Now()) {
		t.Fatalf("qualifying model access errors should block model")
	}
}

func TestRecordModelFailureIfAccessErrorIgnoresQuotaAndServerErrors(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "ignored@example.com")
	body := []byte(`{"error":{"message":"rate limited for model gpt-5.5"}}`)
	for _, status := range []int{401, 429, 500} {
		for i := 0; i < 3; i++ {
			m.RecordModelFailureIfAccessError(acc, "gpt-5.5", status, body)
		}
	}
	if acc.IsModelBlocked("gpt-5.5", time.Now()) {
		t.Fatalf("non-qualifying statuses should not block model")
	}
}
