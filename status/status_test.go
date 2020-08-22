// +build integration

package status

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ludwig125/gke-stockprice/sheet"
)

func TestStatus(t *testing.T) { // Statusの機能全体を通したTest
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sheetCredential := mustGetenv(t, "CREDENTIAL_FILEPATH")
	// spreadsheetのserviceを取得
	srv, err := sheet.GetSheetClient(ctx, "../"+sheetCredential)
	if err != nil {
		t.Fatalf("failed to get sheet service. err: %v", err)
	}

	// daily処理の進捗を管理するためのSheet
	sh := sheet.NewSpreadSheet(srv, mustGetenv(t, "INTEGRATION_TEST_SHEETID"), "status")
	s := Status{Sheet: sh}

	t.Run("ClearStatus", func(t *testing.T) {
		s.ClearStatus()
	})
	t.Run("InsertStatus", func(t *testing.T) {
		s.InsertStatus("task1", time.Date(2020, 1, 3, 23, 59, 59, 0, time.Local), 100*time.Nanosecond)
		s.InsertStatus("task2", time.Date(2020, 1, 4, 0, 0, 0, 0, time.Local), 200*time.Nanosecond)
		s.InsertStatus("task3", time.Date(2020, 1, 4, 0, 0, 1, 0, time.Local), 300*time.Nanosecond)
	})

	// 2020-01-04の午前0時0分0秒を取得
	midnight := getMidnightUnixtime(time.Date(2020, 1, 4, 23, 59, 59, 0, time.Local))

	// 上で入れたtaskのstatusが終了済みかどうかを確認する
	tests := map[string]struct {
		task string
		want bool
	}{
		"task0_is_not_in_status": {
			task: "task0",
			want: false,
		},
		"task1_is_done_yesterday": {
			task: "task1",
			want: false,
		},
		"task2_is_done_today": {
			task: "task2",
			want: true,
		},
		"task3_is_done_today": {
			task: "task3",
			want: true,
		},
	}
	for name, tc := range tests {
		t.Run("IsTaskDoneAfter:"+name, func(t *testing.T) {
			got, err := s.IsTaskDoneAfter(tc.task, midnight) //2020-01-04の午前0時0分0秒以降に終わっているかどうかの確認
			if err != nil {
				t.Fatalf("failed to IsTaskDoneAfter:%v", err)
			}
			if got != tc.want {
				t.Errorf("got: %v, want: %v", got, tc.want)
			}
		})
	}

	tests2 := map[string]struct {
		task string
		want string
	}{
		"task0_is_not_in_status": { // まだstatusにないので実行される
			task: "task0",
			want: "task0",
		},
		"task1_is_not_done_yet": { // 今日の分はまだ実行されていないので実行する
			task: "task1",
			want: "task1",
		},
		"task2_is_done_today": { // 今日は実行済みなので何もしない
			task: "task2",
			want: "",
		},
		"task3_is_done_today": { // 今日は実行済みなので何もしない
			task: "task3",
			want: "",
		},
	}
	for name, tc := range tests2 {
		t.Run("ExecIfIncompleteThisDay:"+name, func(t *testing.T) {
			got := ""
			// 現在時刻を2020-01-04の午前5時0分0秒とする
			thisTime := time.Date(2020, 1, 4, 5, 0, 0, 0, time.Local)
			err := s.ExecIfIncompleteThisDay(tc.task, thisTime, func() error {
				got = tc.task // ExecIfIncompleteThisDayに設定したこの関数が実行されるとtask名でgotを上書きする
				return nil
			})
			if err != nil {
				t.Fatalf("failed to ExecIfIncompleteThisDay:%v", err)
			}
			if got != tc.want {
				t.Errorf("got: %s, want: %s", got, tc.want)
			}
		})
	}

	// 実行後はすべてtrueのはず
	tests3 := map[string]struct {
		task string
		want bool
	}{
		"task0_is_not_in_status": {
			task: "task0",
			want: true,
		},
		"task1_is_done_yesterday": {
			task: "task1",
			want: true,
		},
		"task2_is_done_today": {
			task: "task2",
			want: true,
		},
		"task3_is_done_today": {
			task: "task3",
			want: true,
		},
	}
	for name, tc := range tests3 {
		t.Run("IsTaskDoneAfter:"+name, func(t *testing.T) {
			got, err := s.IsTaskDoneAfter(tc.task, midnight)
			if err != nil {
				t.Fatalf("failed to IsTaskDoneAfter:%v", err)
			}
			if got != tc.want {
				t.Errorf("got: %v, want: %v", got, tc.want)
			}
		})
	}

}

func mustGetenv(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Errorf("%s environment variable not set", k)
	}
	return v
}

func TestGetMidnightUnixtime(t *testing.T) {
	cases := []struct {
		name      string
		inputDate time.Time
		want      int64
	}{
		{
			"2020_7_7_0_0_0",
			time.Date(2020, 7, 7, 0, 0, 1, 0, time.Local),
			1594047600,
		},
		{
			"2020_7_7_23_59_59",
			time.Date(2020, 7, 7, 23, 59, 59, 0, time.Local),
			1594047600,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := getMidnightUnixtime(tt.inputDate)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
