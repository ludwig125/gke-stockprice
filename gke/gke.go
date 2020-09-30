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

// NewCluster Cluster constructor.
func NewCluster(clusterName, computeZone, machineType string, diskSize, numNodes int, preemptible string) (*Cluster, error) {
	if clusterName == "" {
		return nil, errors.New("clusterName is empty")
	}
	if computeZone == "" {
		return nil, errors.New("computeZone is empty")
	}
	if machineType == "" {
		machineType = "g1-small"
	}
	if diskSize == 0 {
		diskSize = 10
	}
	if numNodes == 0 {
		numNodes = 4
	}
	if preemptible != "off" {
		preemptible = "preemptible"
	}

	return &Cluster{
		ClusterName: clusterName,
		ComputeZone: computeZone,
		MachineType: machineType,
		DiskSize:    diskSize,
		NumNodes:    numNodes,
		Preemptible: preemptible,
	}, nil
}

// CreateCluster creates gke cluster.
func (c Cluster) CreateCluster() error {
	cmd := c.createClusterCommand()
	if _, err := command.Exec(cmd); err != nil {
		return fmt.Errorf("failed to Exec: %v, cmd: %s", err, cmd)
	}
	return nil
}

func (c Cluster) createClusterCommand() string {
	preemptible := ""
	if c.Preemptible == "preemptible" {
		preemptible = "--preemptible"
	}
	return fmt.Sprintf(`gcloud --quiet container clusters create %s --zone %s --machine-type=%s --disk-size %d --num-nodes=%d %s`, c.ClusterName, c.ComputeZone, c.MachineType, c.DiskSize, c.NumNodes, preemptible)
}

// CreateClusterIfNotExist creates gke cluster if cluster does not exist.
func (c Cluster) CreateClusterIfNotExist() error {
	// すでにClusterが存在するかどうか確認
	cls, err := c.ListCluster()
	if err != nil {
		return fmt.Errorf("failed to ListCluster: %w", err)
	}

	// GKEクラスタがないときは作成する
	_, ok := c.extractFromListedCluster(cls)
	if !ok {
		log.Println("GKE cluster does not exists. trying to create...")
		if c.CreateCluster(); err != nil {
			return fmt.Errorf("failed to CreateCluster: %#v", err)
		}
	}
	return nil
}

// DeleteCluster delete gke cluster.
func (c Cluster) DeleteCluster() error {
	cmd := c.deleteClusterCommand()
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (c Cluster) deleteClusterCommand() string {
	return fmt.Sprintf("gcloud --quiet container clusters delete %s", c.ClusterName)
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

// ListedClusterから該当のClusterを取得する。取得できなかったらfalse
func (c Cluster) extractFromListedCluster(lcs []ListedCluster) (ListedCluster, bool) {
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
			return fmt.Errorf("failed to ListCluster: %w", err)
		}
		lc, ok := c.extractFromListedCluster(lcs)
		if !ok { // Clusterがなければエラー
			return fmt.Errorf("failed to extractFromListedCluster: %w", err)
		}
		if lc.Status != "RUNNING" { // ClusterがRUNNINGでなければエラー
			return fmt.Errorf("not RUNNING. current status: %s", lc.Status)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to confirm gke cluster status: %w", err)
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

// GKEDeploy deploys gke.
func GKEDeploy(path string) error {
	// path: ex. "./k8s/overlays/dev/"
	cmd := fmt.Sprintf("./kustomize build %s | /usr/bin/kubectl apply -f -", path)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}
