package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ludwig125/gke-stockprice/sheet"
)

type daily struct {
	currentTime: time.Time
	status               sheet.Sheet
	dailyStockPrice      DailyStockPrice
	calculateMovingAvg   CalculateMovingAvg
	calculateGrowthTrend CalculateGrowthTrend
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

	if d.currentTime.IsZero() {
		log.Println("currentTime is zero")
		return fmt.Errorf("currentTime is zero: %#v", d.currentTime)
	}

	// // dailyStockPriceが完了済みであればスキップ
	// if isDone("dailyStockPrice",d.dailyStockPrice.currentTime)

	// 日足株価のスクレイピングとDBへの書き込み
	sp := d.dailyStockPrice
	failedCodes, err := sp.saveStockPrice(ctx, codes, d.currentTime)
	if err != nil {
		return fmt.Errorf("failed to fetchStockPrice: %v", mergeErr(err, failedCodes))
	}

	// TODO: 全部スクレイピングできていなかったら再度試みる処理を入れたい
	// TODO: 最初に取得した株価が全部格納されているか確認したい

	targetCodes := filterCodes(codes, failedCodes)
	if len(targetCodes) == 0 {
		return fmt.Errorf("all codes failed in saveStockPrice: %v", mergeErr(err, failedCodes))
	}

	// 移動平均線の作成とDBへの書き込み
	m := d.calculateMovingAvg
	if err := m.saveMovingAvgs(ctx, targetCodes); err != nil {
		return fmt.Errorf("failed to calcMovingAvg: %v", mergeErr(err, failedCodes))
	}

	// 株価の増減トレンドをSpreadSheetに記載
	g := d.calculateGrowthTrend
	if err := g.growthTrend(ctx, targetCodes); err != nil {
		return fmt.Errorf("failed to growthTrend: %v", mergeErr(err, failedCodes))
	}

	return mergeErr(nil, failedCodes)
}

// 全銘柄から失敗した銘柄を除く関数
func filterCodes(codes []string, failedCodes FailedCodes) []string {
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
