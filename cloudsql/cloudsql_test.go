// +build integration

package cloudsql

import (
	"reflect"
	"testing"
)

func TestNewCloudSQLInstance(t *testing.T) {
	tests := map[string]struct {
		instanceName  string
		region        string
		tier          string
		databaseName  string
		wantInstance  *CloudSQLInstance
		wantErr       string
		wantCreateCmd string
	}{
		"ok": {
			instanceName: "instance1-integration-test",
			region:       "region1",
			tier:         "db-f1-micro",
			databaseName: "database_dev",
			wantInstance: &CloudSQLInstance{
				Instance: "instance1-integration-test",
				Region:   "region1",
				Tier:     "db-f1-micro",
				Database: "database_dev",
			},
			wantErr:       "",
			wantCreateCmd: `gcloud sql instances create instance1-integration-test --tier=db-f1-micro --region=region1 --storage-auto-increase --no-backup`,
		},
		"error_no_instance": {
			// instanceName: "instance1-integration-test",
			region:       "region1",
			tier:         "db-f1-micro",
			databaseName: "database_dev",
			wantErr:      "instanceName is empty",
		},
		"error_no_region": {
			instanceName: "instance1-integration-test",
			// region:       "region1",
			tier:         "db-f1-micro",
			databaseName: "database_dev",
			wantErr:      "region is empty",
		},
		"error_no_tier": {
			instanceName: "instance1-integration-test",
			region:       "region1",
			// tier:         "db-f1-micro",
			databaseName: "database_dev",
			wantErr:      "tier is empty",
		},
		"error_no_database": {
			instanceName: "instance1-integration-test",
			region:       "region1",
			tier:         "db-f1-micro",
			// databaseName: "database_dev",
			wantErr: "databaseName is empty",
		},
		"error_instance_not_contain_integration-test": {
			instanceName: "instance1-integration2-test",
			region:       "region1",
			tier:         "db-f1-micro",
			databaseName: "database_dev",
			wantErr:      "instance name should contains 'integration-test'. instance: instance1-integration2-test",
		},
		"error_instance_contain_prod": {
			instanceName: "instance1-integration-test-prod",
			region:       "region1",
			tier:         "db-f1-micro",
			databaseName: "database_dev",
			wantErr:      "instance name should not contains 'prod'. instance: instance1-integration-test-prod",
		},
		"error_database_not_contain_dev": {
			instanceName: "instance1-integration-test",
			region:       "region1",
			tier:         "db-f1-micro",
			databaseName: "database_",
			wantErr:      "databaseName name should contains 'dev'. databaseName: database_",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			i, err := NewCloudSQLInstance(tc.instanceName, tc.region, tc.tier, tc.databaseName)
			if err != nil {
				t.Logf("error: %v", err)
				if tc.wantErr == "" {
					t.Fatal("error occured unexpectedly", err)
				}
				if err.Error() != tc.wantErr {
					t.Errorf("gotErr: %s, wantErr: %s", err, tc.wantErr)
				}
				return
			}
			if !reflect.DeepEqual(i, tc.wantInstance) {
				t.Errorf("got cluster: %v, want: %v", i, tc.wantInstance)
			}
			cmd := i.createInstanceCommand()
			if cmd != tc.wantCreateCmd {
				t.Errorf("got cmd: %v, want: %v", cmd, tc.wantCreateCmd)
			}
		})
	}
}

func TestDeleteInstanceCommand(t *testing.T) {
	tests := map[string]struct {
		instance      *CloudSQLInstance
		wantErr       string
		wantDeleteCmd string
	}{
		"ok": {
			instance: &CloudSQLInstance{
				Instance: "instance1-integration-test",
				Region:   "region1",
				Tier:     "db-f1-micro",
				Database: "database_dev",
			},
			wantErr:       "",
			wantDeleteCmd: `gcloud sql instances delete instance1-integration-test`,
		},
		"error_instance_not_contain_integration-test": {
			instance: &CloudSQLInstance{
				Instance: "instance1-integration2-test",
				Region:   "region1",
				Tier:     "db-f1-micro",
				Database: "database_dev",
			},
			wantErr: "instance name should contains 'integration-test'. instance: instance1-integration2-test",
		},
		"error_instance_contain_prod": {
			instance: &CloudSQLInstance{
				Instance: "instance1-integration-test-prod",
				Region:   "region1",
				Tier:     "db-f1-micro",
				Database: "database_dev",
			},
			wantErr: "instance name should not contains 'prod'. instance: instance1-integration-test-prod",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			i := tc.instance
			cmd, err := i.deleteInstanceCommand()
			if err != nil {
				t.Logf("error: %v", err)
				if tc.wantErr == "" {
					t.Fatal("error occured unexpectedly", err)
				}
				if err.Error() != tc.wantErr {
					t.Errorf("gotErr: %s, wantErr: %s", err, tc.wantErr)
				}
				return
			}
			if cmd != tc.wantDeleteCmd {
				t.Errorf("got cmd: %v, want: %v", cmd, tc.wantDeleteCmd)
			}
		})
	}
}

func TestDescribeInstance(t *testing.T) {
	cases := []struct {
		name       string
		instance   CloudSQLInstance
		wantStdout string
		wantErr    bool
	}{
		{
			name: "match_projectid",
			instance: CloudSQLInstance{
				//Instance: "gke-stockprice-integration-test-202003240621",
				Instance: "gke-stockprice-integration-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
			},
			wantErr: false,
		},
		{
			name: "not_match_projectid",
			instance: CloudSQLInstance{
				Instance: "gke-stockprice-integration2-test",
				Tier:     "db-f1-micro",
				Region:   "us-central1",
			},
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instance
			res, err := i.DescribeInstance()
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
		Instance: "gke-stockprice-integration-test-202003240621",
		Tier:     "db-f1-micro",
		Region:   "us-central1",
	}
	ok, err := i.ExistCloudSQLInstance()
	wantErr := false
	if (err != nil) != wantErr {
		t.Errorf("error: %v, wantErr: %v", err, wantErr)
		return
	}
	t.Log(ok)
}
