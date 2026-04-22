package handler

import (
	"encoding/json"
	"strings"

	"github.com/valyala/fasthttp"
)

func (h *ProxyHandler) handleAccountsDelete(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "请使用 POST 请求"},
		})
		return
	}
	var req struct {
		Email    string `json:"email"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体 JSON 解析失败"},
		})
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.FilePath = strings.TrimSpace(req.FilePath)
	if req.Email == "" && req.FilePath == "" {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请提供 email 或 file_path"},
		})
		return
	}
	acc := h.manager.FindAccountByIdentifier(req.Email, req.FilePath)
	if acc == nil {
		writeJSON(ctx, fasthttp.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "未找到对应账号"},
		})
		return
	}

	email := acc.GetEmail()
	filePath := acc.FilePath
	h.manager.RemoveAccount(acc, "admin_delete")

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"deleted": map[string]any{
			"email":     email,
			"file_path": filePath,
		},
		"pool_total": h.manager.AccountCount(),
	})
}
