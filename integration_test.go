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

var cloudSQLPort = 3307

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
	// instance:= "gke-stockprice-cloudsql-integration-test-202009060702"
	instanceName := "gke-stockprice-cloudsql-integration-test-" + time.Now().Format("200601021504")
	region := "us-central1"
	tier := "db-f1-micro"
	databaseName := "stockprice_dev"
	instance, err := cloudsql.NewCloudSQLInstance(instanceName, region, tier, databaseName)
	if err != nil {
		t.Fatalf("failed to NewCloudSQLInstance: %v", err)
	}

	// test用GKEクラスタ作成
	clusterConfig := gke.ClusterConfig{
		ClusterName: "gke-stockprice-cluster-integration-test",
		ComputeZone: "us-central1-f",
		MachineType: "g1-small",
		DiskSize:    10,
		NumNodes:    3,
		Preemptible: "on",
	}
	cluster, err := gke.NewCluster(clusterConfig)
	if err != nil {
		t.Fatalf("failed to gke.NewCluster: %v", err)
	}

	// SQLInstanceとGKE CLusterの後処理
	defer func() {
		if err := postTest(ctx, dSrv, instance, cluster); err != nil {
			t.Fatalf("failed to postTest: %v", err)
		}
	}()

	// SQLInstanceとGKE CLusterの作成と認証
	if err := preTest(instance, cluster); err != nil {
		t.Fatalf("failed to preTest: %v", err)
	}

	// retryしながらCloudSQLにデータが入るまで待つ
	if err := checkTestDataInDB(ctx, databaseName); err != nil {
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

func preTest(instance *cloudsql.CloudSQLInstance, cluster *gke.Cluster) error {
	instanceErrCh := make(chan error)
	go func() {
		defer close(instanceErrCh)
		start := time.Now()
		if err := instance.CreateInstanceIfNotExist(); err != nil {
			instanceErrCh <- fmt.Errorf("failed to CreateInstanceIfNotExist: %v", err)
			return
		}
		// SQL instance がRUNNABLEかどうか確認する
		if err := instance.ConfirmCloudSQLInstanceStatusRunnable(); err != nil {
			instanceErrCh <- fmt.Errorf("failed to ConfirmCloudSQLInstanceStatusRunnable: %v", err)
			return
		}
		log.Printf("created SQL instance %#v and created test database %s successfully. time: %v", instance, instance.Database, time.Since(start))
	}()

	clusterErrCh := make(chan error)
	go func() {
		defer close(clusterErrCh)
		start := time.Now()
		if err := cluster.CreateClusterIfNotExist(); err != nil {
			clusterErrCh <- fmt.Errorf("failed to CreateClusterIfNotExist: %v", err)
			return
		}
		// GKE clusterがRUNNINGかどうか確認する
		if err := cluster.EnsureClusterStatusRunning(); err != nil {
			clusterErrCh <- fmt.Errorf("failed to EnsureClusterStatusRunning: %v", err)
			return
		}
		log.Printf("created GKE cluster %#v successfully. time: %v", cluster, time.Since(start))
		if err := cluster.GetCredentials(); err != nil {
			clusterErrCh <- fmt.Errorf("failed to GetCredentials: %v", err)
			return
		}
		log.Println("got GKE clustercredentials successfully")
	}()

	if err := <-instanceErrCh; err != nil {
		return fmt.Errorf("failed to create instance: %v", err)
	}

	if err := <-clusterErrCh; err != nil {
		return fmt.Errorf("failed to create cluster: %v", err)
	}

	sqlConnectionName, err := instance.ConnectionName()
	if err != nil {
		return fmt.Errorf("failed to get instance ConnectionName: %v", err)
	}
	// cloudSQLにmysqlclientから接続するためにCloudSQLProxyを立ち上げる
	if err := runCloudSQLProxy(sqlConnectionName); err != nil {
		return fmt.Errorf("failed to runCloudSQLProxy: %v", err)
	}
	// test用databaseとtableの作成
	// SQLInstanceごと消すので、終了時にDatabaseは消さない
	_, err = database.SetupTestDB(3307) // 3307 はCloudSQL用のport
	if err != nil {
		return fmt.Errorf("failed to SetupTestDB: %v", err)
	}

	// GKE Nikkei mockデプロイ
	if err := gke.KustomizeBuildAndDeploy("./nikkei_mock/k8s/"); err != nil {
		return fmt.Errorf("failed to KustomizeBuildAndDeploy nikkei_mock: %v", err)
	}

	// kubernetesデプロイ前に必要なファイルを配置
	if err := setFiles(sqlConnectionName); err != nil {
		return fmt.Errorf("failed to setFiles: %v", err)
	}

	// GKE stockpriceデプロイ
	if err := gke.KustomizeBuildAndDeploy("./k8s/overlays/dev/"); err != nil {
		return fmt.Errorf("failed to KustomizeBuildAndDeploy: %#v", err)
	}

	return nil
}

func postTest(ctx context.Context, dSrv *drive.Service, instance *cloudsql.CloudSQLInstance, cluster *gke.Cluster) error {
	instanceErrCh := make(chan error)
	go func() {
		defer close(instanceErrCh)
		if !deleteAllAtLast {
			log.Printf("don't delete SQL instance %#v", instance)
			return
		}
		start := time.Now()
		if err := instance.DeleteInstanceIfExist(); err != nil {
			instanceErrCh <- fmt.Errorf("failed to DeleteInstanceIfExist: %v", err)
			return
		}
		log.Printf("delete SQL instance %#v successfully. time: %v", instance, time.Since(start))
	}()

	clusterErrCh := make(chan error)
	go func() {
		defer close(clusterErrCh)

		folderName := mustGetenv("DRIVE_FOLDER_NAME")
		permissionTargetGmail := mustGetenv("DRIVE_PERMISSION_GMAIL")
		fileName := "kubectl_logs"
		dumpTime := now()
		// kubectl logsの結果をupload
		if err := uploadKubectlLog(ctx, dSrv, folderName, permissionTargetGmail, fileName, dumpTime); err != nil {
			log.Printf("failed to uploadKubectlLog: %v", err)
		}

		if !deleteAllAtLast {
			log.Printf("don't delete GKE cluster %v", cluster.ClusterName)
			return
		}
		start := time.Now()
		if err := cluster.DeleteClusterIfExist(); err != nil {
			clusterErrCh <- fmt.Errorf("failed to DeleteCluster: %#v", err)
			return
		}
		log.Printf("delete GKE cluster %v successfully. time: %v", cluster.ClusterName, time.Since(start))
	}()

	if err := <-instanceErrCh; err != nil {
		return fmt.Errorf("failed to delete instance: %v", err)
	}

	if err := <-clusterErrCh; err != nil {
		return fmt.Errorf("failed to delete cluster: %v", err)
	}
	return nil
}

func setFiles(connectionName string) error {
	clusterIP, err := nikkeiMockClusterIP()
	if err != nil {
		return fmt.Errorf("failed to nikkeiMockClusterIP: %v", err)
	}

	// nikkei mockの直近のテストデータをtargetDateとしてCALC_TREND_TARGETDATEに設定する
	// テストデータには'5/16'のような形式でしか日付が入っていないので、
	// いつの年に実行されても正しく起動するようにformatDate関数を使う
	targetDate, err := formatDate(time.Now(), "5/16") // testdataの最新の日付
	if err != nil {
		return fmt.Errorf("failed to formatDate: %v", err)
	}

	fs := []file.File{
		{Name: "db_connection_name.txt", Content: connectionName},
		{Name: "daily_price_url.txt", Content: "http://" + clusterIP},
		{Name: "calc_moving_trend_targetdate.txt", Content: targetDate},
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

func checkTestDataInDB(ctx context.Context, databaseName string) error {
	var db database.DB
	// DBにつながるまでretryする
	if err := retry.WithContext(ctx, 20, 3*time.Second, func() error {
		var e error
		host := fmt.Sprintf("127.0.0.1:%d", cloudSQLPort)
		db, e = database.NewDB(fmt.Sprintf("%s/%s",
			getDSN("root", "", host),
			databaseName))
		return e
	}); err != nil {
		return fmt.Errorf("failed to NewDB: %w", err)
	}

	ret, err := db.SelectDB("SHOW DATABASES")
	if err != nil {
		return fmt.Errorf("failed to SelectDB: %v", err)
	}
	log.Println("SHOW DATABASES:", ret)
	if err := retry.WithContext(ctx, 30, 10*time.Second, func() error {
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
			if i == 0 { // スプレッドシートの最初の行は項目名なので無視する
				continue
			}
			if len(l) > 0 {
				gotCodes = append(gotCodes, l[0])
			}
		}
		wantCodes := []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"}

		// 順序は気にせず比較するためにソートする
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
	c, err := googledrive.NewCommandResultUpload(srv, cmd, fi, -1)
	if err != nil {
		return fmt.Errorf("failed to NewCommandResultUpload: %v", err)
	}
	if err := c.Exec(ctx); err != nil {
		return fmt.Errorf("failed to Exec: %v", err)
	}
	return nil
}
