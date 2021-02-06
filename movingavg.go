package main

import (
	"fmt"
)

// CodeDateMovingAvgs maps code and multiple DateMovingAvgs.
type CodeDateMovingAvgs map[string][]DateMovingAvgs

// Slices converts CodeDateMovingAvgs to double string slice.
func (c CodeDateMovingAvgs) Slices() [][]string {
	// 移動平均のDBに入れるためのスライス
	var movingavgData [][]string
	for code, dateMovingAvgs := range c {
		for _, dateMovingAvg := range dateMovingAvgs {
			movingavgData = append(movingavgData, codeDateMovingavgsToStringSlice(code, dateMovingAvg))
		}
	}
	return movingavgData
}

func codeDateMovingavgsToStringSlice(code string, dateMovingAvgs DateMovingAvgs) []string {
	trim := func(f float64) string {
		// 小数点以下の0しかない部分は入れないために%gを使う
		return fmt.Sprintf("%g", f)
	}
	m := dateMovingAvgs.MovingAvgs
	return []string{
		code,
		dateMovingAvgs.Date,
		trim(m.M3),
		trim(m.M5),
		trim(m.M7),
		trim(m.M10),
		trim(m.M20),
		trim(m.M60),
		trim(m.M100),
	}
}

// DateMovingAvgs has date, movingAvgs(3, 5, 7, 10, 20, 60, 100).
type DateMovingAvgs struct {
	Date       string
	MovingAvgs MovingAvgs
}

// MovingAvgs has each moving average.
type MovingAvgs struct {
	M3   float64
	M5   float64
	M7   float64
	M10  float64
	M20  float64
	M60  float64
	M100 float64
}

// DateClose has Date and Close.
type DateClose struct {
	Date  string
	Close float64
}

// DateCloses has Date and Closes.
type DateCloses []DateClose

func (d DateCloses) closes() []float64 {
	closes := make([]float64, len(d))
	for i, v := range d {
		closes[i] = v.Close
	}
	return closes
}

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
