//go:build unit

package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"scheduler/internal/infrastructure/server/middleware"
)

func TestUnaryRecoveryInterceptor_RecoversPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := middleware.UnaryRecoveryInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("unexpected")
	}

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestHTTPRecoveryMiddleware_RecoversPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("http panic")
	})
	handler := middleware.HTTPRecoveryMiddleware(logger, next)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
