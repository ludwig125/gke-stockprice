package date

import (
	"fmt"
	"testing"
	"time"
)

func TestGetMidnight(t *testing.T) {
	cases := []struct {
		name      string
		inputDate time.Time
		timezone  string
		want      time.Time
	}{
		{
			"2020_7_7_0_0_0",
			time.Date(2020, 7, 7, 0, 0, 1, 0, time.UTC),
			"UTC",
			time.Date(2020, 7, 7, 0, 0, 0, 0, time.UTC),
		},
		{
			"2020_7_7_23_59_59",
			time.Date(2020, 7, 7, 23, 59, 59, 0, time.UTC),
			"UTC",
			time.Date(2020, 7, 7, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMidnight(tt.inputDate, tt.timezone)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSaturdayOrSunday(t *testing.T) {
	cases := []struct {
		name      string
		inputDate time.Time
		want      bool
	}{
		{
			"2019_11_1_is_friday",
			time.Date(2019, 11, 1, 0, 0, 0, 0, time.Local),
			false,
		},
		{
			"2019_11_2_is_saturday",
			time.Date(2019, 11, 2, 0, 0, 0, 0, time.Local),
			true,
		},
		{
			"2019_11_3_is_sunday",
			time.Date(2019, 11, 3, 0, 0, 0, 0, time.Local),
			true,
		},
		{
			"2019_11_4_is_monday",
			time.Date(2019, 11, 4, 0, 0, 0, 0, time.Local),
			false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSaturdayOrSunday(tt.inputDate)
			if got != tt.want {
				t.Errorf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsTargetWeekday(t *testing.T) {
	tests := map[string]struct {
		targetTime     time.Time
		targetWeekdays []string
		want           bool
		wantErr        bool
	}{
		"sunday": {
			targetTime:     time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			targetWeekdays: []string{"Sunday"},
			want:           true,
			wantErr:        false,
		},
		"sunday2": {
			targetTime:     time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			targetWeekdays: []string{"Saturday", "Sunday"},
			want:           true,
			wantErr:        false,
		},
		"saturday": {
			targetTime:     time.Date(2020, 8, 22, 19, 36, 41, 833000000, time.UTC), // 土曜日
			targetWeekdays: []string{"Saturday", "Sunday"},
			want:           true,
			wantErr:        false,
		},
		"not_sunday": {
			targetTime:     time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			targetWeekdays: []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
			want:           false,
			wantErr:        false,
		},
		"invalid": {
			targetTime:     time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			targetWeekdays: []string{"Manday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
			want:           false,
			wantErr:        true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := IsTargetWeekday(tc.targetTime, tc.targetWeekdays)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("gotErr %t, wantErr %t", err, tc.wantErr)
				}
				return
			}
			if got != tc.want {
				t.Errorf("got %t, want %t", got, tc.want)
			}
		})
	}
}

func TestParseRFC3339(t *testing.T) {
	tests := map[string]struct {
		timeStr string
		want    time.Time
		wantErr bool
	}{
		"time1": {
			timeStr: "2020-08-23T19:36:41.833Z",
			want:    time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC),
			wantErr: false,
		},
		"time_err": {
			timeStr: "2020-08-23T19:36:41.833",
			want:    time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC),
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseRFC3339(tc.timeStr)
			if err != nil {
				if tc.wantErr {
					return
				}
				t.Fatalf("gotErr: %v, wantErr: %v", err, tc.wantErr)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got: %v, want: %v", got, tc.want)
			}
			fmt.Println(got)
		})
	}
}

func TestGetLatestTargetWeekday(t *testing.T) {
	tests := map[string]struct {
		now            time.Time
		targetWeekdays []string
		want           time.Time
		wantErr        bool // TODO: 不要？
	}{
		"now_monday_target_sunday": {
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.UTC), // 月曜日
			targetWeekdays: []string{"Sunday"},
			want:           time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			wantErr:        false,
		},
		"now_monday_target_saturday": {
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.UTC), // 月曜日
			targetWeekdays: []string{"Saturday"},
			want:           time.Date(2020, 8, 22, 19, 36, 41, 833000000, time.UTC), // 土曜日
			wantErr:        false,
		},
		"now_monday_target_sunday_saturday": {
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.UTC), // 月曜日
			targetWeekdays: []string{"Sunday", "Saturday"},
			want:           time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			wantErr:        false,
		},
		"now_monday_target_saturday_sunday": {
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.UTC), // 月曜日
			targetWeekdays: []string{"Saturday", "Sunday"},
			want:           time.Date(2020, 8, 23, 19, 36, 41, 833000000, time.UTC), // 日曜日
			wantErr:        false,
		},
		"now_monday_target_tuesday_wednesday_thursday": {
			now:            time.Date(2020, 8, 24, 19, 36, 41, 833000000, time.UTC), // 月曜日
			targetWeekdays: []string{"Tuesday", "Wednesday", "Thursday"},
			want:           time.Date(2020, 8, 20, 19, 36, 41, 833000000, time.UTC), // 木曜日
			wantErr:        false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := GetLatestTargetWeekday(tc.now, tc.targetWeekdays)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("gotErr %t, wantErr %t", err, tc.wantErr)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
