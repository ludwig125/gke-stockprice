// +build integration

package gcloud

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/retry"
)

// GKECluster has GKE cluster information.
type GKECluster struct {
	Project     string
	ClusterName string
	ComputeZone string
	MachineType string
	ExecCmd     bool
}

func (c GKECluster) CreateCluster() error {
	if !strings.Contains(c.ClusterName, "integration-test") {
		return fmt.Errorf("cluster name should contains 'integration-test'. cluster: %s", c.ClusterName)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !c.ExecCmd {
		log.Println("satisfied the condition")
		return nil
	}

	cmd := fmt.Sprintf(`gcloud --quiet container clusters create %s \
	--disk-size 10 --zone %s --machine-type=%s \
	--num-nodes=4 --preemptible`, c.ClusterName, c.ComputeZone, c.MachineType)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (c GKECluster) DeleteCluster() error {
	if !strings.Contains(c.ClusterName, "integration-test") {
		return fmt.Errorf("cluster name should contains 'integration-test'. cluster: %s", c.ClusterName)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !c.ExecCmd {
		log.Println("satisfied the condition")
		return nil
	}

	cmd := fmt.Sprintf("gcloud --quiet container clusters delete %s", c.ClusterName)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (c GKECluster) ListCluster() (*GKEClusterListed, error) {
	// APIを使った方法がうまく行かなかったのでgcloudコマンドを直接使う方法にした

	if !strings.Contains(c.ClusterName, "integration-test") {
		return nil, fmt.Errorf("cluster name should contains 'integration-test'. cluster: %s", c.ClusterName)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !c.ExecCmd {
		log.Println("satisfied the condition")
		return nil, nil
	}

	cmd := fmt.Sprintf("gcloud container clusters list")
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	// listの結果が空ならすぐ返す
	if res.Stdout == "" {
		return nil, nil
	}

	listed, err := formatlistedCluster(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to formatlistedCluster: %v", err)
	}
	for _, l := range listed {
		// cluster名が一致したらそれを返す
		if l.Name == c.ClusterName {
			return &l, nil
		}
	}
	// 見つからなかったときはnilを返す
	return nil, nil
}

//func (c GKECluster) ListCluster() (*container.ListClustersResponse, error) {
// func (c GKECluster) ListCluster() error {
// 	// 参考
// 	// list API: https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters/list?hl=ja
//  // https://godoc.org/google.golang.org/api/container/v1#ProjectsLocationsClustersService
//  // https://github.com/googleapis/google-api-go-client/blob/c28c262979b964300c57573a9dc590329c72f4de/container/v1beta1/container-gen.go#L6957

// 	ctx := context.Background()

// 	cl, err := google.DefaultClient(ctx, container.CloudPlatformScope)
// 	if err != nil {
// 		return fmt.Errorf("failed to get google.DefaultClient: %v", err)
// 	}

// 	containerService, err := container.New(cl)
// 	if err != nil {
// 		return fmt.Errorf("failed to container.New: %w", err)
// 	}

// 	// The parent (project and location) where the clusters will be listed.
// 	// Specified in the format 'projects/*/locations/*'.
// 	// Location "-" matches all zones and all regions.
// 	parent := c.Project

// 	resp, err := containerService.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
// 	//resp, err := containerService.Projects.Locations.Operations.List(parent).Context(ctx).Do()
// 	//resp, err := containerService.Projects.Locations.Clusters.NodePools.List(parent).Context(ctx).Do()
// 	// r := container.NewProjectsLocationsClustersService(containerService)
// 	// resp, err := r.List(parent).Context(ctx).Do()
// 	if err != nil {
// 		return fmt.Errorf("containerService.Projects.Locations.Clusters.List: %w", err)
// 	}

// 	// if !strings.Contains(c.ClusterName, "integration-test") {
// 	// 	return command.Result{}, fmt.Errorf("cluster name should contains 'integration-test'. cluster: %s", c.ClusterName)
// 	// }

// 	fmt.Printf("%#v\n", resp)
// 	return nil
// }

type GKEClusterListed struct {
	Name          string // NAME
	Location      string // LOCATION
	MasterVersion string // MASTER_VERSION
	MasterIP      string // MASTER_IP
	MachineType   string // MACHINE_TYPE
	NodeVersion   string // NODE_VERSION
	NumNodes      string // NUM_NODES
	Status        string // STATUS
}

func formatlistedCluster(s string) ([]GKEClusterListed, error) {
	var listed []GKEClusterListed

	lines := strings.Split(s, "\n") // 改行区切りでlinesに格納
	for i, l := range lines {
		col := strings.Fields(l)
		if i == 0 {
			// １行目が想定するフォーマットでなければエラー
			if (col[0] != "NAME") || (col[1] != "LOCATION") || (col[2] != "MASTER_VERSION") || (col[3] != "MASTER_IP") || (col[4] != "MACHINE_TYPE") || (col[5] != "NODE_VERSION") || (col[6] != "NUM_NODES") || (col[7] != "STATUS") {
				return nil, fmt.Errorf("format error.\n got '%v'\nexpected format 'NAME LOCATION MASTER_VERSION MASTER_IP MACHINE_TYPE NODE_VERSION NUM_NODES STATUS'", col)
			}
		} else {
			c := GKEClusterListed{
				Name:          col[0],
				Location:      col[1],
				MasterVersion: col[2],
				MasterIP:      col[3],
				MachineType:   col[4],
				NodeVersion:   col[5],
				NumNodes:      col[6],
				Status:        col[7],
			}
			listed = append(listed, c)
		}
	}
	return listed, nil
}

func (c GKECluster) ConfirmClusterStatus(wantStatus string) error {
	if err := retry.Retry(30, 20*time.Second, func() error {
		l, err := c.ListCluster()
		if err != nil {
			return fmt.Errorf("failed to ListCluster: %w", err)
		}
		if l.Status != wantStatus {
			return fmt.Errorf("not matched. current: %s, expected: %s", l.Status, wantStatus)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to confirm gke cluster status: %w", err)
	}
	return nil
}

func (c GKECluster) GetCredentials() error {
	cmd := fmt.Sprintf("gcloud config set container/cluster %s", c.ClusterName)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}

	cmd = fmt.Sprintf("gcloud config set compute/zone %s", c.ComputeZone)
	res, err = command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}

	cmd = fmt.Sprintf("gcloud container clusters get-credentials %s", c.ClusterName)
	res, err = command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	log.Println("get-credentials successfully")
	return nil
}

// func gkeKustomizeEdit(cmd string) error {
// 	// cmd := fmt.Sprintf(`(git checkout k8s/overlays/dev/kustomization.yaml && \
// 	// 	cd k8s/overlays/dev && \
// 	// 	kustomize edit add configmap sql-proxy-config \
// 	// 	--from-literal=db_connection_name=%s \
// 	// 	--from-literal=db_name=%s )`, connectionName, databaseName)
// 	res, err := command.ExecAndWait(cmd)
// 	if err != nil {
// 		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
// 	}
// 	return nil
// }

type GKESecretFile struct {
	Filename string
	Content  string
}

func GKESetFilesForDevEnv(path string, files []GKESecretFile) error {
	for _, f := range files {
		// path: ex. "./k8s/overlays/dev/"
		fmt.Println("filename:", f.Filename)
		fmt.Println("content:", f.Content)
		// 改行入れると正しく認識されないので改行を削る
		// 例
		//   2020/01/11 22:50:08 errors parsing config:
		//   googleapi: Error 400: Invalid request: instance name (gke-stockprice-integration-test-202001100551
		//   )., invalid
		cmd := fmt.Sprintf("echo -n '%s' > %s%s", f.Content, path, f.Filename)
		res, err := command.ExecAndWait(cmd)
		if err != nil {
			return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
		}
	}
	return nil
}

func GKEDeploy(path string) error {
	// path: ex. "./k8s/overlays/dev/"
	cmd := fmt.Sprintf("./kustomize build %s | /usr/bin/kubectl apply -f -", path)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

// KustomizeEditConfigMap
func KustomizeEditConfigMap(path string) error {
	// TODO: そのうちGKESetFilesForDevEnvの代わりにこちらでeditする
	// 以下のは例
	// https://stackoverflow.com/questions/56907734/kustomize-configmap-command-what-does-it-do
	// https://stackoverflow.com/questions/46028707/how-to-change-the-current-dir-in-go/46036376
	cmd := exec.Command("bash", "-c", "kustomize edit add configmap mnist-map-training --from-literal=trainSteps=200")
	cmd.Dir = path // kustomize editは対象のパスで実行しないといけないのでディレクトリを変更する

	return cmd.Run()
}
