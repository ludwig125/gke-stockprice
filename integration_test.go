// +build integration

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/database"
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
	instance := cloudSQLInstance{
		Project: "gke-stockprice",
		//Instance: "gke-stockprice-integration-test-202001200539",
		Instance:     "gke-stockprice-integration-test-" + time.Now().Format("200601021504"),
		Tier:         "db-f1-micro",
		Region:       "us-central1",
		DatabaseName: "stockprice_dev",
		ExecCmd:      true,
	}

	// test用GKEクラスタ
	cluster := gkeCluster{
		Project:     "gke-stockprice",
		ClusterName: "gke-stockprice-integration-test",
		ComputeZone: "us-central1-a",
		MachineType: "g1-small",
		ExecCmd:     true,
	}
	defer func() {
		if err := instance.deleteInstance(); err != nil {
			t.Errorf("failed to deleteInstance: %#v", err)
		}
		log.Printf("delete SQL instance %#v successfully", instance)
	}()
	defer func() {
		if err := cluster.deleteCluster(); err != nil {
			t.Errorf("failed to deleteCluster: %#v", err)
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
	cleanup, err := database.SetupTestDB(3307) // 3307 はCloudSQL用のport
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	defer cleanup()

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
	// retryしながら、CloudSQLとSpreadsheetにデータが入るまで待つ
	if err := checkTestDataInDBSheet(ctx); err != nil {
		t.Fatalf("failed to checkTestDataInDBSheet: %v", err)
	}

	// 成功したら、一旦cronを止めて、
	// 次は、test用サーバのURLを本物のURLに差し替え（このURLは環境変数から取得する）てデプロイし直す
	// 何かしらのデータが入っていたらOK

	// 成功してもしなくても、test用GKEクラスタを削除する
	// 成功してもしなくても、test用CloudSQLを削除(または停止)する

}

func setupSQLInstance(instance cloudSQLInstance) error {
	// すでにSQLInstanceが存在するかどうか確認
	ist, err := instance.listInstance()
	if err != nil {
		return fmt.Errorf("failed to listInstance: %#v", err)
	}
	// SQLInstanceがないなら作る
	if ist == nil {
		log.Println("SQL Instance does not exists. trying to create...")
		if err := instance.createInstance(); err != nil {
			return fmt.Errorf("failed to createInstance: %#v", err)
		}
	}
	// RUNNABLEかどうか確認する
	if err := instance.confirmcloudSQLInstanceStatus("RUNNABLE"); err != nil {
		return fmt.Errorf("failed to confirmcloudSQLInstanceStatus: %w", err)
	}

	// // テスト用Databaseを作成
	// if err := instance.findDatabase(); err != nil {
	// 	// まだないなら作る
	// 	if err2 := instance.createTestDatabase(); err2 != nil {
	// 		return fmt.Errorf("failed to createTestDatabase: %w", err2)
	// 	}
	// }

	// // テスト用Databaseが作成されたか確認
	// if err := instance.findDatabase(); err != nil {
	// 	return fmt.Errorf("failed to findDatabase: %w", err)
	// }
	return nil
}

func setupGKECluster(cluster gkeCluster) error {
	// すでにSQLInstanceが存在するかどうか確認
	clList, err := cluster.listCluster()
	if err != nil {
		return fmt.Errorf("failed to listCluster: %w", err)
	}
	//fmt.Println(clList)
	// GKEクラスタがないときは作成する
	if clList == nil {
		log.Println("GKE cluster does not exists. trying to create...")
		if cluster.createCluster(); err != nil {
			return fmt.Errorf("failed to createCluster: %#v", err)
		}
	}

	if err := cluster.confirmClusterStatus("RUNNING"); err != nil {
		return fmt.Errorf("failed to confirmClusterStatus: %w", err)
	}

	return nil
}

func deployGKENikkeiMock() error {
	if err := gkeDeploy("./nikkei_mock/k8s/"); err != nil {
		return fmt.Errorf("failed to deploy: %#v", err)
	}
	return nil
}

func deployGKEStockprice(instance cloudSQLInstance) error {
	fmt.Printf("instance: \n    %#v\n", instance)

	ist, err := instance.listInstance()
	if err != nil {
		return fmt.Errorf("failed to listInstance: %v", err)
	}

	clusterIP, err := nikkeiMockClusterIP()
	if err != nil {
		return fmt.Errorf("failed to nikkeiMockClusterIP: %v", err)
	}

	// Secretを環境変数として読み込むためにファイルを配置する
	secretFiles := []gkeSecretFile{
		gkeSecretFile{
			filename: "dev_db_connection_name.txt",
			content:  ist.ConnectionName,
		},
		gkeSecretFile{
			filename: "dev_daily_price_url.txt",
			content:  "http://" + clusterIP,
		},
	}

	// test用Secretファイルを配置
	if err := gkeSetFilesForDevEnv("./k8s/overlays/dev/", secretFiles); err != nil {
		return fmt.Errorf("failed to gkeSetFilesForDevEnv: %#v", err)
	}

	if err := gkeDeploy("./k8s/overlays/dev/"); err != nil {
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

func startCloudSQLProxy(instance cloudSQLInstance) error {
	ist, err := instance.listInstance()
	if err != nil {
		return fmt.Errorf("failed to listInstance: %#v", err)
	}
	// コマンドの実行を待たないのでチャネルは捨てる
	// TODO: CommandContextや、Process.Killなどを使ってあとから止められるようにする
	// https://golang.org/pkg/os/exec/#CommandContext
	// https://golang.org/pkg/os/#Process.Kill
	_, err = command.Exec(fmt.Sprintf("./cloud_sql_proxy -instances=%s=tcp:3307", ist.ConnectionName))
	if err != nil {
		fmt.Errorf("failed to start cloud_sql_proxy: %v", err)
	}
	time.Sleep(3 * time.Second) // cloud_sql_proxyを立ち上げてから接続できるまで若干時差がある
	log.Println("start cloud_sql_proxy successfully")
	return nil
}

func checkTestDataInDBSheet(ctx context.Context) error {
	var db database.DB
	// DBにつながるまでretryする
	if err := WithContext(ctx, 20, 3*time.Second, func() error {
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

	if err := WithContext(ctx, 20, 5*time.Second, func() error {
		ret, err := db.SelectDB("SHOW DATABASES")
		if err != nil {
			return fmt.Errorf("failed to SelectDB: %v", err)
		}
		log.Println("SHOW DATABASES:", ret)
		// tableに格納されたcodeの数を確認
		retCodes, err := db.SelectDB("SELECT DISTINCT code FROM daily")
		if err != nil {
			return fmt.Errorf("failed to SelectDB: %v", err)
		}
		testcodes := []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"}
		if len(retCodes) != len(testcodes) {
			return fmt.Errorf("got codes: %d, want: %d", len(retCodes), len(testcodes))
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to check db: %v", err)
	}
	return nil
}

// 	// // spreadsheetのserviceを取得
// 	// sheetCredential := mustGetenv("SHEET_CREDENTIAL")
// 	// srv, err := sheet.GetSheetClient(ctx, sheetCredential)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to get sheet service. err: %v", err)
// 	// }
// 	// log.Println("got sheet service successfully")

// 	return nil
// }

func getDSN(usr, pwd, host string) string {
	cred := strings.TrimRight(usr, "\n")
	if pwd != "" {
		cred = cred + ":" + strings.TrimRight(pwd, "\n")
	}
	return fmt.Sprintf("%s@tcp(%s)", cred, strings.TrimRight(host, "\n"))
}
