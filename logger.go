package logger

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

type Logger struct {
	*zap.SugaredLogger
}

type Config struct {
	LogLevel    string   `yaml:"log_level"`
	Development bool     `yaml:"development"`
	Encoding    string   `yaml:"encoding"`
	OutputPaths []string `yaml:"output_paths"`
}

func New(cfg Config) (*Logger, error) {
	if cfg.Encoding == "" {
		cfg.Encoding = "json"
	}

	logLevel := zapcore.InfoLevel
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, err
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	if cfg.Development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	}
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(logLevel),
		Development:       cfg.Development,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          cfg.Encoding, // Убедитесь, что это установлено
		EncoderConfig:     encoderConfig,
		OutputPaths:       append(cfg.OutputPaths, "stderr"),
		ErrorOutputPaths:  []string{"stderr"},
	}

	zapLogger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{zapLogger.Sugar()}, nil
}

func NewDefault() *Logger {
	zapLogger, _ := zap.NewProduction()
	return &Logger{zapLogger.Sugar()}
}

func (l *Logger) With(fields ...interface{}) *Logger {
	return &Logger{l.SugaredLogger.With(fields...)}
}

func (l *Logger) Named(name string) *Logger {
	return &Logger{l.SugaredLogger.Named(name)}
}

func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}

func GinLogger(log *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				log.Errorw(e, "status", c.Writer.Status(),
					"method", c.Request.Method,
					"path", path,
					"query", query,
					"ip", c.ClientIP(),
					"user-agent", c.Request.UserAgent(),
					"latency", latency,
				)
			}
		} else {
			log.Infow("HTTP request",
				"status", c.Writer.Status(),
				"method", c.Request.Method,
				"path", path,
				"query", query,
				"ip", c.ClientIP(),
				"user-agent", c.Request.UserAgent(),
				"latency", latency,
			)
		}
	}
}

func GRPCLoggingInterceptor(log *Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)

		if err != nil {
			log.Errorw("gRPC request failed",
				"method", info.FullMethod,
				"duration", time.Since(start),
				"error", err,
			)
		} else {
			log.Infow("gRPC request",
				"method", info.FullMethod,
				"duration", time.Since(start),
			)
		}

		return resp, err
	}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}
	if reqID, ok := ctx.Value("request_id").(string); ok {
		return l.With("request_id", reqID)
	}
	return l
}
