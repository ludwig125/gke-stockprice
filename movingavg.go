package main

import (
	"fmt"
	"log"
	"strconv"
)

// TODO: 関数にするかメソッドにするか考える
func calculateEachMovingAvg(db DB, code string) error {
	// 直近200日分の終値を取得する
	d, err := getRecentCloses(db, code, 200)
	if err != nil {
		return fmt.Errorf("failed to getRecentCloses: %v", err)
	}
	dcs := DateCloses(d)

	// 取得対象の移動平均
	movingAvgList := []int{3, 5, 7, 10, 20, 60, 100}
	// (日付;移動平均)のMapを3, 5, 7,...ごとに格納したMap
	moving := make(map[int]map[string]string)
	for _, m := range movingAvgList {
		// moving[3]: 3日移動平均
		// moving[5]: 5日移動平均...
		moving[m] = dcs.CalcMovingAvg(m)
	}

	// 移動平均のDBに入れるためのスライス
	var insData [][]string
	for i := 0; i < len(dcs); i++ {
		d := dcs[i].Date // 日付
		var codeDateMovings = make([]string, 2+len(movingAvgList))
		codeDateMovings[0] = code
		codeDateMovings[1] = d
		for j, m := range movingAvgList { // 移動平均
			codeDateMovings[2+j] = moving[m][d]
		}
		insData = append(insData, codeDateMovings)
	}
	if err := db.InsertDB("movingavg", insData); err != nil {
		return fmt.Errorf("failed to insert movingavg: %v", err)
	}
	return nil
}

// DateClose has Date and Close
type DateClose struct {
	Date  string
	Close float64
}

// 日付の降順で返す
func getRecentCloses(db DB, code string, limit int) ([]DateClose, error) {
	limitStr := ""
	if limit != 0 {
		limitStr = fmt.Sprintf("LIMIT %d", limit)
	}
	q := fmt.Sprintf("SELECT date, close FROM daily WHERE code = '%v' ORDER BY date DESC %s;", code, limitStr)
	log.Println("query:", q)
	res, err := db.SelectDB(q)
	if err != nil {
		return nil, fmt.Errorf("failed to selectTable %v", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no selected data")
	}
	var dcs []DateClose
	for _, r := range res {
		// float64型数値に変換
		// 株価には小数点が入っていることがあるのでfloatで扱う
		f, err := strconv.ParseFloat(r[1], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to ParseFloat. %v", err)
		}
		dc := DateClose{
			Date:  r[0],
			Close: f,
		}
		dcs = append(dcs, dc)
	}
	return dcs, nil
}

// DateCloses has Date and Closes
type DateCloses []DateClose

// CalcMovingAvg calcurates moving average for each target days
func (d *DateCloses) CalcMovingAvg(avgDays int) map[string]string {
	//func movingAvg(dcs []DateClose, avgDays int) map[string]float64 {
	dateMoving := make(map[string]string) // 日付と移動平均のMap

	// 与えられた日付&終値の要素数
	length := len(*d)
	for date := 0; date < length; date++ {
		// 引数で与えられた、何日分の移動平均を取るかというavgDaysをセット
		days := avgDays
		// もし上の数が残りのデータ数より多かったら残りのデータ数がdaysになる
		if date+days > length {
			days = length - date
		}
		// date日目から終値をdays分合計する
		var sum float64
		for i := date; i < date+days; i++ {
			sum += (*d)[i].Close
		}
		movingAvg := float64(sum) / float64(days)
		// 小数点以下の0しかない部分は入れないために%gを使う
		dateMoving[(*d)[date].Date] = fmt.Sprintf("%g", movingAvg)
	}
	return dateMoving
}

// func calculateMovingAvg(ctx context.Context, concurrency int) error {
// 	// 最新の日付にある銘柄を取得
// 	res, err := selectDB(
// 		"SELECT code FROM daily WHERE date = (SELECT date FROM daily ORDER BY date DESC LIMIT 1);")
// 	if err != nil {
// 		return fmt.Errorf("failed to selectTable %v", err)
// 	}
// 	codes := res[0] // selectの結果は２次元配列なので0要素目がcodes

// 	length := len(codes)
// 	for begin := 0; begin < length; begin += concurrency {
// 		end := begin + concurrency
// 		if end >= length {
// 			end = length
// 		}
// 		partialCodes := codes[begin:end]
// 		err := calculateMovingAvgConcurrently(ctx, partialCodes)
// 		if err != nil {
// 			return fmt.Errorf("failed to calculateMovingAvgConcurrently: %v", err)
// 		}
// 		// // concurrency単位で移動平均線の計算を並行で行う
// 		// for _, code := range partialCodes {
// 		// 	select {insertDB
// 		// }
// 		// if err := insertDB("movingavg", hogehoge); err != nil {
// 		// 	return fmt.Errorf("failed to insertDB: %v", err)
// 		// }
// 	}
// 	return nil
// }

// func calculateMovingAvgConcurrently(ctx context.Context, codes []string) error {
// 	codeCh := codeGen(ctx, codes)
// 	numWorkers := len(codes)
// 	workers := make([]<-chan string, numWorkers)
// 	for i := 0; i < numWorkers; i++ {
// 		workers[i] = doTask(ctx, codeCh)
// 	}
// 	for d := range merge(ctx, workers) { // mergeから処理済みtaskの番号を読み出し
// 		log.Printf("done code: %s", d)
// 	}
// 	return nil
// }

// // codeをchannel化するgenerator
// func codeGen(ctx context.Context, codes []string) <-chan string {
// 	codeCh := make(chan string)

// 	go func() {
// 		defer close(codeCh)
// 		for _, code := range codes {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			case codeCh <- code:
// 			}
// 		}
// 	}()
// 	return codeCh
// }

// func doTask(ctx context.Context, codeCh <-chan string) <-chan string {
// 	doneCodeCh := make(chan string)
// 	go func() {
// 		defer close(doneCodeCh)
// 		for code := range codeCh {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			default:
// 				// if err := calculateEachMovingAvg(code);err !=nil {
// 				// 	err
// 				// }
// 				doneCodeCh <- code // 処理済みcode番号をchannelにつめる
// 			}
// 		}
// 	}()
// 	return doneCodeCh
// }

// func merge(ctx context.Context, codeChs []<-chan string) <-chan string {
// 	var wg sync.WaitGroup
// 	mergedCodeCh := make(chan string)

// 	mergeCode := func(codeCh <-chan string) {
// 		defer wg.Done()
// 		for t := range codeCh {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			case mergedCodeCh <- t:
// 			}
// 		}
// 	}

// 	wg.Add(len(codeChs))
// 	for _, codeCh := range codeChs {
// 		go mergeCode(codeCh)
// 	}
// 	// 全てのcodeが処理されるまで待つ
// 	go func() {
// 		wg.Wait()
// 		close(mergedCodeCh)
// 	}()
// 	return mergedCodeCh
// }
