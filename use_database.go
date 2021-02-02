package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ludwig125/gke-stockprice/database"
)

func joinCodeForWhereInStatement(codes []string) string {
	cs := make([]string, len(codes))
	for i, code := range codes {
		cs[i] = fmt.Sprintf("'%s'", code)
	}
	return strings.Join(cs, ",")
}

// database からデータをfetchしてくるための関数置き場

// TODO: movingavg.goのrecentClosesと機能が重複している
// TODO: restructure_tables.goのfetchCodesDateClosesと機能が重複している

// TODO: メソッド化したほうがよさそう（コンストラクタの時点でLIMITとかの書式がおかしかったら弾ける）
func fetchCodesDateCloses(db database.DB, dailyTable string, targetCodes []string, fromDate, toDate, limit string) (map[string][]DateClose, error) {
	// TODO: FromやToやLimitのバリデーションチェックをしたほうがいい

	codes := joinCodeForWhereInStatement(targetCodes)

	q := fmt.Sprintf("SELECT code, date, close FROM %s WHERE code in (%s) %s %s ORDER BY code, date DESC %s;", dailyTable, codes, fromDate, toDate, limit)
	res, err := db.SelectDB(q)
	if err != nil {
		return nil, fmt.Errorf("failed to selectTable %v", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no selected data. query: '%s'", q)
	}

	codeDateCloses := make(map[string][]DateClose, len(targetCodes))
	var dcs []DateClose

	// 以下、複数のcodeとdate が混じったデータを処理するので、
	// currentCodeに現在扱っているcodeを格納して（以下の例だと最初は1001）、
	// あるループでcodeがcurrentCodeと異なったら（以下の例だと1002が出現したら）、
	// currentCodeを1002に入れ替えるという方法で区別して扱う
	// 例
	// 1001, 2020/1/3...
	// 1001, 2020/1/2...
	// 1001, 2020/1/1...
	// 1002, 2020/1/3...
	// 1002, 2020/1/2...
	// 1002, 2020/1/1...
	currentCode := ""
	prevClose := float64(1) // default 0だとgrowthRateの計算で0除算になってしまうので1とした
	for i, r := range res {
		code := r[0]
		if i == 0 {
			currentCode = code
		} else if currentCode != code {
			codeDateCloses[currentCode] = dcs
			dcs = nil
			currentCode = code
			prevClose = float64(1) // codeが変わったので1に戻す
		}
		date := r[1]

		close := r[2]
		var floatClose float64
		if close == "--" { // スクレイピングした時に`--`で格納されていることがあったので、この場合は一つ前の値にする
			floatClose = prevClose
			log.Printf("Warning. close is '--'. Use previous close: %v alternatively. code: %s, date: %s", prevClose, code, date)
		} else {
			// float64型数値に変換
			// closeには小数点が入っていることがあるのでfloatで扱う
			var err error
			floatClose, err = strconv.ParseFloat(close, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to ParseFloat. %v. code: %s, date: %s", err, code, date)
			}
		}
		prevClose = floatClose

		dcs = append(dcs, DateClose{Date: date, Close: floatClose})
	}
	codeDateCloses[currentCode] = dcs // 最後のcode分を格納

	if len(codeDateCloses) != len(targetCodes) {
		return nil, fmt.Errorf("unmatch codes. result codes: %d, targetCodes: %d", len(codeDateCloses), len(targetCodes))
	}

	return codeDateCloses, nil
}

// // Trend計算用に、元のMovingavgの項目から必要なもの、m5, m20, m60, m100を絞って返す関数
// // 銘柄コード、日付を渡すと該当のmovings structに対応するX日移動平均を返す
// func fetchTrendMovingAvgs(db database.DB, movingavgTable string, targetCodes []string, fromDate, toDate, limit string) (map[string][]DateTrendMovingAvg, error) {
// 	codes := joinCodeForWhereInStatement(targetCodes)

// 	q := fmt.Sprintf("SELECT code, date, moving5, moving20, moving60, moving100 FROM %s WHERE code in (%s) %s %s ORDER BY code, date DESC %s;", movingavgTable, codes, fromDate, toDate, limit)

// 	res, err := db.SelectDB(q)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to selectDB: %v", err)
// 	}
// 	if len(res) == 0 {
// 		log.Printf("no selected data. query: `%s`", q)
// 		return nil, nil
// 	}

// 	codeTrendMovingAvgs := make(map[string][]DateTrendMovingAvg, len(targetCodes))
// 	var dateTrendMovings []DateTrendMovingAvg
// 	// 以下、複数のcodeとtrend が混じったデータを処理するので、
// 	// currentCodeに現在扱っているcodeを格納して（以下の例だと最初は1001）、
// 	// あるループでcodeがcurrentCodeと異なったら（以下の例だと1002が出現したら）、
// 	// currentCodeを1002に入れ替えるという方法で区別して扱う
// 	// 例
// 	// 1001, 2020/1/3...
// 	// 1001, 2020/1/2...
// 	// 1001, 2020/1/1...
// 	// 1002, 2020/1/3...
// 	// 1002, 2020/1/2...
// 	// 1002, 2020/1/1...
// 	currentCode := ""
// 	for i, r := range res {
// 		code := r[0]
// 		if i == 0 {
// 			currentCode = code
// 		} else if currentCode != code {
// 			codeTrendMovingAvgs[currentCode] = dateTrendMovings
// 			dateTrendMovings = nil
// 			currentCode = code
// 		}

// 		date := r[1]
// 		m5, err := strconv.ParseFloat(r[2], 64)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to ParseFloat moving5: %v", err)
// 		}
// 		m20, err := strconv.ParseFloat(r[3], 64)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to ParseFloat moving20: %v", err)
// 		}
// 		m60, err := strconv.ParseFloat(r[4], 64)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to ParseFloat moving60: %v", err)
// 		}
// 		m100, err := strconv.ParseFloat(r[5], 64)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to ParseFloat moving100: %v", err)
// 		}

// 		tm := TrendMovingAvgs{M5: m5, M20: m20, M60: m60, M100: m100}
// 		dateTrendMovings = append(dateTrendMovings, DateTrendMovingAvg{Date: date, TrendMovingAvgs: tm})
// 	}
// 	codeTrendMovingAvgs[currentCode] = dateTrendMovings // 最後のcode分を格納

// 	if len(codeTrendMovingAvgs) != len(targetCodes) {
// 		return nil, fmt.Errorf("unmatch codes. result codes: %d, targetCodes: %d", len(codeTrendMovingAvgs), len(targetCodes))
// 	}
// 	return codeTrendMovingAvgs, nil

// }

// TrendListを取得する
func fetchTrendList(db database.DB, trendTable string, targetCodes []string, date string) (map[string]TrendList, error) {
	codes := joinCodeForWhereInStatement(targetCodes)

	q := fmt.Sprintf("SELECT code, trend, trendTurn, growthRate, crossMoving5, continuationDays FROM trend WHERE code in (%s) AND date = '%s';", codes, date)
	res, err := db.SelectDB(q)
	if err != nil {
		return nil, fmt.Errorf("failed to selectTable %v, query: %s", err, q)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("failed to fetch data: %v, query: %s", err, q)
	}

	codeTrends := make(map[string]TrendList, len(targetCodes))
	for _, r := range res {
		code := r[0]

		trend, err := strconv.Atoi(r[1])
		if err != nil {
			return nil, fmt.Errorf("failed to convert string trend to int: %v", err)
		}
		trendTurn, err := strconv.Atoi(r[2])
		if err != nil {
			return nil, fmt.Errorf("failed to convert string trendTurn to int: %v", err)
		}
		growthRate, err := strconv.ParseFloat(r[3], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert string growthRate to float64: %v", err)
		}
		crossMoving5, err := strconv.Atoi(r[4])
		if err != nil {
			return nil, fmt.Errorf("failed to convert string crossMoving5 to int: %v", err)
		}
		continuationDays, err := strconv.Atoi(r[5])
		if err != nil {
			return nil, fmt.Errorf("failed to convert string continuationDays to int: %v", err)
		}

		codeTrends[code] = TrendList{
			trend:            Trend(trend),
			trendTurn:        TrendTurnType(trendTurn),
			growthRate:       float64(growthRate),
			crossMoving5:     CrossMoving5Type(crossMoving5),
			continuationDays: continuationDays,
		}
	}

	if len(codeTrends) != len(targetCodes) {
		return nil, fmt.Errorf("unmatch codes. result codes: %d, targetCodes: %d", len(codeTrends), len(targetCodes))
	}
	return codeTrends, nil
}

// // 直近数日分の営業日のTrendを取得する
// func fetchPastTrends(db database.DB, trendTable string, targetCodes []string, fromDate, toDate, limit string) (map[string][]DateTrend, error) {
// 	codes := joinCodeForWhereInStatement(targetCodes)

// 	q := fmt.Sprintf("SELECT code, trend FROM trend WHERE code in (%s) %s %s ORDER BY date DESC %s;", codes, fromDate, toDate, limit)
// 	res, err := db.SelectDB(q)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to selectTable %v", err)
// 	}
// 	if len(res) == 0 {
// 		log.Printf("%s: got no pastTrend", codes)
// 		return []Trend{}, nil // 過去のTrendがない場合はエラーにはしない
// 	}

// 	// if len(res) < g.longTermThresholdDays { // TODO ここ不要では
// 	// 	log.Printf("%s: got not enouth pastTrend. threshold: %d", code, g.longTermThresholdDays)
// 	// 	return []Trend{}, nil // 過去のTrendがない場合はエラーにはしない
// 	// }

// 	codePastTrends := make(map[string][]DateTrend, len(targetCodes))
// 	var dateTrends []DateTrend
// 	// 以下、複数のcodeとtrend が混じったデータを処理するので、
// 	// currentCodeに現在扱っているcodeを格納して（以下の例だと最初は1001）、
// 	// あるループでcodeがcurrentCodeと異なったら（以下の例だと1002が出現したら）、
// 	// currentCodeを1002に入れ替えるという方法で区別して扱う
// 	// 例
// 	// 1001, 2020/1/3...
// 	// 1001, 2020/1/2...
// 	// 1001, 2020/1/1...
// 	// 1002, 2020/1/3...
// 	// 1002, 2020/1/2...
// 	// 1002, 2020/1/1...
// 	currentCode := ""
// 	for i, r := range res {
// 		code := r[0]
// 		if i == 0 {
// 			currentCode = code
// 		} else if currentCode != code {
// 			codePastTrends[currentCode] = dateTrends
// 			dateTrends = nil
// 			currentCode = code
// 		}

// 		date := r[1]
// 		trend := r[2]
// 		t, err := strconv.Atoi(trend)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to convert string trend to int: %v", err)
// 		}

// 		dateTrends = append(dateTrends, DateTrend{Date: date, Trend: Trend(t)})
// 	}
// 	codePastTrends[currentCode] = dateTrends // 最後のcode分を格納

// 	if len(codePastTrends) != len(targetCodes) {
// 		return nil, fmt.Errorf("unmatch codes. result codes: %d, targetCodes: %d", len(codePastTrends), len(targetCodes))
// 	}
// 	return codePastTrends, nil
// }
