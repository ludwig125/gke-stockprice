package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ludwig125/gke-stockprice/gke"
	"github.com/ludwig125/gke-stockprice/googledrive"
	"google.golang.org/api/drive/v3"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	credential := mustGetenv("CREDENTIAL_FILEPATH")
	dSrv, err := googledrive.GetDriveService(ctx, credential) // rootディレクトリに置いてあるserviceaccountのjsonを使う
	if err != nil {
		log.Fatalf("failed to GetDriveService: %v", err)
	}
	clusterConfig := gke.ClusterConfig{
		ClusterName: "gke-stockprice-cluster-prod",
		ComputeZone: "us-central1-f",
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
	// GKEクラスタがないときは終了
	if _, ok := cluster.ExtractFromListedCluster(cls); !ok {
		log.Println("GKE cluster does not exist. no need to delete")
		return
	}

	// kubectl logs
	if err := kubectlLogs(ctx, dSrv, cluster); err != nil {
		log.Println("failed to kubectlLogs:", err)
	}
	// cluster削除
	if err := deleteCluster(cluster); err != nil {
		log.Fatalf("failed to deleteCluster: %v", err)
	}

	// Clusterが存在するかどうか確認
	cls, err = cluster.ListCluster()
	if err != nil {
		log.Fatalf("failed to ListCluster: %v", err)
	}
	// GKEクラスタがあるときはエラー
	if _, ok := cluster.ExtractFromListedCluster(cls); ok {
		log.Fatalf("GKE cluster stil exists. failed to delete cluster")
	}
	log.Println("GKE cluster already deleted")
}

func kubectlLogs(ctx context.Context, dSrv *drive.Service, cluster *gke.Cluster) error {
	// kubectl logsの前にクラスタの認証が必要
	if err := cluster.GetCredentials(); err != nil {
		return fmt.Errorf("failed to GetCredentials: %v", err)
	}
	log.Println("got GKE clustercredentials successfully")

	folderName := mustGetenv("DRIVE_FOLDER_NAME")
	permissionTargetGmail := mustGetenv("DRIVE_PERMISSION_GMAIL")
	fileName := "kubectl_logs"
	dumpTime := time.Now()
	// kubectl logsの結果をupload
	if err := uploadKubectlLog(ctx, dSrv, folderName, permissionTargetGmail, fileName, dumpTime); err != nil {
		return fmt.Errorf("failed to uploadKubectlLog: %v", err)
	}
	return nil
}

func deleteCluster(cluster *gke.Cluster) error {
	if d := os.Getenv("DELETE_ALL_AT_LAST"); d == "off" {
		log.Printf("DELETE_ALL_AT_LAST is off. don't delete GKE cluster %v", cluster.ClusterName)
		return nil
	}
	start := time.Now()
	if err := cluster.DeleteClusterIfExist(); err != nil {
		return fmt.Errorf("failed to DeleteClusterIfExist: %#v", err)

	}
	log.Printf("delete GKE cluster %v successfully. time: %v", cluster.ClusterName, time.Since(start))
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
