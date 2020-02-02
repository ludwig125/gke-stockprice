package retry

import (
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestRetry(t *testing.T) {
	interval := 1 * time.Microsecond
	var count int
	// countが3未満ならエラーを返すテスト用関数
	testFn := func() error {
		if count < 3 {
			count++
			return errors.New("error")
		}
		return nil
	}

	cases := []struct {
		name    string
		limit   int
		fn      func() error
		wantErr error
	}{
		{
			name:    "fail_reach_retry_limit",
			limit:   3,
			fn:      testFn,
			wantErr: errors.New("error"),
		},
		{
			name:    "success_not_reach_retry_limit",
			limit:   4,
			fn:      testFn,
			wantErr: nil,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			count = 0
			if err := Retry(tt.limit, interval, tt.fn); err != nil {
				// エラー文が想定通りか比較
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("got error: %#v, want error: %#v", err.Error(), tt.wantErr.Error())
				}
			}
		})
	}
}
