// +build integration

package main

import (
	"testing"
	"time"
)

func TestWhetherOrNotUpload(t *testing.T) {
	tests := map[string]struct {
		now            time.Time
		lastUpdated    time.Time
		targetWeekdays []string
		want           bool
	}{
		"now_is_targetWeekday": {
			now:            time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.Local), // 日曜日
			lastUpdated:    time.Date(2020, 8, 23, 0, 0, 1, 0, time.Local),            // 日曜日
			targetWeekdays: []string{"Sunday"},
			want:           true,
		},
		"now_is_targetWeekday2": {
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.Local), // 月曜日
			lastUpdated:    time.Date(2020, 8, 23, 0, 0, 1, 0, time.Local),            // 日曜日
			targetWeekdays: []string{"Monday"},
			want:           true,
		},
		"lastUpdated_is_after_latest_targetWeekday": { // 今が月曜日で最終更新日が直近の日曜日の午前０時０分０秒以降->更新する必要なし
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.Local), // 月曜日
			lastUpdated:    time.Date(2020, 8, 23, 0, 0, 1, 0, time.Local),            // 日曜日
			targetWeekdays: []string{"Sunday"},
			want:           false,
		},
		"lastUpdated_is_before_latest_targetWeekday": { // 今が月曜日で最終更新日が直近の日曜日の午前０時０分０秒以前->更新する必要あり
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.Local), // 月曜日
			lastUpdated:    time.Date(2020, 8, 22, 23, 59, 59, 0, time.Local),         // 土曜日
			targetWeekdays: []string{"Sunday"},
			want:           true,
		},
		"lastUpdated_is_empty": { // lastUpdatedが空->更新する必要あり
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.Local), // 月曜日
			lastUpdated:    time.Time{},                                               // 0001-01-01 00:00:00 +0000 UTC
			targetWeekdays: []string{"Sunday"},
			want:           true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := MySQLDumper{DumpExecuteDays: tc.targetWeekdays, DumpTime: tc.now}
			got, err := m.whetherOrNotUpload(tc.lastUpdated)
			if err != nil {
				t.Errorf("gotErr: %v", err)
				return
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
