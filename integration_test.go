// +build integration

package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/ludwig125/gke-stockprice/cloudsql"
	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/file"
	"github.com/ludwig125/gke-stockprice/gke"
	"github.com/ludwig125/gke-stockprice/googledrive"
	"github.com/ludwig125/gke-stockprice/retry"
	"github.com/ludwig125/gke-stockprice/sheet"
	"google.golang.org/api/drive/v3"
)

// テスト終了時に全部消すかどうか
const deleteAllAtLast = true

func TestGKEStockPrice(t *testing.T) {
	defer func() {
		log.Println("integration test finished")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	credential := mustGetenv("CREDENTIAL_FILEPATH")
	dSrv, err := googledrive.GetDriveService(ctx, credential) // rootディレクトリに置いてあるserviceaccountのjsonを使う
	if err != nil {
		t.Fatalf("failed to GetDriveService: %v", err)
	}

	// SQLInstanceの作成
	instance := cloudsql.CloudSQLInstance{
		Project: "gke-stockprice",
		// Instance: "gke-stockprice-cloudsql-integration-test-202009060702",
		Instance:     "gke-stockprice-cloudsql-integration-test-" + time.Now().Format("200601021504"),
		Tier:         "db-f1-micro",
		Region:       "us-central1",
		DatabaseName: "stockprice_dev",
		ExecCmd:      true, // 実際に作成削除を行う
	}
	// すでにSQLInstanceが存在するかどうか確認
	if err := instance.CreateInstanceIfNotExist(); err != nil {
		t.Fatalf("failed to CreateInstanceIfNotExist: %v", err)
	}
	// SQLInstanceの後処理
	defer func() {
		if !deleteAllAtLast {
			log.Printf("don't delete SQL instance %#v", instance)
			return
		}
		if err := instance.DeleteInstance(); err != nil {
			t.Errorf("failed to DeleteInstance: %v", err)
			return
		}
		// TODO: 本当に消えているかわからないので別に確認したほうが良さそう
		// log.Printf("delete SQL instance %#v successfully", instance)
	}()

	// test用GKEクラスタ作成
	clusterName := "gke-stockprice-cluster-integration-test"
	computeZone := "us-central1-f"
	machineType := "g1-small"
	diskSize := 10
	numNodes := 4
	preemptible := "on"
	cluster, err := gke.NewCluster(clusterName, computeZone, machineType, diskSize, numNodes, preemptible)
	if err != nil {
		t.Fatalf("failed to gke.NewCluster: %v", err)
	}
	if err := cluster.CreateClusterIfNotExist(); err != nil {
		t.Fatalf("failed to gke.CreateClusterIfNotExist: %v", err)
	}
	// GKE Clusterの後処理
	defer func() {
		folderName := mustGetenv("DRIVE_FOLDER_NAME")
		permissionTargetGmail := mustGetenv("DRIVE_PERMISSION_GMAIL")
		fileName := "kubectl_logs"
		dumpTime := now()
		// kubectl logsの結果をupload
		if err := uploadKubectlLog(ctx, dSrv, folderName, permissionTargetGmail, fileName, dumpTime); err != nil {
			t.Errorf("failed to uploadKubectlLog: %v", err)
		}

		if !deleteAllAtLast {
			log.Printf("don't delete GKE cluster %v", cluster.ClusterName)
			return
		}
		if err := cluster.DeleteCluster(); err != nil {
			t.Errorf("failed to DeleteCluster: %#v", err)
			return
		}
		// TODO: 本当に消えているかわからないので別に確認したほうが良さそう
		// log.Printf("delete GKE cluster %v successfully", cluster.ClusterName)
	}()

	// SQL instance がRUNNABLEかどうか確認する
	if err := instance.ConfirmCloudSQLInstanceStatus("RUNNABLE"); err != nil {
		t.Fatalf("failed to ConfirmCloudSQLInstanceStatus: %v", err)
	}
	log.Printf("created SQL instance %#v and created test database %s successfully", instance, instance.DatabaseName)

	// GKE clusterがRUNNINGかどうか確認する
	if err := cluster.EnsureClusterStatusRunning(); err != nil {
		t.Fatalf("failed to gke.EnsureClusterStatusRunning: %v", err)
	}
	log.Printf("created GKE cluster %#v successfully", cluster)
	if err := cluster.GetCredentials(); err != nil {
		t.Fatalf("failed to GetCredentials: %v", err)
	}
	log.Println("got GKE clustercredentials successfully")

	sqlConnectionName, err := instance.ConnectionName()
	if err != nil {
		t.Fatalf("failed to get instance ConnectionName: %v", err)
	}
	// cloudSQLにmysqlclientから接続するためにCloudSQLProxyを立ち上げる
	if err := runCloudSQLProxy(sqlConnectionName); err != nil {
		t.Fatalf("failed to runCloudSQLProxy: %v", err)
	}
	// test用databaseとtableの作成
	// deferによってテスト終了時に削除する
	// _, err = database.SetupTestDB(3307) // 3307 はCloudSQL用のport
	cleanup, err := database.SetupTestDB(3307) // 3307 はCloudSQL用のport
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	defer func() {
		if !deleteAllAtLast {
			log.Println("don't delete database")
			return
		}
		cleanup()
	}()

	// GKE Nikkei mockデプロイ
	if err := gke.KustomizeBuildAndDeploy("./nikkei_mock/k8s/"); err != nil {
		t.Fatalf("failed to KustomizeBuildAndDeploy nikkei_mock: %v", err)
	}

	// kubernetesデプロイ前に必要なファイルを配置
	if err := setFiles(sqlConnectionName); err != nil {
		t.Fatalf("failed to setFiles: %v", err)
	}

	// GKE stockpriceデプロイ
	if err := gke.KustomizeBuildAndDeploy("./k8s/overlays/dev/"); err != nil {
		t.Fatalf("failed to KustomizeBuildAndDeploy: %#v", err)
	}
	// retryしながらCloudSQLにデータが入るまで待つ
	if err := checkTestDataInDB(ctx); err != nil {
		t.Errorf("failed to checkTestDataInDB: %v", err)
	}

	// spreadsheetのserviceを取得
	sSrv, err := sheet.GetSheetClient(ctx, credential)
	if err != nil {
		t.Fatalf("failed to get sheet service. err: %v", err)
	}
	log.Println("got sheet service successfully")
	sheet := sheet.NewSpreadSheet(sSrv, mustGetenv("INTEGRATION_TEST_SHEETID"), "trend")

	// retryしながらSpreadsheetにデータが入るまで待つ
	if err := checkTestDataInSheet(ctx, sheet); err != nil {
		t.Errorf("failed to checkTestDataInSheet: %v", err)
	}

	// 成功したら、一旦cronを止めて、
	// 次は、test用サーバのURLを本物のURLに差し替え（このURLは環境変数から取得する）てデプロイし直す
	// 何かしらのデータが入っていたらOK

	// 成功してもしなくても、test用GKEクラスタを削除する
	// 成功してもしなくても、test用CloudSQLを削除(または停止)する
}

func setupSQLInstance(instance cloudsql.CloudSQLInstance) error {
	// すでにSQLInstanceが存在するかどうか確認
	ok, err := instance.ExistCloudSQLInstance()
	if err != nil {
		return fmt.Errorf("failed to ExistCloudSQLInstance: %#v", err)
	}
	if !ok {
		// SQLInstanceがないなら作る
		log.Println("SQL Instance does not exists. trying to create...")
		if err := instance.CreateInstance(); err != nil {
			return fmt.Errorf("failed to CreateInstance: %#v", err)
		}
	}

	// RUNNABLEかどうか確認する
	if err := instance.ConfirmCloudSQLInstanceStatus("RUNNABLE"); err != nil {
		return fmt.Errorf("failed to ConfirmCloudSQLInstanceStatus: %w", err)
	}

	return nil
}

func setFiles(connectionName string) error {
	clusterIP, err := nikkeiMockClusterIP()
	if err != nil {
		return fmt.Errorf("failed to nikkeiMockClusterIP: %v", err)
	}

	// nikkei mockの直近のテストデータをtargetDateとしてGROWTHTREND_TARGETDATEに設定する
	// テストデータには'5/16'のような形式でしか日付が入っていないので、
	// いつの年に実行されても正しく起動するようにformatDate関数を使う
	targetDate, err := formatDate(time.Now(), "5/16") // testdataの最新の日付
	if err != nil {
		return fmt.Errorf("failed to formatDate: %v", err)
	}

	fs := []file.File{
		{Name: "db_connection_name.txt", Content: connectionName},
		{Name: "daily_price_url.txt", Content: "http://" + clusterIP},
		{Name: "growthtrend_targetdate.txt", Content: targetDate},
	}

	if err := file.CreateFiles("k8s/overlays/dev", fs...); err != nil {
		return fmt.Errorf("failed to CreateFiles: %v", err)
	}
	return nil
}

func nikkeiMockClusterIP() (string, error) {
	cmd := "kubectl get service gke-nikkei-mock-service -o jsonpath='{.spec.clusterIP}'"
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return res.Stdout, nil
}

// cloud sql proxyを起動する
func runCloudSQLProxy(connectionName string) error {
	// コマンドの実行を待たないのでチャネルは捨てる
	// TODO: CommandContextや、Process.Killなどを使ってあとから止められるようにする
	// https://golang.org/pkg/os/exec/#CommandContext
	// https://golang.org/pkg/os/#Process.Kill
	//_, err = command.Exec(fmt.Sprintf("./cloud_sql_proxy -instances=%s=tcp:3307", ist.ConnectionName))
	if _, err := command.Exec(fmt.Sprintf("./cloud_sql_proxy -instances=%s=tcp:3307", connectionName)); err != nil {
		return fmt.Errorf("failed to run cloud_sql_proxy: %v", err)
	}
	time.Sleep(3 * time.Second) // cloud_sql_proxyを立ち上げてから接続できるまで若干時差がある
	log.Println("run cloud_sql_proxy successfully")
	return nil
}

func checkTestDataInDB(ctx context.Context) error {
	var db database.DB
	// DBにつながるまでretryする
	if err := retry.WithContext(ctx, 20, 3*time.Second, func() error {
		var e error
		db, e = database.NewDB(fmt.Sprintf("%s/%s",
			getDSN("root", "", "127.0.0.1:3307"),
			"stockprice_dev"))
		return e
	}); err != nil {
		return fmt.Errorf("failed to NewDB: %w", err)
	}

	ret, err := db.SelectDB("SHOW DATABASES")
	if err != nil {
		return fmt.Errorf("failed to SelectDB: %v", err)
	}
	log.Println("SHOW DATABASES:", ret)
	if err := retry.WithContext(ctx, 20, 10*time.Second, func() error {
		// tableに格納されたcodeの数を確認
		retCodes, err := db.SelectDB("SELECT DISTINCT code FROM daily")
		if err != nil {
			return fmt.Errorf("failed to SelectDB: %v", err)
		}
		log.Println("SELECT DISTINCT code FROM daily:", retCodes)
		wantCodes := []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"}
		if len(retCodes) != len(wantCodes) {
			return fmt.Errorf("got codes: %d, want: %d", len(retCodes), len(wantCodes))
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to check db: %v", err)
	}
	return nil
}

func checkTestDataInSheet(ctx context.Context, sheet sheet.Sheet) error {
	if err := retry.WithContext(ctx, 20, 3*time.Second, func() error {
		got, err := sheet.Read()
		if err != nil {
			return fmt.Errorf("failed to read sheet: %w", err)
		}
		//fmt.Println("got:", got)
		var gotCodes []string
		for i, l := range got {
			if i == 0 {
				continue
			}
			if len(l) > 0 {
				gotCodes = append(gotCodes, l[0])
			}
		}
		wantCodes := []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"}
		sort.Slice(gotCodes, func(i, j int) bool { return gotCodes[i] < gotCodes[j] })
		if !reflect.DeepEqual(gotCodes, wantCodes) {
			return fmt.Errorf("gotCodes: %v, wantCodes: %v", gotCodes, wantCodes)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to check sheet: %w", err)
	}
	return nil
}

func uploadKubectlLog(ctx context.Context, srv *drive.Service, folderName, permissionTargetGmail, fileName string, dumpTime time.Time) error {
	// フォルダIDの取得（フォルダがなければ作る）
	folderID, err := googledrive.GetFolderIDOrCreate(srv, folderName, permissionTargetGmail) // permission共有Gmailを空にするとユーザにはUIから見ることはできないことに注意
	if err != nil {
		return fmt.Errorf("failed to GetFolderIDOrCreate: %v, folderName(parent folder): %s", err, folderName)
	}

	fi := googledrive.FileInfo{
		Name:        fileName,
		Description: fmt.Sprintf("%s dumpdate: %s", fileName, dumpTime.Format("2006-01-02")),
		MimeType:    "text/plain",
		ParentID:    folderID,
		Overwrite:   true,
	}

	cmd := "kubectl logs $(kubectl get pods | grep stockprice | awk '{print $1}') -c gke-stockprice-container"
	c, err := googledrive.NewCommandResultUpload(srv, cmd, fi)
	if err != nil {
		return fmt.Errorf("failed to NewCommandResultUpload: %v", err)
	}
	if err := c.Exec(ctx); err != nil {
		return fmt.Errorf("failed to Exec: %v", err)
	}
	return nil
}
