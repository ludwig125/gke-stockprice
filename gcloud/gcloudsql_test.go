// +build integration

package gcloud

import (
	"testing"
)

func TestCreateDeleteInstance(t *testing.T) {
	cases := []struct {
		name       string
		instance   CloudSQLInstance
		wantStdout string
		wantErr    string
	}{
		{
			name: "match_condition",
			instance: CloudSQLInstance{
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
			instance: CloudSQLInstance{
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
			if err := i.CreateInstance(); err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("got error: %v want error: %s", err, tt.wantErr)
				}
			}
			// if got.Stdout != tt.wantStdout {
			// 	t.Errorf("got stdout: %s want stdout: %s", got.Stdout, tt.wantStdout)
			// }

			if err := i.DeleteInstance(); err != nil {
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

func TestListInstance(t *testing.T) {
	cases := []struct {
		name       string
		instance   CloudSQLInstance
		wantStdout string
		wantErr    bool
	}{
		{
			name: "match_projectid",
			instance: CloudSQLInstance{
				Project: "gke-stockprice",
				//Instance: "gke-stockprice-integration-test-202003240621",
				Instance: "gke-stockprice-integration-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
				ExecCmd:  false,
			},
			wantErr: false,
		},
		{
			name: "not_match_projectid",
			instance: CloudSQLInstance{
				Project:  "gke-stockprice-test",
				Instance: "gke-stockprice-integration2-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
				ExecCmd:  false,
			},
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instance
			// if _, err := i.ListInstance(); (err != nil) != tt.wantErr {
			// 	t.Errorf("error: %v, wantErr: %v", err, tt.wantErr)
			// 	return
			// }
			res, err := i.ListInstance()
			if (err != nil) != tt.wantErr {
				t.Errorf("error: %v, wantErr: %v", err, tt.wantErr)
				return
			}
			t.Log(res)
		})
	}
}

func TestExistCloudSQLInstance(t *testing.T) {
	i := CloudSQLInstance{
		Project:  "gke-stockprice-test",
		Instance: "gke-stockprice-integration-test-202003240621",
		Tier:     "db-f1-micro",
		Region:   "us-central1",
		ExecCmd:  false,
	}
	ok, err := i.ExistCloudSQLInstance()
	wantErr := false
	if (err != nil) != wantErr {
		t.Errorf("error: %v, wantErr: %v", err, wantErr)
		return
	}
	t.Log(ok)
}
