package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func testJWTWithClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestParseIDTokenClaimsExtractsSubscriptionWindow(t *testing.T) {
	idToken := testJWTWithClaims(t, map[string]any{
		"email": "alice@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id":                "acc_123",
			"chatgpt_plan_type":                 "pro",
			"chatgpt_subscription_active_start": "2026-05-01T00:00:00Z",
			"chatgpt_subscription_active_until": "2026-06-01T00:00:00Z",
		},
	})

	accountID, email, planType, activeStart, activeUntil := parseIDTokenClaims(idToken)

	if accountID != "acc_123" {
		t.Fatalf("accountID = %q", accountID)
	}
	if email != "alice@example.com" {
		t.Fatalf("email = %q", email)
	}
	if planType != "pro" {
		t.Fatalf("planType = %q", planType)
	}
	if activeStart != "2026-05-01T00:00:00Z" {
		t.Fatalf("activeStart = %q", activeStart)
	}
	if activeUntil != "2026-06-01T00:00:00Z" {
		t.Fatalf("activeUntil = %q", activeUntil)
	}
}
