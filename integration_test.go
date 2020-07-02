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

	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/gcloud"
	"github.com/ludwig125/gke-stockprice/retry"
	"github.com/ludwig125/gke-stockprice/sheet"
)

func TestGKEStockPrice(t *testing.T) {

	// 事前にtest用CloudSQLを作成する
	// 作成に時間がかかる場合は停止するだけにしておいて、
	// 現在のステータスを確認後に、開始する

	// TODO: 停止起動はCurlでできるのであとで書き換えてもいい
	// https://cloud.google.com/sql/docs/mysql/start-stop-restart-instance

	// CloudSQLの起動をステータスから確認する
	/*example:
	        $gcloud sql instances list
	        NAME                     DATABASE_VERSION  LOCATION       TIER         PRIMARY_ADDRESS  PRIVATE_ADDRESS  STATUS
			gke-stockprice-testdb    MYSQL_5_7         us-central1-c  db-f1-micro  34.66.91.128     -                STOPPED
	*/

	instance := gcloud.CloudSQLInstance{
		Project: "gke-stockprice",
		// Instance: "gke-stockprice-cloudsql-integration-test-202006260624",
		Instance:     "gke-stockprice-cloudsql-integration-test-" + time.Now().Format("200601021504"),
		Tier:         "db-f1-micro",
		Region:       "us-central1",
		DatabaseName: "stockprice_dev",
		ExecCmd:      true, // 実際に作成削除を行う
	}

	// test用GKEクラスタ
	cluster := gcloud.GKECluster{
		Project:     "gke-stockprice",
		ClusterName: "gke-stockprice-cluster-integration-test",
		ComputeZone: "us-central1-f",
		MachineType: "g1-small",
		ExecCmd:     true, // 実際に作成削除を行う
	}
	defer func() {
		log.Println("integration test finished")
	}()
	defer func() {
		if err := instance.DeleteInstance(); err != nil {
			t.Errorf("failed to DeleteInstance: %#v", err)
		}
		log.Printf("delete SQL instance %#v successfully", instance)
	}()
	defer func() {
		if err := cluster.DeleteCluster(); err != nil {
			t.Errorf("failed to DeleteCluster: %#v", err)
		}
		log.Printf("delete GKE cluster %#v successfully", cluster.ClusterName)
	}()

	// SQLInstance作成、DB作成
	if err := setupSQLInstance(instance); err != nil {
		t.Fatalf("failed to setupSQLInstance: %v", err)
	}
	log.Printf("created SQL instance %#v and created test database %s successfully", instance, instance.DatabaseName)

	if err := startCloudSQLProxy(instance); err != nil {
		t.Fatalf("failed to startCloudSQLProxy: %v", err)
	}
	// test用databaseとtableの作成
	// deferによってテスト終了時に削除する
	_, err := database.SetupTestDB(3307) // 3307 はCloudSQL用のport
	//cleanup, err := database.SetupTestDB(3307) // 3307 はCloudSQL用のport
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	//defer cleanup()

	// GKECluster作成
	if err := setupGKECluster(cluster); err != nil {
		t.Fatalf("failed to setupGKECluster: %v", err)
	}
	log.Printf("created GKE cluster %#v successfully", cluster)

	// SpreadSheetに必要なデータを入れる
	// TODO: unit testとかぶっているのであとで直す
	// holiday
	// code

	// GKE Nikkei mockデプロイ
	if err := deployGKENikkeiMock(); err != nil {
		t.Fatalf("failed to deployGKENikkeiMock: %v", err)
	}

	// GKE Stockpriceデプロイ
	// 	kustomize buildの際に、以下の方法でkustomize edit add configmap することで、testサーバをscraping先として設定できそう
	// https://github.com/kubernetes-sigs/kustomize/blob/master/examples/springboot/README.md#add-configmap-generator
	// 同様に、cloud sqlのIDもkustomize edit add configmap で設定できそう
	if err := deployGKEStockprice(instance); err != nil {
		t.Fatalf("failed to deployGKEStockprice: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// retryしながらCloudSQLにデータが入るまで待つ
	if err := checkTestDataInDB(ctx); err != nil {
		t.Errorf("failed to checkTestDataInDB: %v", err)
	}
	// retryしながらSpreadsheetにデータが入るまで待つ
	if err := checkTestDataInSheet(ctx); err != nil {
		t.Errorf("failed to checkTestDataInSheet: %v", err)
	}

	// displaylogs()

	// 成功したら、一旦cronを止めて、
	// 次は、test用サーバのURLを本物のURLに差し替え（このURLは環境変数から取得する）てデプロイし直す
	// 何かしらのデータが入っていたらOK

	// 成功してもしなくても、test用GKEクラスタを削除する
	// 成功してもしなくても、test用CloudSQLを削除(または停止)する

}

func setupSQLInstance(instance gcloud.CloudSQLInstance) error {
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

func setupGKECluster(cluster gcloud.GKECluster) error {
	// すでにSQLInstanceが存在するかどうか確認
	clList, err := cluster.ListCluster()
	if err != nil {
		return fmt.Errorf("failed to ListCluster: %w", err)
	}

	// GKEクラスタがないときは作成する
	if clList == nil {
		log.Println("GKE cluster does not exists. trying to create...")
		if cluster.CreateCluster(); err != nil {
			return fmt.Errorf("failed to CreateCluster: %#v", err)
		}
	}

	if err := cluster.ConfirmClusterStatus("RUNNING"); err != nil {
		return fmt.Errorf("failed to ConfirmClusterStatus: %w", err)
	}

	if err := cluster.GetCredentials(); err != nil {
		return fmt.Errorf("failed to GetCredentials: %w", err)
	}

	return nil
}

func deployGKENikkeiMock() error {
	if err := gcloud.GKEDeploy("./nikkei_mock/k8s/"); err != nil {
		return fmt.Errorf("failed to deploy: %#v", err)
	}
	return nil
}

func deployGKEStockprice(instance gcloud.CloudSQLInstance) error {
	fmt.Printf("instance: \n    %#v\n", instance)

	ist, err := instance.DescribeInstance()
	if err != nil {
		return fmt.Errorf("failed to DescribeInstance: %v", err)
	}

	clusterIP, err := nikkeiMockClusterIP()
	if err != nil {
		return fmt.Errorf("failed to nikkeiMockClusterIP: %v", err)
	}

	// nikkei mockの直近のテストデータをtargetDateとしてGROWTHTREND_TARGETDATEに設定する
	targetDate, err := formatDate(time.Now(), "5/16") // testdataの最新の日付
	if err != nil {
		return fmt.Errorf("failed to formatDate: %v", err)
	}

	// TODO "secret" じゃなくて普通にConfigでいい気がする
	// Secretを環境変数として読み込むためにファイルを配置する
	secretFiles := []gcloud.GKESecretFile{
		{
			Filename: "db_connection_name.txt",
			Content:  ist.ConnectionName,
		},
		{
			Filename: "daily_price_url.txt",
			Content:  "http://" + clusterIP,
		},
		{
			Filename: "growthtrend_targetdate.txt",
			Content:  targetDate,
		},
	}

	// test用Secretファイルを配置
	if err := gcloud.GKESetFilesForDevEnv("./k8s/overlays/dev/", secretFiles); err != nil {
		return fmt.Errorf("failed to GKESetFilesForDevEnv: %#v", err)
	}

	if err := gcloud.GKEDeploy("./k8s/overlays/dev/"); err != nil {
		return fmt.Errorf("failed to deploy: %#v", err)
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

func startCloudSQLProxy(instance gcloud.CloudSQLInstance) error {
	ist, err := instance.DescribeInstance()
	if err != nil {
		return fmt.Errorf("failed to DescribeInstance: %#v", err)
	}
	// コマンドの実行を待たないのでチャネルは捨てる
	// TODO: CommandContextや、Process.Killなどを使ってあとから止められるようにする
	// https://golang.org/pkg/os/exec/#CommandContext
	// https://golang.org/pkg/os/#Process.Kill
	//_, err = command.Exec(fmt.Sprintf("./cloud_sql_proxy -instances=%s=tcp:3307", ist.ConnectionName))
	_, err = command.Exec(fmt.Sprintf("./cloud_sql_proxy -instances=%s=tcp:3307", ist.ConnectionName))
	if err != nil {
		return fmt.Errorf("failed to start cloud_sql_proxy: %v", err)
	}
	time.Sleep(3 * time.Second) // cloud_sql_proxyを立ち上げてから接続できるまで若干時差がある
	log.Println("start cloud_sql_proxy successfully")
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
		if e != nil {
			return e
		}
		return nil
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

func checkTestDataInSheet(ctx context.Context) error {
	// spreadsheetのserviceを取得
	sheetCredential := mustGetenv("SHEET_CREDENTIAL")
	srv, err := sheet.GetSheetClient(ctx, sheetCredential)
	if err != nil {
		return fmt.Errorf("failed to get sheet service. err: %v", err)
	}
	log.Println("got sheet service successfully")

	if err := retry.WithContext(ctx, 20, 3*time.Second, func() error {
		ts := sheet.NewSpreadSheet(srv, mustGetenv("INTEGRATION_TEST_SHEETID"), "trend")
		got, err := ts.Read()
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

// func displaylogs() {

// }
