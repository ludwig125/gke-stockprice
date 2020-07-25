package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	sheets "google.golang.org/api/sheets/v4"

	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/retry"
	"github.com/ludwig125/gke-stockprice/sheet"
)

var (
	jst = getLocation() // タイムゾーンを全体で使う
	env = useEnvOrDefault("ENV", "dev")
	now = func() time.Time { return time.Now().In(jst) }
)

func getLocation() *time.Location {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Panicf("failed to get LoadLocation: %v", err)
	}
	return jst
}

func main() {
	start := time.Now()
	log.Println("start:", start)
	if os.Getenv("DELETE_GKE_CLUSTER") == "on" {
		ciToken := mustGetenv("CIRCLE_API_USER_TOKEN")
		defer func() {
			if err := requestCircleci(ciToken, "delete_gke_cluster"); err != nil {
				log.Printf("failed to requestCircleci: %v", err)
			}
			log.Println("requestCircleci successfully", ciToken)
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	//ref. https://golang.org/pkg/os/signal/#example_Notify_allSignals
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer func() {
		signal.Stop(sigCh) // シグナルの受付を終了する
		cancel()           // あとで上のctxと一緒に有効にする
	}()

	go func() {
		select {
		case sig := <-sigCh: // シグナルを受け取ったらここに入る
			fmt.Println("Got signal!", sig)
			cancel() // cancelを呼び出して全ての処理を終了させる
			return
		}
	}()

	result := "finished successfully"
	emoji := ":sunny:"
	// 日時バッチ処理
	if err := execDailyProcess(ctx); err != nil {
		log.Println("failed to execDailyProcess:", err)
		result = err.Error()
		emoji = ":umbrella:"
		cancel() // 何らかのエラーが発生した場合、他の処理も全てcancelさせる
	}

	finish := time.Now()
	if os.Getenv("SEND_SLACK_MESSAGE") == "on" {
		msg := createSlackMsg("gke-stockprice", start, finish, result)
		sl := NewSlackClient(mustGetenv("SLACK_TOKEN"), mustGetenv("SLACK_CHANNEL"))
		if err := sl.SendMessage("gke-stockprice", msg, emoji); err != nil {
			log.Printf("failed to SendMessage: %v", err)
		}
	}
	// // TODO: あとで以下消す
	// for i := 0; i < 2000; i++ {
	// 	select {
	// 	case <-ctx.Done():
	// 		return
	// 	default:
	// 	}
	// 	if i%10 == 0 {
	// 		log.Println("sleep 1 sec:", i)
	// 	}
	// 	time.Sleep(1 * time.Second)
	// }

	log.Println("process finished successfully")
}

func execDailyProcess(ctx context.Context) error {
	// databaseの取得
	db, err := getDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to getDatabase: %v", err)
	}
	defer db.CloseDB()
	log.Println("connected db successfully")

	// spreadsheetのserviceを取得
	srv, err := getSheetService(ctx, mustGetenv("SHEET_CREDENTIAL"))
	if err != nil {
		return fmt.Errorf("failed to getSheetService: %v", err)
	}
	log.Println("got sheet service successfully")

	var dayoff DayOff
	if env == "prod" && os.Getenv("CHECK_DAYOFF") == "on" {
		previousDate := now().AddDate(0, 0, -1)
		dayoff = isDayOff(previousDate, sheet.NewSpreadSheet(srv, mustGetenv("HOLIDAY_SHEETID"), "holiday"))
	}

	// 銘柄一覧の取得
	codeSheet := sheet.NewSpreadSheet(srv, mustGetenv("COMPANYCODE_SHEETID"), "tse-first")
	codes, err := fetchCompanyCode(codeSheet)
	if err != nil {
		return fmt.Errorf("failed to fetchCompanyCode: %v", err)
	}
	if codes == nil || len(codes) == 0 { // codesが空だったらエラーで終了
		return errors.New("no target company codes")
	}

	// 株価trendを表示するためのSheet
	trendSheet := sheet.NewSpreadSheet(srv, mustGetenv("TREND_SHEETID"), "trend")

	// daily処理の進捗を管理するためのSheet
	statusSheet := sheet.NewSpreadSheet(srv, mustGetenv("STATUS_SHEETID"), "status")
	d := daily{
		status: statusSheet,
		dayoff: dayoff,
		dailyStockPrice: DailyStockPrice{
			db:                 db,
			dailyStockpriceURL: mustGetenv("DAILY_PRICE_URL"),                                                          // 日足株価scrape先のURL
			fetchInterval:      time.Duration(strToInt(useEnvOrDefault("SCRAPE_INTERVAL", "1000"))) * time.Millisecond, // スクレイピングの間隔(millisec)
			fetchTimeout:       time.Duration(strToInt(useEnvOrDefault("SCRAPE_TIMEOUT", "1000"))) * time.Millisecond,  // スクレイピングのtimeout(millisec)
		},
		calculateMovingAvg: CalculateMovingAvg{
			db:              db,
			calcConcurrency: strToInt(useEnvOrDefault("CALC_MOVINGAVG_CONCURRENCY", "3")), // 最大同時並行数
		},
		calculateGrowthTrend: CalculateGrowthTrend{
			db:              db,
			sheet:           trendSheet,
			calcConcurrency: strToInt(useEnvOrDefault("CALC_GROWTHTREND_CONCURRENCY", "3")),                               // 最大同時並行数
			targetDate:      useEnvOrDefault("GROWTHTREND_TARGETDATE", time.Now().AddDate(0, 0, -1).Format("2006/01/02")), // defaultは起動日の前日
		},
	}
	if err := d.exec(ctx, codes); err != nil {
		return fmt.Errorf("failed to daily: %v", err)
	}

	// 一週間に一度（土曜日？）はbackup

	return nil
}

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Panicf("environment variable '%s' not set", k)
	}
	log.Printf("%s environment variable set", k)

	// if d := os.Getenv("DEBUG"); d == "on" {
	// 	log.Printf("%s: %s", k, v)
	// }
	return v
}

