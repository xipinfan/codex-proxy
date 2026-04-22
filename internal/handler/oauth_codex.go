package handler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"codex-proxy/internal/auth"

	"github.com/valyala/fasthttp"
)

func (h *ProxyHandler) handleCodexOAuthStart(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "请使用 POST 请求"},
		})
		return
	}
	authorizeURL, state, expiresIn, err := h.codexOAuth.StartAuthorization()
	if err != nil {
		writeJSON(ctx, fasthttp.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error()},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"authorize_url": authorizeURL,
		"state":         state,
		"expires_in":    expiresIn,
	})
}

func (h *ProxyHandler) handleCodexOAuthResult(ctx *fasthttp.RequestCtx) {
	if !ctx.IsGet() {
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "请使用 GET 请求"},
		})
		return
	}

	state := strings.TrimSpace(string(ctx.QueryArgs().Peek("state")))
	if state == "" {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "缺少 state"},
		})
		return
	}

	pollResult, err := h.codexOAuth.PollAuthorizationResult(state)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error()},
		})
		return
	}

	if pollResult.Status == auth.OAuthAuthorizationPending {
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"status": string(auth.OAuthAuthorizationPending),
		})
		return
	}

	if pollResult.Status == auth.OAuthAuthorizationFailed {
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"status":  string(auth.OAuthAuthorizationFailed),
			"message": pollResult.ErrorMessage,
		})
		return
	}

	td := pollResult.TokenData
	if td == nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "授权结果缺少令牌数据"},
		})
		return
	}
	payload := auth.TokenFile{
		Type:         "codex",
		RefreshToken: td.RefreshToken,
		RK:           td.RefreshToken,
		AccessToken:  td.AccessToken,
		IDToken:      td.IDToken,
		AccountID:    td.AccountID,
		Email:        td.Email,
		Expire:       td.Expire,
	}
	body, _ := json.Marshal(payload)
	result, err := h.manager.IngestAccountsFromJSON(body)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error()},
		})
		return
	}

	acc := h.manager.FindAccountByIdentifier(td.Email, "")
	if acc == nil {
		acc = h.manager.FindAccountByAccountID(td.AccountID)
	}
	if acc != nil {
		vctx, vcancel := context.WithTimeout(context.Background(), 40*time.Second)
		validation := h.manager.ValidateAccountAfterIngest(vctx, acc)
		vcancel()
		result.Validation = &validation
	} else {
		validation := auth.IngestValidation{
			Email:     td.Email,
			AccountID: td.AccountID,
			Success:   false,
			Message:   "导入后未在号池中定位到账号，无法执行同步校验",
		}
		result.Validation = &validation
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"status": string(auth.OAuthAuthorizationCompleted),
		"result": result,
	})
}

func (h *ProxyHandler) handleCodexOAuthComplete(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "请使用 POST 请求"},
		})
		return
	}

	var req struct {
		CallbackURL string `json:"callback_url"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体 JSON 解析失败"},
		})
		return
	}
	if req.CallbackURL == "" {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "缺少 callback_url"},
		})
		return
	}

	exchangeCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	td, err := h.codexOAuth.ExchangeCallbackURL(exchangeCtx, req.CallbackURL)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error()},
		})
		return
	}

	payload := auth.TokenFile{
		Type:         "codex",
		RefreshToken: td.RefreshToken,
		RK:           td.RefreshToken,
		AccessToken:  td.AccessToken,
		IDToken:      td.IDToken,
		AccountID:    td.AccountID,
		Email:        td.Email,
		Expire:       td.Expire,
	}
	body, _ := json.Marshal(payload)
	result, err := h.manager.IngestAccountsFromJSON(body)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error()},
		})
		return
	}

	acc := h.manager.FindAccountByIdentifier(td.Email, "")
	if acc == nil {
		acc = h.manager.FindAccountByAccountID(td.AccountID)
	}
	if acc != nil {
		vctx, vcancel := context.WithTimeout(context.Background(), 40*time.Second)
		validation := h.manager.ValidateAccountAfterIngest(vctx, acc)
		vcancel()
		result.Validation = &validation
	} else {
		validation := auth.IngestValidation{
			Email:     td.Email,
			AccountID: td.AccountID,
			Success:   false,
			Message:   "导入后未在号池中定位到账号，无法执行同步校验",
		}
		result.Validation = &validation
	}

	writeJSON(ctx, fasthttp.StatusOK, result)
}
