package gke

import (
	"reflect"
	"testing"
)

func TestNewCluster(t *testing.T) {
	tests := map[string]struct {
		clusterName   string
		computeZone   string
		machineType   string
		diskSize      int
		numNodes      int
		preemptible   string
		wantCluster   *Cluster
		wantErr       bool
		wantCreateCmd string
	}{
		"default": {
			clusterName: "cluster1",
			computeZone: "zone1",
			wantCluster: &Cluster{
				ClusterName: "cluster1",
				ComputeZone: "zone1",
				MachineType: "g1-small",
				DiskSize:    10,
				NumNodes:    4,
				Preemptible: "preemptible",
			},
			wantErr:       false,
			wantCreateCmd: `gcloud --quiet container clusters create cluster1 --zone zone1 --machine-type=g1-small --disk-size 10 --num-nodes=4 --preemptible`,
		},
		"error_no_cluster": {
			computeZone: "zone1",
			wantCluster: &Cluster{
				ClusterName: "cluster1",
				ComputeZone: "zone1",
				MachineType: "g1-small",
				DiskSize:    10,
				NumNodes:    4,
				Preemptible: "preemptible",
			},
			wantErr: true,
		},
		"error_no_zone": {
			clusterName: "cluster1",
			wantCluster: &Cluster{
				ClusterName: "cluster1",
				ComputeZone: "zone1",
				MachineType: "g1-small",
				DiskSize:    10,
				NumNodes:    4,
				Preemptible: "preemptible",
			},
			wantErr: true,
		},
		"custom": {
			clusterName: "cluster1",
			computeZone: "zone1",
			machineType: "type1",
			diskSize:    20,
			numNodes:    5,
			preemptible: "off",
			wantCluster: &Cluster{
				ClusterName: "cluster1",
				ComputeZone: "zone1",
				MachineType: "type1",
				DiskSize:    20,
				NumNodes:    5,
				Preemptible: "off",
			},
			wantErr:       false,
			wantCreateCmd: `gcloud --quiet container clusters create cluster1 --zone zone1 --machine-type=type1 --disk-size 20 --num-nodes=5 `,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cl, err := NewCluster(tc.clusterName, tc.computeZone, tc.machineType, tc.diskSize, tc.numNodes, tc.preemptible)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("gotErr: %v, wantErr: %v", err, tc.wantErr)
				}
				return
			}
			if !reflect.DeepEqual(cl, tc.wantCluster) {
				t.Errorf("got cluster: %v, want: %v", cl, tc.wantCluster)
			}
			cmd := cl.createClusterCommand()
			if cmd != tc.wantCreateCmd {
				t.Errorf("got cmd: %v, want: %v", cmd, tc.wantCreateCmd)
			}
		})
	}
}

func TestFormatListedCluster(t *testing.T) {
	cases := []struct {
		name    string
		listed  string
		want    ListedCluster
		wantErr string
	}{
		{
			name: "match_format",
			listed: `NAME                             LOCATION    MASTER_VERSION  MASTER_IP       MACHINE_TYPE  NODE_VERSION    NUM_NODES  STATUS
			gke-stockprice-integration-test  us-west1-c  1.13.11-gke.14  35.230.100.164  g1-small      1.13.11-gke.14  2          RUNNING`,
			want: ListedCluster{
				Name:          "gke-stockprice-integration-test",
				Location:      "us-west1-c",
				MasterVersion: "1.13.11-gke.14",
				MasterIP:      "35.230.100.164",
				MachineType:   "g1-small",
				NodeVersion:   "1.13.11-gke.14",
				NumNodes:      "2",
				Status:        "RUNNING",
			},
			wantErr: "",
		},
		{
			name: "non_match_format",
			listed: `NAME2                             LOCATION    MASTER_VERSION  MASTER_IP       MACHINE_TYPE  NODE_VERSION    NUM_NODES  STATUS
			gke-stockprice-integration-test  us-west1-c  1.13.11-gke.14  35.230.100.164  g1-small      1.13.11-gke.14  2          RUNNING`,
			wantErr: "format error.\n got '[NAME2 LOCATION MASTER_VERSION MASTER_IP MACHINE_TYPE NODE_VERSION NUM_NODES STATUS]'\nexpected format 'NAME LOCATION MASTER_VERSION MASTER_IP MACHINE_TYPE NODE_VERSION NUM_NODES STATUS'",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatlistedCluster(tt.listed)
			if err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("got error %s want error %s", err.Error(), tt.wantErr)
				}
			}
			if reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
