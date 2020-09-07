package date

import (
	"fmt"
	"log"
	"time"
)

// // TimeIn returns the time in UTC if the name is "" or "UTC".
// // It returns the local time if the name is "Local".
// // Otherwise, the name is taken to be a location name in
// func TimeIn(t time.Time, name string) (time.Time, error) {
// 	loc, err := time.LoadLocation(name)
// 	if err == nil {
// 		t = t.In(loc)
// 	}
// 	return t, err
// }

// GetMidnight returns t's 0 hours 0 minutes 0 seconds just.
func GetMidnight(t time.Time, name string) (time.Time, error) {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Time{}, err
	}
	// 与えられた時刻の0時0分0秒のUnixTimeを取得する
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, loc), nil
}

// IsSaturdayOrSunday check d is Saturday or Sunday. TODO: IsTargetWeekdayに置き換える
func IsSaturdayOrSunday(d time.Time) bool {
	// 与えられた日付が土日かどうか判断する
	day := d.Weekday()
	log.Printf("requested date: %s, day: %s", d.Format("2006/01/02"), day.String())
	switch day {
	case 6, 0: // Saturday, Sunday
		return true
	}
	return false
}

// IsTargetWeekday check t is target day.
func IsTargetWeekday(t time.Time, targetWeekdays []string) (bool, error) {
	// 与えられた日付が指定の曜日かどうか判断する
	day := t.Weekday().String()
	// log.Printf("requested date: %s %s, targetWeekdays: %v", t.Format("2006/01/02"), day, targetWeekdays)
	for _, d := range targetWeekdays {
		if d != "Sunday" && d != "Monday" && d != "Tuesday" && d != "Wednesday" && d != "Thursday" && d != "Friday" && d != "Saturday" {
			return false, fmt.Errorf("targetWeekday is invalid: %v", targetWeekdays)
		}
		if day == d {
			// log.Printf("requested date: %v matched targetWeekday: %v", day, targetWeekdays)
			return true, nil
		}
	}
	// log.Printf("requested date: %v does not matched targetWeekday: %v", day, targetWeekdays)
	return false, nil
}

// ParseRFC3339 parse RFC3339 format string to time.Time.
func ParseRFC3339(tstr string) (time.Time, error) {
	// 文字列形式の時刻をRFC3339形式のtime.Timeに変換する
	t, err := time.Parse(
		time.RFC3339, tstr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to time RFC3339 parse: %v. target: %v", err, tstr)
	}
	return t, nil
}

// GetLatestTargetWeekday returns latest Weekday's date before arg t.
func GetLatestTargetWeekday(t time.Time, targetWeekdays []string) (time.Time, error) {
	// 一番最近のX曜日の日付(時刻以下は引数で与えられたtと同じ)を取得する
	// 曜日が複数与えられた場合は一番最近の曜日を採用する
	for {
		ok, err := IsTargetWeekday(t, targetWeekdays)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to IsTargetWeekday: %v", err)
		}
		if ok {
			return t, nil
		}
		t = t.AddDate(0, 0, -1)
	}
}
