package auth

import (
	"fmt"
	"strings"
	"time"

	codexdb "codex-proxy/internal/db"

	log "github.com/sirupsen/logrus"
)

type accountUsageKeySet map[string]struct{}

func usageAccountKey(accountID, email string) string {
	if id := strings.TrimSpace(accountID); id != "" {
		return "id:" + strings.ToLower(id)
	}
	if em := strings.TrimSpace(email); em != "" {
		return "email:" + strings.ToLower(em)
	}
	return ""
}

func addUsageBucket(dst *UsageBucket, src UsageBucket) {
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
	dst.RequestCount += src.RequestCount
}

func normalizeUsage(inputTokens, outputTokens, totalTokens int64) (int64, int64, int64) {
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	if totalTokens <= 0 {
		totalTokens = inputTokens + outputTokens
	}
	if totalTokens < 0 {
		totalTokens = 0
	}
	return inputTokens, outputTokens, totalTokens
}

func (m *Manager) attachAccountUsageRecorder(acc *Account) {
	if acc == nil {
		return
	}
	_ = acc.GetUsageKey()
	if m == nil || m.db == nil {
		acc.usageRecorder = nil
		return
	}
	acc.usageRecorder = func(inputTokens, outputTokens, totalTokens int64, recordedAt time.Time) {
		m.enqueueUsagePersist(acc, inputTokens, outputTokens, totalTokens, recordedAt)
	}
}

func (m *Manager) enqueueUsagePersist(acc *Account, inputTokens, outputTokens, totalTokens int64, recordedAt time.Time) {
	if m == nil || m.db == nil || acc == nil {
		return
	}
	inputTokens, outputTokens, totalTokens = normalizeUsage(inputTokens, outputTokens, totalTokens)
	if totalTokens <= 0 {
		return
	}
	accountKey := acc.GetUsageKey()
	accountID := acc.GetAccountID()
	email := acc.GetEmail()
	if accountKey == "" {
		return
	}

	select {
	case m.usagePersistSem <- struct{}{}:
		go func() {
			defer func() { <-m.usagePersistSem }()
			if err := m.persistUsageDailyForKey(accountKey, accountID, email, recordedAt, inputTokens, outputTokens, totalTokens); err != nil {
				log.Warnf("usage 聚合写入失败 [%s]: %v", email, err)
			}
		}()
	default:
		/* 背压时改为同步兜底，保证统计准确性优先于完全异步 */
		m.usagePersistSem <- struct{}{}
		defer func() { <-m.usagePersistSem }()
		if err := m.persistUsageDailyForKey(accountKey, accountID, email, recordedAt, inputTokens, outputTokens, totalTokens); err != nil {
			log.Warnf("usage 聚合写入失败 [%s]: %v", email, err)
		}
	}
}

func (m *Manager) persistUsageDaily(accountID, email string, recordedAt time.Time, inputTokens, outputTokens, totalTokens int64) error {
	return m.persistUsageDailyForKey(usageAccountKey(accountID, email), accountID, email, recordedAt, inputTokens, outputTokens, totalTokens)
}

