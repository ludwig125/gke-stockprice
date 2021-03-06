package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/drive/v3"
	sheets "google.golang.org/api/sheets/v4"

	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/googledrive"
	"github.com/ludwig125/gke-stockprice/retry"
	"github.com/ludwig125/gke-stockprice/sheet"
	"github.com/ludwig125/gke-stockprice/status"
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
	if job := os.Getenv("DELETE_GKE_CLUSTER_JOB"); job != "" {
		log.Println("circleci target DELETE_GKE_CLUSTER_JOB:", job)
		ciToken := mustGetenv("CIRCLE_API_USER_TOKEN")
		defer func() {
			if err := requestCircleci(ciToken, job); err != nil {
				log.Printf("failed to requestCircleci: %v. job:%s", err, job)
				return
			}
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
	if err := receivePanic(func() error { // execProcess内でpanicしたら原因をSlackに伝搬する
		return execProcess(ctx)
	}); err != nil {
		log.Println("failed to execProcess:", err)
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

	log.Println("process finished successfully")
}

func execProcess(ctx context.Context) error {
	// databaseの取得
	db, err := getDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to getDatabase: %v", err)
	}
	defer db.CloseDB()
	log.Println("connected db successfully")

	// spreadsheetのserviceを取得
	srv, err := getSheetService(ctx, mustGetenv("CREDENTIAL_FILEPATH"))
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

	if err := restructureTablesFromDaily(db, codes, statusSheet); err != nil {
		return fmt.Errorf("failed to restructureTablesFromDaily: %v", err)
	}

	d := daily{
		status: statusSheet,
		dayoff: dayoff,
		dailyStockPrice: DailyStockPrice{
			db:                 db,
			dailyStockpriceURL: mustGetenv("DAILY_PRICE_URL"),                                                          // 日足株価scrape先のURL
			fetchInterval:      time.Duration(strToInt(useEnvOrDefault("SCRAPE_INTERVAL", "1000"))) * time.Millisecond, // スクレイピングの間隔(millisec)
			fetchTimeout:       time.Duration(strToInt(useEnvOrDefault("SCRAPE_TIMEOUT", "1000"))) * time.Millisecond,  // スクレイピングのtimeout(millisec)
		},
		calculateDailyMovingAvgTrend: CalculateDailyMovingAvgTrend{
			db:                    db,
			sheet:                 trendSheet,
			calcConcurrency:       strToInt(useEnvOrDefault("CALC_MOVING_TREND_CONCURRENCY", "3")), // 最大同時並列処理数
			targetDate:            calculateTrendTargetDate(),
			longTermThresholdDays: 2, // TODO: どれくらいにすればいいか考える
		},
	}
	if err := d.exec(ctx, codes); err != nil {
		return fmt.Errorf("failed to daily: %v", err)
	}

	// MySQLの中身をGoogleDriveにbackup
	// mysqldump and upload to google drive
	driveSrv, err := googledrive.GetDriveService(ctx, mustGetenv("CREDENTIAL_FILEPATH")) // rootディレクトリに置いてあるserviceaccountのjsonを使う
	if err != nil {
		return fmt.Errorf("failed to GetDriveService: %v", err)
	}
	if err := backupMySQL(ctx, driveSrv); err != nil {
		return fmt.Errorf("failed to backupMySQL: %v", err)
	}

	return nil
}

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Panicf("environment variable '%s' not set", k)
	}

	if d := os.Getenv("DEBUG"); d == "on" {
		log.Printf("%s environment variable set: '%s'", k, v)
	} else {
		log.Printf("%s environment variable set", k)
	}
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

func strToSlice(s string) []string {
	var ss []string
	for _, v := range strings.Split(s, ",") {
		ss = append(ss, v)
	}
	return ss
}

func calculateTrendTargetDate() string {
	date := os.Getenv("CALC_TREND_TARGETDATE")
	if date == "previous_date" {
		return time.Now().AddDate(0, 0, -1).Format("2006/01/02") // defaultは起動日の前日
	}
	return date
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
					useEnvOrDefault("DB_PASSWORD", ""),
					"127.0.0.1:3306"), // TODO: 環境変数から取得する
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

