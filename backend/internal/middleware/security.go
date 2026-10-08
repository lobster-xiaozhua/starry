package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders 补齐浏览器侧的基础防护头。
//
// 反向代理（nginx/Caddy）通常也会加一遍，但两者并不冲突：这些头要么幂等，
// 要么由浏览器取最严格值。把防线放在应用层而不是只依赖网关，
// 是因为「少配了某个 proxy 的一段配置」正是最容易被忽略的失效方式。
//
// hsts 仅在确定对外是 HTTPS 时开启：误发 HSTS 会让该域名的 http 访问在
// max-age 内彻底不可用，回滚成本极高，因此由调用方按部署形态显式决定。
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 禁止 sniff 嗅探内容类型：上传目录 /uploads 里的用户文件同名不同后缀时尤其重要。
		c.Header("X-Content-Type-Options", "nosniff")
		// 本应用没有任何需要被嵌套的场景，直接拒绝 frame/iframe 嵌入，阻断点击劫持。
		c.Header("X-Frame-Options", "DENY")
		// Referrer-Policy：对外链不再泄漏当前页面路径（含 /note/<id> 这类可枚举地址）。
		c.Header("Referrer-Policy", "no-referrer")
		// 显式关闭敏感设备能力，避免第三方脚本或注入内容偷偷申请权限。
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if hsts {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