func (m *Manager) persistUsageDailyForKey(accountKey, accountID, email string, recordedAt time.Time, inputTokens, outputTokens, totalTokens int64) error {
	if m == nil || m.db == nil {
		return nil
	}
	if accountKey == "" {
		return nil
	}
	inputTokens, outputTokens, totalTokens = normalizeUsage(inputTokens, outputTokens, totalTokens)
	if totalTokens <= 0 {
		return nil
	}
	if recordedAt.IsZero() {
		recordedAt = time.Now()
	}
	usageDate := recordedAt.Format("2006-01-02")

	switch m.dbDialect {
	case codexdb.DialectMySQL:
		_, err := m.db.Exec(`
INSERT INTO codex_usage_daily (account_key, account_id, email, usage_date, request_count, input_tokens, output_tokens, total_tokens, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, ?, ?, ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))
ON DUPLICATE KEY UPDATE
	account_id = VALUES(account_id),
	email = VALUES(email),
	request_count = request_count + VALUES(request_count),
	input_tokens = input_tokens + VALUES(input_tokens),
	output_tokens = output_tokens + VALUES(output_tokens),
	total_tokens = total_tokens + VALUES(total_tokens),
	updated_at = UTC_TIMESTAMP(6)`,
			accountKey, strings.TrimSpace(accountID), strings.TrimSpace(email), usageDate, inputTokens, outputTokens, totalTokens,
		)
		return err
	case codexdb.DialectSQLite:
		_, err := m.db.Exec(`
INSERT INTO codex_usage_daily (account_key, account_id, email, usage_date, request_count, input_tokens, output_tokens, total_tokens, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(account_key, usage_date) DO UPDATE SET
	account_id = excluded.account_id,
	email = excluded.email,
	request_count = codex_usage_daily.request_count + excluded.request_count,
	input_tokens = codex_usage_daily.input_tokens + excluded.input_tokens,
	output_tokens = codex_usage_daily.output_tokens + excluded.output_tokens,
	total_tokens = codex_usage_daily.total_tokens + excluded.total_tokens,
	updated_at = CURRENT_TIMESTAMP`,
			accountKey, strings.TrimSpace(accountID), strings.TrimSpace(email), usageDate, inputTokens, outputTokens, totalTokens,
		)
		return err
	default:
		_, err := m.db.Exec(`
INSERT INTO codex_usage_daily (account_key, account_id, email, usage_date, request_count, input_tokens, output_tokens, total_tokens, created_at, updated_at)
VALUES ($1, $2, $3, $4, 1, $5, $6, $7, NOW(), NOW())
ON CONFLICT (account_key, usage_date) DO UPDATE SET
	account_id = EXCLUDED.account_id,
	email = EXCLUDED.email,
	request_count = codex_usage_daily.request_count + EXCLUDED.request_count,
	input_tokens = codex_usage_daily.input_tokens + EXCLUDED.input_tokens,
	output_tokens = codex_usage_daily.output_tokens + EXCLUDED.output_tokens,
	total_tokens = codex_usage_daily.total_tokens + EXCLUDED.total_tokens,
	updated_at = NOW()`,
			accountKey, strings.TrimSpace(accountID), strings.TrimSpace(email), usageDate, inputTokens, outputTokens, totalTokens,
		)
		return err
	}
}

