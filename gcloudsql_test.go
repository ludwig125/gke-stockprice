// +build integration

package main

import (
	"testing"
)

func TestCreateDeletecloudSQLInstance(t *testing.T) {
	cases := []struct {
		name       string
		instance   cloudSQLInstance
		wantStdout string
		wantErr    string
	}{
		{
			name: "match_condition",
			instance: cloudSQLInstance{
				Project:  "gke-stockprice",
				Instance: "gke-stockprice-integration-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
				ExecCmd:  false, // 実際には作成削除しない
			},
			//wantStdout: "satisfied the condition",
			wantErr: "",
		},
		{
			name: "not_match_condition",
			instance: cloudSQLInstance{
				Project:  "gke-stockprice",
				Instance: "gke-stockprice-integration2-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
				ExecCmd:  false, // 実際には作成削除しない
			},
			//wantStdout: "",
			wantErr: "instance name should contains 'integration-test'. instance: gke-stockprice-integration2-test",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instance
			if err := i.createInstance(); err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("got error: %v want error: %s", err, tt.wantErr)
				}
			}
			// if got.Stdout != tt.wantStdout {
			// 	t.Errorf("got stdout: %s want stdout: %s", got.Stdout, tt.wantStdout)
			// }

			if err := i.deleteInstance(); err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("got error: %v want error: %s", err, tt.wantErr)
				}
			}
			// if got.Stdout != tt.wantStdout {
			// 	t.Errorf("got stdout: %s want stdout: %s", gotStdout, tt.wantStdout)
			// }
		})
	}
}

func TestListcloudSQLInstance(t *testing.T) {
	cases := []struct {
		name       string
		instance   cloudSQLInstance
		wantStdout string
		wantErr    string
	}{
		{
			name: "match_projectid",
			instance: cloudSQLInstance{
				Project:  "gke-stockprice",
				Instance: "gke-stockprice-integration-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
				ExecCmd:  false,
			},
			wantErr: "failed to Pages: failed to find instance: gke-stockprice-integration-test",
		},
		{
			name: "not_match_projectid",
			instance: cloudSQLInstance{
				Project:  "gke-stockprice-test",
				Instance: "gke-stockprice-integration2-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
				ExecCmd:  false,
			},
			wantErr: "failed to Pages: googleapi: Error 400: Project specified in the request is invalid., errorInvalidProject",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instance
			if _, err := i.listInstance(); err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("got error: %v want error: %s", err, tt.wantErr)
				}
			}
		})
	}
}
