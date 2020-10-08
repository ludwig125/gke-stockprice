package gke

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/retry"
)

// Cluster has GKE cluster information.
type Cluster struct {
	ClusterName string
	ComputeZone string
	MachineType string
	DiskSize    int
	NumNodes    int
	Preemptible string
}

// ClusterConfig is configuration for new gke cluster
type ClusterConfig struct {
	ClusterName string
	ComputeZone string
	MachineType string
	DiskSize    int
	NumNodes    int
	Preemptible string
}

// NewCluster Cluster constructor.
func NewCluster(c ClusterConfig) (*Cluster, error) {
	if c.ClusterName == "" {
		return nil, errors.New("clusterName is empty")
	}
	if c.ComputeZone == "" {
		return nil, errors.New("computeZone is empty")
	}
	machineType := "g1-small"
	if c.MachineType != "" {
		machineType = c.MachineType
	}
	diskSize := 10
	if c.DiskSize != 0 {
		diskSize = c.DiskSize
	}
	numNodes := 4
	if c.NumNodes != 0 {
		numNodes = c.NumNodes
	}
	preemptible := "on"
	if c.Preemptible == "off" {
		preemptible = "off"
	}

	return &Cluster{
		ClusterName: c.ClusterName,
		ComputeZone: c.ComputeZone,
		MachineType: machineType,
		DiskSize:    diskSize,
		NumNodes:    numNodes,
		Preemptible: preemptible,
	}, nil
}

// CreateCluster creates gke cluster.
func (c Cluster) CreateCluster() error {
	cmd := c.createClusterCommand()
	res, err := command.ExecAndWait(cmd) // コマンドの完了を待つ
	if err != nil {
		return fmt.Errorf("failed to Exec: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	log.Printf("CreateCluster result: %#v", res)
	return nil
}

func (c Cluster) createClusterCommand() string {
	preemptible := ""
	if c.Preemptible == "on" {
		preemptible = "--preemptible"
	}
	// 作るかどうかy/n の入力を待たないように-quietオプションつけている
	return fmt.Sprintf(`gcloud --quiet container clusters create %s --zone %s --machine-type=%s --disk-size %d --num-nodes=%d %s`, c.ClusterName, c.ComputeZone, c.MachineType, c.DiskSize, c.NumNodes, preemptible)
}

// CreateClusterIfNotExist creates gke cluster if cluster does not exist.
func (c Cluster) CreateClusterIfNotExist() error {
	// Clusterが存在するかどうか確認
	cls, err := c.ListCluster()
	if err != nil {
		return fmt.Errorf("failed to ListCluster: %w", err)
	}

	// GKEクラスタがないときは作成する
	if _, ok := c.ExtractFromListedCluster(cls); ok {
		log.Println("GKE cluster already exist. no need to create")
		return nil
	}
	log.Println("GKE cluster does not exists. trying to create...")
	if c.CreateCluster(); err != nil {
		return fmt.Errorf("failed to CreateCluster: %#v", err)
	}
	return nil
}

// DeleteCluster delete gke cluster.
func (c Cluster) DeleteCluster() error {
	log.Printf("trying to delete gke cluster: %s...", c.ClusterName)
	cmd := c.deleteClusterCommand()
	res, err := command.ExecAndWait(cmd)
	if err != nil { // 削除完了を待つ
		return fmt.Errorf("failed to Exec: %v, cmd: %s", err, cmd)
	}
	log.Printf("DeleteCluster result: %#v", res)
	return nil
}

func (c Cluster) deleteClusterCommand() string {
	return fmt.Sprintf("gcloud --quiet container clusters delete %s", c.ClusterName)
}

// DeleteClusterIfExist delete gke cluster if cluster exist.
func (c Cluster) DeleteClusterIfExist() error {
	// Clusterが存在するかどうか確認
	cls, err := c.ListCluster()
	if err != nil {
		return fmt.Errorf("failed to ListCluster: %w", err)
	}

	// GKEクラスタがあるときは削除する
	if _, ok := c.ExtractFromListedCluster(cls); !ok {
		log.Println("GKE cluster does not exist. no need to delete")
		return nil
	}
	log.Println("GKE cluster exists. trying to delete...")
	if c.DeleteCluster(); err != nil {
		return fmt.Errorf("failed to DeleteCluster: %#v", err)
	}
	return nil
}

// ListCluster lists all gke clusters.
func (c Cluster) ListCluster() ([]ListedCluster, error) {
	cmd := fmt.Sprintf("gcloud container clusters list")
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	// listの結果が空ならすぐ返す
	if res.Stdout == "" {
		return nil, nil
	}

	cls, err := formatlistedCluster(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to formatlistedCluster: %v", err)
	}
	return cls, nil
}

// ListedCluster is formated result for 'gcloud container clusters list'.
type ListedCluster struct {
	Name          string // NAME
	Location      string // LOCATION
	MasterVersion string // MASTER_VERSION
	MasterIP      string // MASTER_IP
	MachineType   string // MACHINE_TYPE
	NodeVersion   string // NODE_VERSION
	NumNodes      string // NUM_NODES
	Status        string // STATUS
}

// ExtractFromListedCluster extract cluster from ListedCluster.
func (c Cluster) ExtractFromListedCluster(lcs []ListedCluster) (ListedCluster, bool) {
	// ListedClusterから該当のClusterを取得する。取得できなかったらfalse
	for _, lc := range lcs {
		// cluster名が一致したらok
		if lc.Name == c.ClusterName {
			return lc, true
		}
	}
	// 見つからなかったときはfalseを返す
	return ListedCluster{}, false
}

func formatlistedCluster(s string) ([]ListedCluster, error) {
	var listed []ListedCluster

	lines := strings.Split(s, "\n") // 改行区切りでlinesに格納
	for i, l := range lines {
		col := strings.Fields(l)
		if i == 0 {
			// １行目が想定するフォーマットでなければエラー
			if (col[0] != "NAME") || (col[1] != "LOCATION") || (col[2] != "MASTER_VERSION") || (col[3] != "MASTER_IP") || (col[4] != "MACHINE_TYPE") || (col[5] != "NODE_VERSION") || (col[6] != "NUM_NODES") || (col[7] != "STATUS") {
				return nil, fmt.Errorf("format error.\n got '%v'\nexpected format 'NAME LOCATION MASTER_VERSION MASTER_IP MACHINE_TYPE NODE_VERSION NUM_NODES STATUS'", col)
			}
		} else {
			c := ListedCluster{
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

// EnsureClusterStatusRunning confirms cluster status RUNNING.
func (c Cluster) EnsureClusterStatusRunning() error {
	if err := retry.Retry(30, 20*time.Second, func() error {
		lcs, err := c.ListCluster()
		if err != nil {
			return fmt.Errorf("failed to ListCluster: %v", err)
		}
		lc, ok := c.ExtractFromListedCluster(lcs)
		if !ok { // Clusterがなければエラー
			return fmt.Errorf("failed to extractFromListedCluster: %v", err)
		}
		if lc.Status != "RUNNING" { // ClusterがRUNNINGでなければエラー
			return fmt.Errorf("not RUNNING. current status: %s", lc.Status)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to confirm gke cluster status: %v", err)
	}
	return nil
}

// GetCredentials get credentials for gke cluster.
func (c Cluster) GetCredentials() error {
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

// KustomizeBuildAndDeploy deploys gke.
func KustomizeBuildAndDeploy(path string) error {
	// path: ex. "./k8s/overlays/dev/"
	cmd := fmt.Sprintf("./kustomize build %s | /usr/bin/kubectl apply -f -", path)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}