func restructureTablesFromDaily(db database.DB, codes []string, statusSheet sheet.Sheet) error {
	st := status.Status{Sheet: statusSheet} // Status管理用の変数
	start := now()

	executeDate := os.Getenv("RESTRUCTURE_EXECUTE_DATE")
	if executeDate == "" {
		log.Printf("RESTRUCTURE_EXECUTE_DATE is not set. no need to restructure")
		return nil
	}
	today := time.Now().Format("2006/01/02")
	if executeDate != today {
		log.Printf("RESTRUCTURE_EXECUTE_DATE(%s) is not today(%s). no need to restructure", executeDate, today)
		return nil
	}
	log.Printf("RESTRUCTURE_EXECUTE_DATE(%s) is today(%s). Trying to restructure...", executeDate, today)
	defer func() {
		st.InsertStatus("restructureTablesFromDaily", now(), now().Sub(start)) // now().Sub(start)で所要時間も入れておく
		log.Println("restructureTablesFromDaily", now(), now().Sub(start))
	}()

	config := CalcMovingTrendConfig{
		DB:             db,
		DailyTable:     useEnvOrDefault("RESTRUCTURE_FROM_DAILY_TABLE", "daily"),
		MovingAvgTable: mustGetenv("RESTRUCTURE_TO_MOVINGAVG_TABLE"),
		TrendTable:     mustGetenv("RESTRUCTURE_TO_TREND_TABLE"),
		Codes:          codes,
		FromDate:       useEnvOrDefault("RESTRUCTURE_FROM_DATE", time.Now().AddDate(0, 0, -10).Format("2006/01/02")),
		ToDate:         useEnvOrDefault("RESTRUCTURE_TO_DATE", time.Now().Format("2006/01/02")),
		MaxConcurrency: strToInt(useEnvOrDefault("RESTRUCTURE_MAX_CONCURRENCY", "10")),
		// RestructureMovingavg: true,
		// RestructureTrend:     true,
		// TODO: LongTermThresholdDaysも環境変数から指定する
	}
	calc, err := NewCalcMovingTrend(config)
	if err != nil {
		return fmt.Errorf("failed to NewCalcMovingTrend: %w", err)
	}
	if err := calc.Exec(); err != nil {
		return fmt.Errorf("failed to Exec: %w", err)
	}
	return nil
}

func backupMySQL(ctx context.Context, driveSrv *drive.Service) error {
	if d := os.Getenv("MYSQLDUMP_TO_GOOGLEDRIVE"); d != "on" {
		log.Printf("MYSQLDUMP_TO_GOOGLEDRIVE is not on. no need to backup")
		return nil
	}

	dumper, err := NewMySQLDumper(driveSrv,
		DumpConf{
			DumpExecuteDays:       strToSlice(useEnvOrDefault("DUMP_EXECUTE_DAYS", "Sunday")),
			FolderName:            mustGetenv("DRIVE_FOLDER_NAME"),
			PermissionTargetGmail: useEnvOrDefault("DRIVE_PERMISSION_GMAIL", ""),
			MimeType:              useEnvOrDefault("DRIVE_FILE_MIMETYPE", "text/plain"),
			DumpTime:              now(),
			NeedToBackup:          strToInt(useEnvOrDefault("DRIVE_NEED_TO_BACKUP", "3")),
			DBUser:                mustGetenv("DB_USER"),
			DBPassword:            useEnvOrDefault("DB_PASSWORD", ""),
			Host:                  useEnvOrDefault("DB_HOST", "127.0.0.1"),
			Port:                  mustGetenv("DB_PORT"),
			DBName:                mustGetenv("DB_NAME"),
			TableNames:            strToSlice(useEnvOrDefault("DUMP_TARGET_TABLES", "daily,movingavg")),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to NewMySQLDumper: %v", err)
	}
	if err := dumper.MySQLDumpToGoogleDrive(ctx); err != nil {
		return fmt.Errorf("failed to UploadToGoogleDrive: %v", err)
	}
	return nil
}

// 引数として与えた関数内でPanicが生じたらrecoverでキャッチしてエラーに書きだす
func receivePanic(fn func() error) (err error) {
	// ref. https://blog.golang.org/defer-panic-and-recover
	// https://yourbasic.org/golang/recover-from-panic/
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("recovered in function : %v\nstacktrace: %s", e, string(debug.Stack()))
		}
	}()
	return fn()
}
