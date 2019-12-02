package main

import (
	"log"
	"time"

	"github.com/pkg/errors"

	"github.com/ludwig125/gke-stockprice/sheet"
)

// spreadsheetの'holiday' sheetを読み取って 与えられた日付が祝日かどうか判断する
func isHoliday(s sheet.Sheet, d time.Time) (bool, error) {
	// 'holiday' sheet を読み取り
	// sheetには「2019/01/01」の形式の休日が縦一列になっていることを想定している
	// 東京証券取引所の休日: https://www.jpx.co.jp/corporate/calendar/index.html
	holidays, err := s.Read()
	if err != nil {
		return true, errors.Wrap(err, "failed to ReadSheet")
	}
	if holidays == nil || len(holidays) == 0 {
		return true, errors.New("no data in holidays")
	}

	date := d.Format("2006/01/02") // "2019/10/31" のようなフォーマットにする
	log.Println("requested date:", date)
	for _, h := range holidays {
		if h[0] == date {
			return true, nil
		}
	}
	return false, nil
}

// 与えられた日付が土日かどうか判断する
func isSaturdayOrSunday(d time.Time) bool {
	day := d.Weekday()
	log.Printf("requested date: %s, day: %s", d.Format("2006/01/02"), day.String())
	switch day {
	case 6, 0: // Saturday, Sunday
		return true
	}
	return false
}
