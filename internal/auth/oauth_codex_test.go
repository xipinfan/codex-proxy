package auth

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestCodexOAuthFlowStoresCallbackResultForPolling(t *testing.T) {
	flow := NewCodexOAuthFlow("", false)
	flow.skipCallbackServer = true

	expected := &TokenData{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		IDToken:      "id-token",
		Email:        "alice@example.com",
		AccountID:    "acct_123",
	}
	flow.codeExchange = func(context.Context, string, string) (*TokenData, error) {
		return expected, nil
	}

	_, state, _, err := flow.StartAuthorization()
	if err != nil {
		t.Fatalf("StartAuthorization() error = %v", err)
	}

	callbackURL := codexOAuthRedirectURI + "?code=oauth-code&state=" + url.QueryEscape(state)
	if err := flow.HandleCallbackURL(context.Background(), callbackURL); err != nil {
		t.Fatalf("HandleCallbackURL() error = %v", err)
	}

	pending, err := flow.PollAuthorizationResult(state)
	if err != nil {
		t.Fatalf("PollAuthorizationResult() error = %v", err)
	}
	if pending.Status != OAuthAuthorizationCompleted {
		t.Fatalf("expected completed status, got %s", pending.Status)
	}
	if pending.TokenData == nil || pending.TokenData.RefreshToken != expected.RefreshToken {
		t.Fatalf("expected refresh token %q, got %#v", expected.RefreshToken, pending.TokenData)
	}

	_, err = flow.PollAuthorizationResult(state)
	if err == nil {
		t.Fatalf("expected state to be consumed after completed poll")
	}
}

func TestCodexOAuthFlowStoresCallbackErrorForPolling(t *testing.T) {
	flow := NewCodexOAuthFlow("", false)
	flow.skipCallbackServer = true
	expectedErr := errors.New("exchange failed")
	flow.codeExchange = func(context.Context, string, string) (*TokenData, error) {
		return nil, expectedErr
	}

	_, state, _, err := flow.StartAuthorization()
	if err != nil {
		t.Fatalf("StartAuthorization() error = %v", err)
	}

	callbackURL := codexOAuthRedirectURI + "?code=oauth-code&state=" + url.QueryEscape(state)
	if err := flow.HandleCallbackURL(context.Background(), callbackURL); err == nil {
		t.Fatalf("expected HandleCallbackURL() to surface exchange error")
	}

	result, err := flow.PollAuthorizationResult(state)
	if err != nil {
		t.Fatalf("PollAuthorizationResult() error = %v", err)
	}
	if result.Status != OAuthAuthorizationFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Fatalf("expected callback error message to be persisted")
	}
}