func useEnvOrDefault(key, def string) string {
	v := def
	if fromEnv := os.Getenv(key); fromEnv != "" {
		v = fromEnv
	}
	log.Printf("%s environment variable set", key)
	return v
}

func strToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Panicf("failed to convert %s to int", s)
	}
	return i
}

func getDSN(usr, pwd, host string) string {
	cred := strings.TrimRight(usr, "\n")
	if pwd != "" {
		cred = cred + ":" + strings.TrimRight(pwd, "\n")
	}
	return fmt.Sprintf("%s@tcp(%s)", cred, strings.TrimRight(host, "\n"))
}

func getDatabase(ctx context.Context) (database.DB, error) {
	var db database.DB

	switch {
	case env == "prod":
		// prod環境ならPASSWORD必須
		log.Println("this is prod. trying to connect database...")

		// DBにつながるまでretryする
		if err := retry.WithContext(ctx, 120, 10*time.Second, func() error {
			var e error
			db, e = database.NewDB(fmt.Sprintf("%s/%s",
				getDSN(mustGetenv("DB_USER"),
					mustGetenv("DB_PASSWORD"),
					"127.0.0.1:3306"),
				"stockprice")) // TODO: ここもmustGetenv("DB_NAME")にしていいかも

			return e
		}); err != nil {
			return nil, fmt.Errorf("failed to NewDB: %w", err)
		}
	case env == "dev":
		log.Println("this is dev. trying to connect database...")

		// DBにつながるまでretryする
		if err := retry.WithContext(ctx, 120, 10*time.Second, func() error {
			var e error
			db, e = database.NewDB(fmt.Sprintf("%s/%s",
				getDSN("root", "", "127.0.0.1:3306"),
				"stockprice_dev"))
			return e
		}); err != nil {
			return nil, fmt.Errorf("failed to NewDB: %w", err)
		}
	default:
		log.Println("this is local")

		var err error
		db, err = database.NewDB("root@/stockprice_dev")
		if err != nil {
			return nil, fmt.Errorf("failed to NewDB: %w", err)
		}
	}
	return db, nil
}

func getSheetService(ctx context.Context, credential string) (*sheets.Service, error) {
	srv, err := sheet.GetSheetClient(ctx, credential)
	if err != nil {
		return nil, fmt.Errorf("failed to get sheet service. err: %v", err)
	}
	return srv, nil
}

// どこか別のファイルに持っていく
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
	return codes, nil
}

// 引数として与えた関数内でPanicが生じたらrecoverでキャッチしてエラーに書きだす
func receivePanic(fn func() error) (err error) {
	// ref. https://blog.golang.org/defer-panic-and-recover
	// https://yourbasic.org/golang/recover-from-panic/
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("recovered in function : %v", e)
		}
	}()
	return fn()
}
