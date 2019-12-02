package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/sheet"
)

func fetchCompanyCode(s sheet.Sheet) ([]string, error) {
	var codes []string
	resp, err := s.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to ReadSheet: %v", err)
	}
	for _, v := range resp {
		c := v[0]
		if c == "" { // 空の場合は登録しない
			continue
		}
		if _, err := strconv.Atoi(c); err != nil {
			return nil, fmt.Errorf("failed to convert int: %v", err)
		}
		codes = append(codes, c)
	}
	//codes := []string{"100", "101", "102", "103", "104", "105", "106", "107"}
	return codes, nil
}

func fetchStockPrice(
	ctx context.Context, db database.DB,
	codes []string, dailyStockpriceURL string,
	maxInsertNum int, scrapeInterval time.Duration) ([]string, error) {
	// scrapeDailyStockPricesで発生したerrorは全部warning扱いにする
	// warnsにまとめて格納して返す
	var warns []string

	length := len(codes)
	// codeを順にscrapingして、maxInsertNumの数ごとにDBにInsertする
	for begin := 0; begin < length; begin += maxInsertNum {
		end := begin + maxInsertNum
		if end >= length {
			end = length
		}
		partialCodes := codes[begin:end]
		// maxInsertNum単位でscraping
		//start := time.Now()
		for _, code := range partialCodes {
			select {
			case <-ctx.Done(): // ctx のcancelを受け取ったら終了
				return nil, ctx.Err()
			default:
			}

			//scrapeStart := time.Now()
			// 指定された銘柄codeをScrape
			// scrapeに失敗してもwarnとして最後にエラーを出す
			// pは[日付, 始値, 高値, 安値, 終値, 売買高, 修正後終値]の配列が１ヶ月分入った二重配列
			prices, warn := scrapeDailyStockPrices(ctx, code, dailyStockpriceURL)
			if warn != nil {
				log.Printf("failed to scrape code. %v", warn)
				warns = append(warns, fmt.Sprintf("%v", warn))
				continue
			}
			//fmt.Printf("each scrape since scrapeStart %v\n", time.Since(scrapeStart))

			// 以下で、取得したpricesの前にcodeを追加して、DBに格納する
			var codePrices [][]string
			// ["日付", "始値"...],["日付", "始値"...],...を１行ずつ展開
			for _, price := range prices {
				// ["日付", "始値", "高値", "安値", "終値", "売買高", "修正後終値]の配列の先頭に銘柄codeを追加する
				codePrice := make([]string, 8) // 事前に8要素分確保
				codePrice[0] = code            // 先頭に銘柄を格納
				for i := 0; i < 7; i++ {
					codePrice[i+1] = price[i]
				}
				// ["銘柄", "日付",..."修正後終値]の配列をcodePricesに追加
				codePrices = append(codePrices, codePrice) // [][]stringの後ろに[]stringを追加
			}
			if err := db.InsertDB("daily", codePrices); err != nil {
				return nil, fmt.Errorf("failed to InsertDB: %v", err)
			}

			//fmt.Printf("insertDB since start %v\n", time.Since(start))
			time.Sleep(scrapeInterval) // scrape先への負荷を考えて毎回1秒待つ
		}
		//fmt.Printf("each maxInsertNum scrape since start %v\n", time.Since(start))

	}
	return warns, nil
}

func calculateMovingAvg(ctx context.Context, db database.DB, codes []string, concurrency int) error {
	eg, ctx := errgroup.WithContext(ctx)

	sem := make(chan struct{}, concurrency)
	defer close(sem)
	for _, code := range codes {
		sem <- struct{}{} // チャネルに送信

		c := code
		eg.Go(func() error {
			defer func() { <-sem }()
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err := calculateEachMovingAvg(db, c); err != nil {
				return fmt.Errorf("failed to calculateEachMovingAvg: %v", err)
			}
			log.Printf("calculated code: '%s' moving average successfully", c)
			return nil
		})
	}

	return eg.Wait()
}

func calculateGrowthTrend(ctx context.Context, db database.DB, codes []string, concurrency int) error {
	eg, ctx := errgroup.WithContext(ctx)

	sem := make(chan struct{}, concurrency)
	defer close(sem)
	for _, code := range codes {
		sem <- struct{}{} // チャネルに送信

		c := code
		eg.Go(func() error {
			defer func() { <-sem }()
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err := calculateEachGrowthTrend(db, c); err != nil {
				return fmt.Errorf("failed to calculateEachGrowthTrend: %v", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

// // codeを処理して処理済みのCode番号をchannelとして返す関数
// func doCode(ctx context.Context, codeCh <-chan string) <-chan int {
// 	doneCodeCh := make(chan int)
// 	go func() {
// 		defer close(doneCodeCh)
// 		for code := range codeCh {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			default:
// 				log.Printf("do code number: %s", code)
// 				// log.Printf("do code number: %d\n", code.Number)
// 				// // codeのための処理をする
// 				// // ここではcode にかかるCostだけSleepする
// 				// time.Sleep(code.Cost)
// 				// doneCodeCh <- code.Number // 処理済みcode番号をchannelにつめる
// 			}
// 		}
// 	}()
// 	return doneCodeCh
// }
