// Package request 收敛「读接口」的查询参数解析，统一处理三类长期被随手写的脏逻辑：
//
//  1. 分页：page / size 这类整型参数，handler 里到处是 `n, _ := strconv.Atoi(...)`，
//     非法输入被静默当成 0，再靠 store 层兜底 —— 错位的防护，且 store 漏兜就出 bug。
//  2. 数量：limit / k 这类「决定下游负载」的参数，没有上限时一个超大值就能打爆
//     数据库或向量检索（返回海量结果把内存吃满）。
//  3. 排序：ORDER BY 直接拼接用户输入是经典的 SQL 注入入口；必须走白名单。
//
// 本包不依赖任何业务模块，作为基础设施供各模块 import；模块隔离检查的白名单里
// 已包含 `starry/backend/internal/request`。
package request

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Page 解析页码。缺省或非法（非正整数）一律回落到 1，从不为 0 ——
// 0 意味着「不做 offset」的语义歧义，历史上正是这类静默 0 让分页行为不可预期。
func Page(c *gin.Context) int {
	n, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// Size 解析每页条数，钳制到 [1, maxSize]；缺省为 defaultSize。
// maxSize 必须 > 0，否则退化为 defaultSize —— 调用方配置错误绝不能变成「不限」。
func Size(c *gin.Context, defaultSize, maxSize int) int {
	if maxSize < 1 {
		return defaultSize
	}
	n, err := strconv.Atoi(c.DefaultQuery("size", strconv.Itoa(defaultSize)))
	if err != nil || n < 1 {
		return defaultSize
	}
	if n > maxSize {
		return maxSize
	}
	return n
}

// IntParam 解析任意整型查询参数并钳制到 [min, max]；缺省或非法按 def。
// 典型用途是 limit / k 这类「数量」参数，给一个硬上限防止超大值击穿下游。
// 当 max < min 时视为「不设上限」，仅做 min 下限钳制（调用方须明确如此意图）。
func IntParam(c *gin.Context, key string, def, min, max int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if max >= min && n > max {
		return max
	}
	return n
}

// Sort 把用户传入的排序键映射成**白名单内的安全 SQL 片段**。
//
// allowed 形如 {"updated_at": "updated_at", "title": "title"} —— 值才是真正进 SQL
// 的列名，键是用户可写的别名。返回形如 `updated_at DESC`，绝不会把用户输入原样
// 拼进 ORDER BY（那是教科书级注入入口）。非法键回落 defaultSQL；方向只接受 ASC，
// 其余（含空）一律 DESC，不接受任何其它方向字符串。
func Sort(c *gin.Context, allowed map[string]string, defaultSQL string) string {
	col, ok := allowed[c.Query("sort")]
	if !ok {
		return defaultSQL
	}
	dir := "DESC"
	if strings.EqualFold(c.Query("order"), "ASC") {
		dir = "ASC"
	}
	return col + " " + dir
}
