package captchahttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开验证码路由。
func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	public.GET("/captcha/image", handler.GetImageCaptcha)
}
