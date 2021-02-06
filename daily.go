package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ludwig125/gke-stockprice/retry"
	"github.com/ludwig125/gke-stockprice/sheet"
	"github.com/ludwig125/gke-stockprice/status"
)

type daily struct {
	status                       sheet.Sheet
	dayoff                       DayOff
	dailyStockPrice              DailyStockPrice
	calculateDailyMovingAvgTrend CalculateDailyMovingAvgTrend
	// calculateMovingAvg   CalculateMovingAvg
	// calculateGrowthTrend CalculateGrowthTrend
}

func (d daily) exec(ctx context.Context, codes []string) error {
	mergeErr := func(err error, failedCodes FailedCodes) error {
		if err != nil {
			if len(failedCodes) != 0 {
				return fmt.Errorf("%v\n%v", err, failedCodes.Error())
			}
			return fmt.Errorf("%v", err)
		}
		if len(failedCodes) != 0 {
			return fmt.Errorf("%v", failedCodes.Error())
		}
		return nil
	}

	if now().IsZero() {
		log.Println("now is zero")
		return fmt.Errorf("now is zero: %#v", now())
	}

	// Status管理用の変数
	st := status.Status{Sheet: d.status}

	// 日足株価のスクレイピングとDBへの書き込み
	// statusシートを見て本日分が未完了であれば実行する(ExecIfIncompleteThisDay関数の機能)
	sp := d.dailyStockPrice
	var failedCodes FailedCodes
	if err := st.ExecIfIncompleteThisDay("saveStockPrice", now(), func() error {
		var e error
		failedCodes, e = sp.saveStockPrice(ctx, codes, now())
		return e
	}); err != nil {
		return fmt.Errorf("failed to saveStockPrice: %v", mergeErr(err, failedCodes))
	}

	// 全部スクレイピングできていなかったら再度試みる
	// - failedCodesを使って、数回saveStockPriceを実行する
	// - それでも失敗したものをあらためてfailedCodesとする
	if len(failedCodes) > 0 {
		retryCnt := 0
		if err := retry.WithContext(ctx, 2, 5*time.Second, func() error {
			fcodes := failedCodesSlice(failedCodes) // failedCodesから銘柄のスライスを取得
			if len(fcodes) == 0 {                   // failedCodesがなければ終了
				return nil
			}
			retryCnt++
			log.Printf("retry: %d. trying to fetch stockprice for failed codes: %v", retryCnt, fcodes)
			start := now()
			newFailedCodes, err := sp.saveStockPrice(ctx, fcodes, now())
			st.InsertStatus(fmt.Sprintf("saveStockPrice_retry%d", retryCnt), now(), now().Sub(start)) // now().Sub(start)で所要時間も入れておく
			log.Printf("retry: %d. failedCodes error: '%#v', saveStockPrice error: %v", retryCnt, newFailedCodes.Error(), err)

			failedCodes = newFailedCodes // failedCodesを上書き
			return fmt.Errorf("failedCodes error: %v, saveStockPrice error: %v", newFailedCodes.Error(), err)
		}); err != nil {
			// retry 時のエラーはログに出すだけにしておく
			log.Printf("failed to saveStockPrice in retry: %v", err)
		}
	}

	// TODO: 最初に取得した株価が全部格納されているか確認したい

	// 前の日が祝日だったら以降の処理はせずに終了する
	if d.dayoff.dayOff {
		log.Printf("previous day is dayoff: %s. finish task", d.dayoff.reason)
		return mergeErr(nil, failedCodes)
	}

	// この後の処理のために、失敗した銘柄以外を抜き出す。全部失敗して一つも残らなかったらエラーで終了
	targetCodes := filterSuccessCodes(codes, failedCodes)
	if len(targetCodes) == 0 {
		return fmt.Errorf("all codes failed in saveStockPrice: %v", mergeErr(nil, failedCodes))
	}

	// 移動平均線とTrendの作成とDBへの書き込み
	// statusシートを見て本日分が未完了であれば実行する
	m := d.calculateDailyMovingAvgTrend
	if err := st.ExecIfIncompleteThisDay("calculateDailyMovingAvgTrend", now(), func() error {
		// TODO: fromは、最後に書き込みが行われた時間を確認したうえで設定してもよさそう
		return m.Exec(targetCodes)
	}); err != nil {
		return fmt.Errorf("failed to calculateDailyMovingAvgTrend: %v", mergeErr(err, failedCodes))
	}

	return mergeErr(nil, failedCodes)
}

// 失敗した銘柄のスライスを返す関数
func failedCodesSlice(failedCodes FailedCodes) []string {
	fcodes := make([]string, len(failedCodes))
	for i, f := range failedCodes {
		fcodes[i] = f.code
	}
	return fcodes
}

// 全銘柄から失敗した銘柄を除く関数
func filterSuccessCodes(codes []string, failedCodes FailedCodes) []string {
	match := func(code string, failedCodes FailedCodes) bool {
		for _, f := range failedCodes {
			if code == f.code {
				return true
			}
		}
		return false
	}

	var filteredCodes []string
	for _, c := range codes {
		if !match(c, failedCodes) {
			filteredCodes = append(filteredCodes, c)
		}
	}
	return filteredCodes
}
