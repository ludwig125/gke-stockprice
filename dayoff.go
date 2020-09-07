package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"

	"github.com/ludwig125/gke-stockprice/date"
	"github.com/ludwig125/gke-stockprice/sheet"
)

// DayOff has dayoff(true/false) and it's reason.
type DayOff struct {
	dayOff bool
	reason string
}

func isDayOff(previousDate time.Time, s sheet.Sheet) DayOff {
	// 前の日が休日かどうか
	isHoli, err := isHoliday(s, previousDate)
	if err != nil {
		// sheetからデータが取れないだけであればエラー出して処理自体は続ける
		log.Printf("failed to check isHoliday: %v", err)
	}
	if isHoli {
		return DayOff{dayOff: true, reason: fmt.Sprintf("%v is holiday", previousDate)}
	}
	// 前の日が土日かどうか
	if date.IsSaturdayOrSunday(previousDate) {
		log.Println("previous day is saturday or sunday. finish task")
		return DayOff{dayOff: true, reason: fmt.Sprintf("%v is saturday or sunday", previousDate)}
	}

	return DayOff{}
}

// spreadsheetの'holiday' sheetを読み取って 与えられた日付が祝日かどうか判断する
func isHoliday(s sheet.Sheet, d time.Time) (bool, error) {
	// 'holiday' sheet を読み取り
	// sheetには「2019/01/01」の形式の休日が縦一列になっていることを想定している
	// 東京証券取引所の休日: https://www.jpx.co.jp/corporate/calendar/index.html
	holidays, err := s.Read()
	if err != nil {
		return false, fmt.Errorf("failed to ReadSheet: %v", err)
	}
	if holidays == nil || len(holidays) == 0 {
		return false, errors.New("no data in holidays")
	}

	date := d.Format("2006/01/02") // "2019/10/31" のようなフォーマットにする
	log.Println("requested date:", date)
	for _, h := range holidays {
		if h[0] == date { //対象の日付が一覧にあったら休日判定
			return true, nil
		}
	}
	return false, nil
}
