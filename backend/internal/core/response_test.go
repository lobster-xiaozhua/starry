package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestResponseEnvelope 锁定全局 API 响应契约：所有模块共用同一信封格式，
// 前端依赖 code=0 判定成功，因此这里任一字段变化都是破坏性变更。
func TestResponseEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("OK", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		OK(c, gin.H{"id": "abc"})

		if w.Code != http.StatusOK {
			t.Fatalf("期望 200，实际 %d", w.Code)
		}
		var got Response
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("响应不是合法 JSON: %v", err)
		}
		if got.Code != 0 || got.Message != "ok" {
			t.Fatalf("期望 code=0 message=ok，实际 %+v", got)
		}
		if got.Data == nil {
			t.Fatal("data 不应为 nil")
		}
	})

	t.Run("Fail", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		Fail(c, http.StatusBadRequest, 1004, "参数错误")

		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 HTTP 400，实际 %d", w.Code)
		}
		var got Response
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got.Code != 1004 || got.Message != "参数错误" {
			t.Fatalf("期望业务码 1004，实际 %+v", got)
		}
	})

	t.Run("FailWithField", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		FailWithField(c, http.StatusBadRequest, 1004, "密码不符合策略", map[string]string{"password": "太短"})

		var got struct {
			Code int `json:"code"`
			Data struct {
				Fields map[string]string `json:"fields"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("响应不是合法 JSON: %v", err)
		}
		if got.Data.Fields["password"] != "太短" {
			t.Fatalf("期望字段级错误明细，实际 %+v", got.Data.Fields)
		}
	})
}
