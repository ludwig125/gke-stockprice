package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	now = getNow() // 現在時刻を全体で使う
	env = useEnvOrDefault("ENV", "dev")
)

func getNow() time.Time {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatalf("failed to get LoadLocation: %v", err)
	}
	return time.Now().In(jst)
}

func main() {
	log.Println("start")
	// if os.Getenv("ENABLE_GKE_CLUSTER_DELETE") == "on" {
	// 	ciToken := mustGetenv("CIRCLE_API_USER_TOKEN")
	// 	defer func() {
	// 		err := requestCircleci(ciToken, "delete_gke_cluster")
	// 		if err != nil {
	// 			log.Printf("failed to requestCircleci: %v", err)
	// 		}
	// 		log.Println("requestCircleci successfully", ciToken)
	// 	}()
	// }

	//	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	//ref. https://golang.org/pkg/os/signal/#example_Notify_allSignals
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer func() {
		signal.Stop(sigCh) // シグナルの受付を終了する
		//		cancel() // あとで上のctxと一緒に有効にする
	}()

	go func() {
		select {
		case sig := <-sigCh: // シグナルを受け取ったらここに入る
			fmt.Println("Got signal!", sig)
			//	cancel() // cancelを呼び出して全ての処理を終了させる
		}
	}()

	for i := 0; i <= 10000; i++ {
		log.Printf("i=%d", i)
		time.Sleep(1 * time.Second)
	}
	log.Println("finish")

	// // 日時バッチ処理
	// if err := daily(ctx); err != nil {
	// 	log.Println(err)
	// 	cancel() // 何らかのエラーが発生した場合、他の処理も全てcancelさせる
	// 	return
	// }
}

func daily(ctx context.Context) error {
	// 環境変数の読み込み
	var db DB
	var dberr error
	if env == "prod" {
		// prod環境ならPASSWORD必須
		log.Println("this is prod. trying to fetch CLOUDSQL_PASSWORD")
		db, dberr = NewDB(fmt.Sprintf("%s:%s@cloudsql(%s)/%s",
			mustGetenv("CLOUDSQL_USER"),
			mustGetenv("CLOUDSQL_PASSWORD"),
			mustGetenv("CLOUDSQL_CONNECTION_NAME"),
			"stockprice"))
	} else {
		log.Println("this is dev")
		db, dberr = NewDB("root@/stockprice_dev")
	}
	if dberr != nil {
		return fmt.Errorf("failed to NewDB: %v", dberr)
	}
	defer db.CloseDB()
	// DBに接続されているか確認
	// この部分NewDB内に持っていくかあとで考える
	if err := ensureDB(db); err != nil {
		return fmt.Errorf("failed to ensureDB: %v", err)
	}
	log.Println("connected db successfully")

	// spreadsheetのserviceを取得
	sheetCredential := mustGetenv("SHEET_CREDENTIAL")
	srv, err := getSheetClient(ctx, sheetCredential)
	if err != nil {
		return fmt.Errorf("failed to get sheet service. err: %v", err)
	}
	log.Println("got sheet service successfully")

	dailyStockpriceURL := mustGetenv("DAILY_PRICE_URL")                                                      // 日足株価scrape先のURL
	maxInsertDBNum := strToInt(useEnvOrDefault("MAX_INSERT_DB_NUM", "3"))                                    // DBへInsertする際の最大件数
	scrapeInterval := time.Duration(strToInt(useEnvOrDefault("SCRAPE_INTERVAL", "1000"))) * time.Millisecond // スクレイピングの間隔(millisec)
	calcMovingavgConcurrency := strToInt(useEnvOrDefault("CALC_MOVINGAVG_CONCURRENCY", "3"))                 // DBへInsertする際の最大件数

	holidaySheet := NewSpreadSheet(srv, mustGetenv("HOLIDAY_SHEETID"), "holiday")
	codeSheet := NewSpreadSheet(srv, mustGetenv("COMPANYCODE_SHEETID"), "tse-first")
	// ---- ここまで環境変数の取得などの前作業

	holi, err := isHoliday(holidaySheet, now.AddDate(0, 0, -1))
	if err != nil {
		// sheetからデータが取れないだけであればエラー出して処理自体は続ける
		log.Printf("failed to isHoliday: %v", err)
	}
	// 前の日が祝日だったら起動しないで終わる
	if holi {
		log.Println("previous day is holiday. finish task")
		return nil
	}
	// 前の日が土日だったら起動しないで終わる
	if isSaturdayOrSunday(now.AddDate(0, 0, -1)) {
		log.Println("previous day is saturday or sunday. finish task")
		return nil
	}

	// 銘柄一覧の取得
	codes, err := fetchCompanyCode(codeSheet)
	if err != nil {
		return fmt.Errorf("failed to fetchCompanyCode: %v", err)
	}
	if codes == nil || len(codes) == 0 { // codesが空だったらエラーで終了
		return errors.New("no target company codes")
	}

	// 日足株価のスクレイピングとDBへの書き込み
	warns, err := fetchStockPrice(ctx, db, codes, dailyStockpriceURL, maxInsertDBNum, scrapeInterval)
	if err != nil {
		return fmt.Errorf("failed to fetchStockPrice: %v", err)
	}

	// TODO: 全部スクレイピングできていなかったら再度試みる処理を入れたい
	// TODO: 最初に取得した株価が全部格納されているか確認したい

	// 移動平均線の作成とDBへの書き込み
	// 最新の日付にある銘柄を取得
	codesFromDB, err := db.SelectDB(
		"SELECT code FROM daily WHERE date = (SELECT date FROM daily ORDER BY date DESC LIMIT 1);")
	if err != nil {
		return fmt.Errorf("failed to selectTable %v", err)
	}
	//codes := res[0] // selectの結果は２次元配列なので0要素目がcodes
	if err := calculateMovingAvg(ctx, db, codesFromDB[0], calcMovingavgConcurrency); err != nil {
		return err
	}

	if err := calculateGrowthTrend(ctx, db, codesFromDB[0], calcMovingavgConcurrency); err != nil {
		return err
	}

	// 一週間に一度（土曜日？）はbackup

	// 全部終わったあと、warnsをerrorとして返す
	if len(warns) > 0 {
		return fmt.Errorf("%s", strings.Join(warns, ","))
	}

	return nil
}

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s environment variable not set", k)
	}
	log.Printf("%s environment variable set", k)
	return v
}

func useEnvOrDefault(key, def string) string {
	v := def
	if fromEnv := os.Getenv(key); fromEnv != "" {
		v = fromEnv
	}
	return v
}

func strToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("failed to convert %s to int", s)
	}
	return i
}

func ensureDB(db DB) error {
	// ensure using database
	res, err := db.SelectDB("select database()")
	if err != nil {
		return fmt.Errorf("failed to select database(): %v", err)
	}
	// database が指定されていなかったらNULLが返る
	if res[0][0] == "" {
		return fmt.Errorf("database needs to be used. 'select database()': '%v'", res)
	}
	log.Printf("use database %s", res[0][0])
	return nil
}