func (m *Manager) usageOverview(accountKeySet accountUsageKeySet, now time.Time) (UsageOverview, map[string]UsageOverview, error) {
	out := UsageOverview{}
	perAccount := make(map[string]UsageOverview, len(accountKeySet))
	if m == nil || m.db == nil {
		return out, perAccount, nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	today := now.Format("2006-01-02")
	sevenDayStart := now.AddDate(0, 0, -6).Format("2006-01-02")
	thirtyDayStart := now.AddDate(0, 0, -29).Format("2006-01-02")

	argsRecent := []any{thirtyDayStart}
	queryRecent := `SELECT account_key, usage_date, request_count, input_tokens, output_tokens, total_tokens FROM codex_usage_daily WHERE usage_date >= $1`
	if m.dbDialect == codexdb.DialectMySQL || m.dbDialect == codexdb.DialectSQLite {
		queryRecent = `SELECT account_key, usage_date, request_count, input_tokens, output_tokens, total_tokens FROM codex_usage_daily WHERE usage_date >= ?`
	}

	rows, err := m.db.Query(queryRecent, argsRecent...)
	if err != nil {
		return out, perAccount, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountKey string
		var usageDateRaw any
		var requestCount, inputTokens, outputTokens, totalTokens int64
		if scanErr := rows.Scan(&accountKey, &usageDateRaw, &requestCount, &inputTokens, &outputTokens, &totalTokens); scanErr != nil {
			return out, perAccount, scanErr
		}
		usageDate := normalizeUsageDateValue(usageDateRaw)
		if usageDate == "" {
			return out, perAccount, fmt.Errorf("unexpected usage_date value %T", usageDateRaw)
		}
		bucket := UsageBucket{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
			RequestCount: requestCount,
		}

		if usageDate == today {
			addUsageBucket(&out.Today, bucket)
		}
		if usageDate >= sevenDayStart {
			addUsageBucket(&out.SevenDays, bucket)
		}
		addUsageBucket(&out.ThirtyDays, bucket)

		if _, keep := accountKeySet[accountKey]; keep {
			ov := perAccount[accountKey]
			if usageDate == today {
				addUsageBucket(&ov.Today, bucket)
			}
			if usageDate >= sevenDayStart {
				addUsageBucket(&ov.SevenDays, bucket)
			}
			addUsageBucket(&ov.ThirtyDays, bucket)
			perAccount[accountKey] = ov
		}
	}
	if err = rows.Err(); err != nil {
		return out, perAccount, err
	}

	lifetimeRows, err := m.db.Query(`SELECT account_key, SUM(request_count), SUM(input_tokens), SUM(output_tokens), SUM(total_tokens) FROM codex_usage_daily GROUP BY account_key`)
	if err != nil {
		return out, perAccount, err
	}
	defer lifetimeRows.Close()

	for lifetimeRows.Next() {
		var accountKey string
		var requestCount, inputTokens, outputTokens, totalTokens int64
		if scanErr := lifetimeRows.Scan(&accountKey, &requestCount, &inputTokens, &outputTokens, &totalTokens); scanErr != nil {
			return out, perAccount, scanErr
		}
		bucket := UsageBucket{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
			RequestCount: requestCount,
		}
		addUsageBucket(&out.Lifetime, bucket)
		if _, keep := accountKeySet[accountKey]; keep {
			ov := perAccount[accountKey]
			addUsageBucket(&ov.Lifetime, bucket)
			perAccount[accountKey] = ov
		}
	}
	if err = lifetimeRows.Err(); err != nil {
		return out, perAccount, err
	}

	out.UpdatedAt = now
	for key, ov := range perAccount {
		ov.UpdatedAt = now
		perAccount[key] = ov
	}
	return out, perAccount, nil
}

func (m *Manager) UsageOverviewForAccounts(accounts []*Account, now time.Time) (UsageOverview, map[string]UsageOverview, error) {
	return m.usageOverview(usageAccountKeySetFromAccounts(accounts), now)
}

func usageAccountKeySetFromAccounts(accounts []*Account) accountUsageKeySet {
	keys := make(accountUsageKeySet, len(accounts))
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		key := acc.GetUsageKey()
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	return keys
}

func UsageAccountKeyFromStats(stats AccountStats) string {
	return usageAccountKey(stats.AccountID, stats.Email)
}

func normalizeUsageDateValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return normalizeUsageDateString(x)
	case []byte:
		return normalizeUsageDateString(string(x))
	case time.Time:
		return x.Format("2006-01-02")
	default:
		return normalizeUsageDateString(fmt.Sprint(v))
	}
}

func normalizeUsageDateString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

func ApplyUsageOverview(stats *UsageStats, overview UsageOverview) {
	if stats == nil {
		return
	}
	stats.TodayInputTokens = overview.Today.InputTokens
	stats.TodayOutputTokens = overview.Today.OutputTokens
	stats.TodayTotalTokens = overview.Today.TotalTokens
	stats.TodayRequestCount = overview.Today.RequestCount
	stats.SevenDayInputTokens = overview.SevenDays.InputTokens
	stats.SevenDayOutputTokens = overview.SevenDays.OutputTokens
	stats.SevenDayTotalTokens = overview.SevenDays.TotalTokens
	stats.SevenDayRequestCount = overview.SevenDays.RequestCount
	stats.ThirtyDayInputTokens = overview.ThirtyDays.InputTokens
	stats.ThirtyDayOutputTokens = overview.ThirtyDays.OutputTokens
	stats.ThirtyDayTotalTokens = overview.ThirtyDays.TotalTokens
	stats.ThirtyDayRequestCount = overview.ThirtyDays.RequestCount
	stats.LifetimeInputTokens = overview.Lifetime.InputTokens
	stats.LifetimeOutputTokens = overview.Lifetime.OutputTokens
	stats.LifetimeTotalTokens = overview.Lifetime.TotalTokens
	stats.LifetimeRequestCount = overview.Lifetime.RequestCount
}
