package auth

import (
	"database/sql"
	"testing"
	"time"

	codexdb "codex-proxy/internal/db"

	_ "modernc.org/sqlite"
)

func newUsageTestManager(t *testing.T) *Manager {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err = codexdb.SetupSchema(db, codexdb.DialectSQLite); err != nil {
		t.Fatalf("setup schema: %v", err)
	}
	m := NewManager(t.TempDir(), db, "", 3000, NewRoundRobinSelector(), false, &ManagerOptions{
		DBDialect: codexdb.DialectSQLite,
	})
	return m
}

func TestUsageOverviewAggregatesByWindows(t *testing.T) {
	m := newUsageTestManager(t)
	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	if err := m.persistUsageDaily("acc-1", "a@example.com", now, 100, 40, 140); err != nil {
		t.Fatalf("persist usage 1: %v", err)
	}
	if err := m.persistUsageDaily("acc-1", "a@example.com", now, 60, 20, 80); err != nil {
		t.Fatalf("persist usage 2: %v", err)
	}
	if err := m.persistUsageDaily("acc-1", "a@example.com", now.AddDate(0, 0, -3), 200, 100, 300); err != nil {
		t.Fatalf("persist usage 3: %v", err)
	}
	if err := m.persistUsageDaily("acc-1", "a@example.com", now.AddDate(0, 0, -20), 250, 150, 400); err != nil {
		t.Fatalf("persist usage 4: %v", err)
	}
	if err := m.persistUsageDaily("acc-1", "a@example.com", now.AddDate(0, 0, -40), 300, 200, 500); err != nil {
		t.Fatalf("persist usage 5: %v", err)
	}
	if err := m.persistUsageDaily("acc-2", "b@example.com", now, 30, 20, 50); err != nil {
		t.Fatalf("persist usage 6: %v", err)
	}

	accounts := []*Account{
		{Token: TokenData{AccountID: "acc-1", Email: "a@example.com"}},
	}
	summary, perAccount, err := m.UsageOverviewForAccounts(accounts, now)
	if err != nil {
		t.Fatalf("usage overview: %v", err)
	}

	if summary.Today.TotalTokens != 270 || summary.Today.RequestCount != 3 {
		t.Fatalf("unexpected summary today: %+v", summary.Today)
	}
	if summary.SevenDays.TotalTokens != 570 || summary.SevenDays.RequestCount != 4 {
		t.Fatalf("unexpected summary seven days: %+v", summary.SevenDays)
	}
	if summary.ThirtyDays.TotalTokens != 970 || summary.ThirtyDays.RequestCount != 5 {
		t.Fatalf("unexpected summary thirty days: %+v", summary.ThirtyDays)
	}
	if summary.Lifetime.TotalTokens != 1470 || summary.Lifetime.RequestCount != 6 {
		t.Fatalf("unexpected summary lifetime: %+v", summary.Lifetime)
	}

	key := usageAccountKey("acc-1", "a@example.com")
	ov, ok := perAccount[key]
	if !ok {
		t.Fatalf("missing account overview for key %s", key)
	}
	if ov.Today.TotalTokens != 220 || ov.Today.RequestCount != 2 {
		t.Fatalf("unexpected account today: %+v", ov.Today)
	}
	if ov.SevenDays.TotalTokens != 520 || ov.SevenDays.RequestCount != 3 {
		t.Fatalf("unexpected account seven days: %+v", ov.SevenDays)
	}
	if ov.ThirtyDays.TotalTokens != 920 || ov.ThirtyDays.RequestCount != 4 {
		t.Fatalf("unexpected account thirty days: %+v", ov.ThirtyDays)
	}
	if ov.Lifetime.TotalTokens != 1420 || ov.Lifetime.RequestCount != 5 {
		t.Fatalf("unexpected account lifetime: %+v", ov.Lifetime)
	}
}

func TestAttachAccountUsageRecorderPersistsRecordUsage(t *testing.T) {
	m := newUsageTestManager(t)
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	acc := &Account{
		Token: TokenData{
			AccountID: "acc-3",
			Email:     "recorder@example.com",
		},
	}
	m.attachAccountUsageRecorder(acc)
	acc.RecordUsage(10, 5, 0)

	accounts := []*Account{acc}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		summary, _, err := m.UsageOverviewForAccounts(accounts, now)
		if err == nil && summary.Lifetime.TotalTokens == 15 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("record usage was not persisted in time")
}

func TestRecordUsageWaitsForPersistCapacityInsteadOfDropping(t *testing.T) {
	m := newUsageTestManager(t)
	now := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	acc := &Account{
		Token: TokenData{
			AccountID: "acc-4",
			Email:     "backpressure@example.com",
		},
	}
	m.attachAccountUsageRecorder(acc)

	// Saturate the semaphore first to reproduce backpressure.
	m.usagePersistSem <- struct{}{}
	go func() {
		time.Sleep(75 * time.Millisecond)
		<-m.usagePersistSem
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		acc.RecordUsage(7, 8, 0)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("record usage did not finish after persist capacity was released")
	}

	accounts := []*Account{acc}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		summary, _, err := m.UsageOverviewForAccounts(accounts, now)
		if err == nil && summary.Lifetime.TotalTokens == 15 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("record usage was dropped under persist backpressure")
}

func TestUsageOverviewKeepsPerAccountHistoryAfterAccountIDAppears(t *testing.T) {
	m := newUsageTestManager(t)
	now := time.Now()
	acc := &Account{
		Token: TokenData{
			Email: "identity-shift@example.com",
		},
	}
	m.attachAccountUsageRecorder(acc)

	acc.RecordUsage(10, 5, 0)
	waitForUsageTotal(t, m, []*Account{acc}, now, 15)

	acc.UpdateToken(TokenData{
		AccountID: "acc-identity-shift",
		Email:     "identity-shift@example.com",
	})
	acc.RecordUsage(6, 4, 0)

	accounts := []*Account{acc}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, perAccount, err := m.UsageOverviewForAccounts(accounts, now)
		if err == nil {
			key := acc.GetUsageKey()
			if ov, ok := perAccount[key]; ok && ov.Lifetime.TotalTokens == 25 && ov.Today.TotalTokens == 25 {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("per-account overview split after account_id appeared")
}

func waitForUsageTotal(t *testing.T, m *Manager, accounts []*Account, now time.Time, want int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		summary, _, err := m.UsageOverviewForAccounts(accounts, now)
		if err == nil && summary.Lifetime.TotalTokens == want {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("usage total did not reach %d in time", want)
}
