package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/ludwig125/gke-stockprice/database"
)

var (
	// 移動平均の計算対象の期間
	targetPeriod = 200
	// 取得対象の移動平均
	targetMovingAvgs = []int{3, 5, 7, 10, 20, 60, 100}
)

// CodeDateMovingAvgs has code and multiple DateMovingAvgs.
type CodeDateMovingAvgs struct {
	code           string
	dateMovingAvgs []DateMovingAvgs
}

// Slices converts CodeDateMovingAvgs to double string slice.
func (c CodeDateMovingAvgs) Slices() [][]string {
	trim := func(f float64) string {
		// 小数点以下の0しかない部分は入れないために%gを使う
		return fmt.Sprintf("%g", f)
	}

	// 移動平均のDBに入れるためのスライス
	var ss [][]string
	for _, d := range c.dateMovingAvgs {
		s := []string{
			c.code,
			d.date,
			trim(d.movingAvgs.M3),
			trim(d.movingAvgs.M5),
			trim(d.movingAvgs.M7),
			trim(d.movingAvgs.M10),
			trim(d.movingAvgs.M20),
			trim(d.movingAvgs.M60),
			trim(d.movingAvgs.M100)}
		ss = append(ss, s)
	}
	return ss
}

// CalculateMovingAvg is configuration to calculate moving average.
type CalculateMovingAvg struct {
	db              database.DB
	calcConcurrency int
}

func (m CalculateMovingAvg) saveMovingAvgs(ctx context.Context, codes []string) error {
	eg, ctx := errgroup.WithContext(ctx)

	sem := make(chan struct{}, m.calcConcurrency)
	defer close(sem)
	for _, code := range codes {
		code := code

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sem <- struct{}{}

		eg.Go(func() error {
			defer func() { <-sem }()
			dm, err := m.movingAvgs(code)
			if err != nil {
				return fmt.Errorf("failed to calculateEachMovingAvg: %v", err)
			}
			cdm := CodeDateMovingAvgs{code: code, dateMovingAvgs: dm}
			if err := m.db.InsertDB("movingavg", cdm.Slices()); err != nil {
				return fmt.Errorf("failed to insert movingavg: %v", err)
			}
			log.Printf("calculated code: '%s' moving average successfully", code)
			return nil
		})
	}
	return eg.Wait()
}

func (m CalculateMovingAvg) movingAvgs(code string) ([]DateMovingAvgs, error) {
	// 直近200日分の終値を取得する
	rc, err := m.recentCloses(code)
	if err != nil {
		return nil, fmt.Errorf("failed to get recentCloses: %v", err)
	}

	// (日付:移動平均)のMapを3, 5, 7,...ごとに格納したMap
	moving := make(map[int]map[string]float64)
	for _, days := range targetMovingAvgs {
		// moving[3]: 3日移動平均
		// moving[5]: 5日移動平均...
		moving[days] = rc.calcMovingAvg(days)
	}
	var dateMovingAvgs []DateMovingAvgs
	for _, r := range rc {
		d := r.Date // 日付
		dm := DateMovingAvgs{
			date: d,
			movingAvgs: MovingAvgs{
				M3:   moving[3][d],
				M5:   moving[5][d],
				M7:   moving[7][d],
				M10:  moving[10][d],
				M20:  moving[20][d],
				M60:  moving[60][d],
				M100: moving[100][d],
			},
		}
		dateMovingAvgs = append(dateMovingAvgs, dm)
	}
	return dateMovingAvgs, nil
}

// 日付の降順で返す
func (m CalculateMovingAvg) recentCloses(code string) (DateCloses, error) {
	// TODO： ここなんとかする
	return recentCloses(m.db, code, targetPeriod)
}

// DateMovingAvgs has date, movingAvgs(3, 5, 7, 10, 20, 60, 100).
type DateMovingAvgs struct {
	date       string
	movingAvgs MovingAvgs
}

// MovingAvgs has each moving average
type MovingAvgs struct {
	M3   float64
	M5   float64
	M7   float64
	M10  float64
	M20  float64
	M60  float64
	M100 float64
}

// DateClose has Date and Close
type DateClose struct {
	Date  string
	Close float64
}

// DateCloses has Date and Closes
type DateCloses []DateClose

func (d DateCloses) calcMovingAvg(days int) map[string]float64 {
	dateMoving := make(map[string]float64) // 日付と移動平均のMap

	// 与えられた日付&終値の要素数
	length := len(d)
	for date := 0; date < length; date++ {
		// もし上の数が残りのデータ数より多かったら残りのデータ数がdaysになる
		if date+days > length {
			days = length - date
		}
		// date日目から終値をdays分合計する
		var sum float64
		for i := date; i < date+days; i++ {
			sum += d[i].Close
		}
		movingAvg := float64(sum) / float64(days)
		dateMoving[d[date].Date] = movingAvg
	}
	return dateMoving
}

// 日付の降順で返す
func recentCloses(db database.DB, code string, limit int) (DateCloses, error) {
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
	var dcs DateCloses
	for _, r := range res {
		// float64型数値に変換
		// 株価には小数点が入っていることがあるのでfloatで扱う
		f, err := strconv.ParseFloat(r[1], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to ParseFloat. %v", err)
		}
		dcs = append(dcs, DateClose{Date: r[0], Close: f})
	}
	return dcs, nil
}
