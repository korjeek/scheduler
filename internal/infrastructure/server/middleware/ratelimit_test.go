//go:build unit

package middleware_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"scheduler/internal/infrastructure/server/middleware"
)

func TestUnaryRateLimitInterceptor_Allow(t *testing.T) {
	limiter := rate.NewLimiter(100, 100)
	interceptor := middleware.UnaryRateLimitInterceptor(limiter)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryRateLimitInterceptor_Exceed(t *testing.T) {
	limiter := rate.NewLimiter(0, 0)
	interceptor := middleware.UnaryRateLimitInterceptor(limiter)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}
