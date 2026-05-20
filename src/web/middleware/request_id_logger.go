package middleware

import (
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewGinZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return ginzap.GinzapWithConfig(logger, &ginzap.Config{
		TimeFormat: time.RFC3339,
		UTC:        true,
		Context: ginzap.Fn(func(c *gin.Context) []zapcore.Field {
			fields := []zapcore.Field{}
			if requestID := c.Writer.Header().Get("X-Request-Id"); requestID != "" {
				fields = append(fields, zap.String("request_id", requestID))
			}
			//var body []byte
			//var buf bytes.Buffer
			//tee := io.TeeReader(c.Request.Body, &buf)
			//body, err := io.ReadAll(tee)
			//fmt.Printf("读取body错误: %v\n", err)
			//c.Request.Body = io.NopCloser(&buf)
			//authHeader := c.Request.Header.Get("Authorization")
			//fields = append(fields, zap.String("body", string(body)))
			//fields = append(fields, zap.String("Authorization", string(authHeader)))
			return fields
		}),
	})
}
