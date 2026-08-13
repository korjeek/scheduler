//go:build unit

package scheduler_test

import (
	"errors"
	"scheduler/internal/usecases/scheduler/mocks"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scheduler/internal/usecases/scheduler"
)

func TestComputeNextRun(t *testing.T) {
	fixedTime := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	newMockSchedule := func(nextTime time.Time) cron.Schedule {
		return &mockSchedule{next: nextTime}
	}

	tests := []struct {
		name        string
		cronExpr    string
		runAt       *time.Time
		setupMock   func(parser *mocks.MockCronParser)
		wantNext    *time.Time
		wantErr     bool
		errContains string
	}{
		{
			name:     "cron expression ok, returns next from schedule",
			cronExpr: "@every 5m",
			setupMock: func(m *mocks.MockCronParser) {
				m.EXPECT().Parse("@every 5m").Return(newMockSchedule(fixedTime.Add(5*time.Minute)), nil)
			},
			wantNext: timePtr(fixedTime.Add(5 * time.Minute)),
		},
		{
			name:     "cron parse error",
			cronExpr: "invalid",
			setupMock: func(m *mocks.MockCronParser) {
				m.EXPECT().Parse("invalid").Return(nil, errors.New("bad expression"))
			},
			wantErr:     true,
			errContains: "bad expression",
		},
		{
			name:     "no cron, runAt provided – returns runAt in UTC",
			cronExpr: "",
			runAt:    timePtr(time.Date(2026, 8, 10, 14, 30, 0, 0, time.FixedZone("MSK", 3*3600))),
			wantNext: timePtr(time.Date(2026, 8, 10, 11, 30, 0, 0, time.UTC)),
		},
		{
			name:     "no cron, runAt nil – returns nil, nil",
			cronExpr: "",
			runAt:    nil,
			wantNext: nil,
		},
		{
			name:     "cron empty, runAt is zero time (still returns it in UTC)",
			cronExpr: "",
			runAt:    timePtr(time.Time{}),
			wantNext: timePtr(time.Time{}.UTC()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockParser := mocks.NewMockCronParser(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockParser)
			}

			s := scheduler.NewTaskScheduler(mockParser)
			gotNext, err := s.ComputeNextRun(tt.cronExpr, tt.runAt)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				if tt.wantNext == nil {
					assert.Nil(t, gotNext)
				} else {
					require.NotNil(t, gotNext)
					assert.WithinDuration(t, *tt.wantNext, *gotNext, time.Second,
						"expected %v, got %v", tt.wantNext, gotNext)
				}
			}
		})
	}
}

type mockSchedule struct {
	next time.Time
}

func (m *mockSchedule) Next(time.Time) time.Time {
	return m.next
}

func timePtr(t time.Time) *time.Time {
	return &t
}
