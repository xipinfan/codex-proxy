package auth

import "testing"

func TestMergePoliciesIncludeDefault401(t *testing.T) {
	rp := mergeRefreshHTTPPolicies(nil)
	p, ok := rp[401]
	if !ok {
		t.Fatalf("expected refresh policy for 401")
	}
	if p.phase != policyPhaseRefreshOnce {
		t.Fatalf("expected refresh 401 phase refresh_once, got %v", p.phase)
	}
	if p.final != HTTPErrorActionCooldown {
		t.Fatalf("expected refresh 401 final cooldown, got %v", p.final)
	}

	qp := mergeQuotaHTTPPolicies(nil)
	p2, ok := qp[401]
	if !ok {
		t.Fatalf("expected quota policy for 401")
	}
	if p2.phase != policyPhaseRefreshOnce {
		t.Fatalf("expected quota 401 phase refresh_once, got %v", p2.phase)
	}
	if p2.final != HTTPErrorActionCooldown {
		t.Fatalf("expected quota 401 final cooldown, got %v", p2.final)
	}
}
