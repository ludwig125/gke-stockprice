// +build integration

package main

import (
	"reflect"
	"testing"
)

func TestListCluster(t *testing.T) {
	cases := []struct {
		name       string
		cluster    gkeCluster
		wantStdout string
		wantErr    string
	}{
		{
			name: "match_projectid",
			cluster: gkeCluster{
				Project:     "gke-stockprice",
				ClusterName: "gke-stockprice-integration-test",
				ComputeZone: "us-west1-c",
				MachineType: "g1-small",
				ExecCmd:     true,
			},
			wantErr: "",
		},
		// {
		// 	name: "not_match_projectid",
		// 	instance: cloudSQLInstance{
		// 		Project:  "gke-stockprice-test",
		// 		Instance: "gke-stockprice-integration2-test",
		// 		Tier:     "db-f1-micro",
		// 		Region:   "us-central1",
		// 		ExecCmd:  false,
		// 	},
		// 	wantErr: "failed to Pages: googleapi: Error 400: Project specified in the request is invalid., errorInvalidProject",
		// },
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.cluster
			if _, err := c.listCluster(); err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("got error: %v want error: %s", err, tt.wantErr)
				}
			}
		})
	}
}

func TestFormatListedCluster(t *testing.T) {
	cases := []struct {
		name    string
		listed  string
		want    gkeClusterListed
		wantErr string
	}{
		{
			name: "match_format",
			listed: `NAME                             LOCATION    MASTER_VERSION  MASTER_IP       MACHINE_TYPE  NODE_VERSION    NUM_NODES  STATUS
			gke-stockprice-integration-test  us-west1-c  1.13.11-gke.14  35.230.100.164  g1-small      1.13.11-gke.14  2          RUNNING`,
			want: gkeClusterListed{
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
