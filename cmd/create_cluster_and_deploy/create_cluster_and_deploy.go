package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/gke"
)

func main() {
	clusterConfig := gke.ClusterConfig{
		ClusterName: "gke-stockprice-cluster-prod", // TODO: 環境変数から取得したほうがいいかどうか?
		ComputeZone: "us-central1-f",
		MachineType: "g1-small",
		NumNodes:    3, // 2だと`nodes are available: 1 Insufficient memory, 2 Insufficient cpu.` となったので
	}
	cluster, err := gke.NewCluster(clusterConfig)
	if err != nil {
		log.Fatalf("failed to gke.NewCluster: %v", err)
	}

	// Clusterが存在するかどうか確認
	cls, err := cluster.ListCluster()
	if err != nil {
		log.Fatalf("failed to ListCluster: %v", err)
	}
	// GKEクラスタがあるときは終了
	if _, ok := cluster.ExtractFromListedCluster(cls); ok {
		log.Println("GKE cluster does not exist. no need to delete")
		return
	}
	// cloud sql instanceの存在確認
	if err := checkCloudSQLInstance(); err != nil {
		log.Fatalf("failed to checkCloudSQLInstance: %v", err)
	}

	// cluster作成
	if err := createCluster(cluster); err != nil {
		log.Fatalf("failed to deleteCluster: %v", err)
	}

	// Clusterが存在するかどうか確認
	cls, err = cluster.ListCluster()
	if err != nil {
		log.Fatalf("failed to ListCluster: %v", err)
	}
	// GKEクラスタがないときはエラー
	if _, ok := cluster.ExtractFromListedCluster(cls); !ok {
		log.Fatalf("GKE cluster not exists. failed to create cluster")
	}
	log.Println("GKE cluster exist")

	// deployの前にクラスタの認証が必要
	if err := cluster.GetCredentials(); err != nil {
		log.Fatalf("failed to GetCredentials: %v", err)
	}
	log.Println("got GKE clustercredentials successfully")

	// GKE stockpriceデプロイ
	if err := gke.KustomizeBuildAndDeploy("./k8s/overlays/prod/"); err != nil {
		log.Fatalf("failed to KustomizeBuildAndDeploy: %#v", err)
	}
	log.Println("GKE stockprice deployed successfuly")
}

func checkCloudSQLInstance() error {
	sqlInstance := "gke-stockprice-cloudsql-prod"
	// gcloud sql instances listの結果が0だと、`Listed 0 items.`が出力されるので/dev/nullに捨てる
	cmd := fmt.Sprintf("gcloud sql instances list 2> /dev/null | grep %s", sqlInstance)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	if !strings.Contains(res.Stdout, "RUNNABLE") {
		return fmt.Errorf("failed to confirm %s RUNNABLE. res: %v", sqlInstance, res)
	}
	return nil
}

// func kubectlLogs(ctx context.Context, dSrv *drive.Service) {
// 	folderName := mustGetenv("DRIVE_FOLDER_NAME")
// 	permissionTargetGmail := mustGetenv("DRIVE_PERMISSION_GMAIL")
// 	fileName := "kubectl_logs"
// 	dumpTime := time.Now()
// 	// kubectl logsの結果をupload
// 	if err := uploadKubectlLog(ctx, dSrv, folderName, permissionTargetGmail, fileName, dumpTime); err != nil {
// 		log.Printf("failed to uploadKubectlLog: %v", err)
// 	}
// }

func createCluster(cluster *gke.Cluster) error {
	start := time.Now()
	if err := cluster.CreateClusterIfNotExist(); err != nil {
		return fmt.Errorf("failed to CreateClusterIfNotExist: %#v", err)

	}
	log.Printf("create GKE cluster %v successfully. time: %v", cluster.ClusterName, time.Since(start))
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
