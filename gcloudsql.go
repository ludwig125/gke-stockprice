// +build integration

package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/ludwig125/gke-stockprice/command"
)

type cloudSQLInstance struct {
	Project      string
	Instance     string
	Tier         string
	Region       string
	DatabaseName string
	ExecCmd      bool
}

func (i cloudSQLInstance) createInstance() error {
	// PROJECT_NAME=gke-stockprice
	// gcloud config set project $PROJECT_NAME

	// DB_TIER=db-f1-micro
	// DB_REGION=us-central1
	// TIME=`date +"%Y%m%d%H%M"`
	// DB_NAME=$PROJECT_NAME-$DB_REGION-$DB_TIER-$TIME

	if !strings.Contains(i.Instance, "integration-test") {
		return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !i.ExecCmd {
		log.Println("satisfied the condition")
		return nil
	}

	cmd := fmt.Sprintf("gcloud sql instances create %s --tier=%s --region=%s --storage-auto-increase --no-backup", i.Instance, i.Tier, i.Region)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (i cloudSQLInstance) deleteInstance() error {
	if !strings.Contains(i.Instance, "integration-test") {
		return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !i.ExecCmd {
		log.Println("satisfied the condition")
		return nil
	}

	cmd := fmt.Sprintf("gcloud sql instances delete %s", i.Instance)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (i cloudSQLInstance) listInstance() (*sqladmin.DatabaseInstance, error) {
	// 参考
	// list API: https://cloud.google.com/sql/docs/mysql/admin-api/v1beta4/operations/list?hl=ja
	// 取れる情報: https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#DatabaseInstance
	// APIのgithub: https://github.com/googleapis/google-api-go-client/blob/master/sqladmin/v1beta4/sqladmin-gen.go
	// Stateの意味: https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#SqlInstanceState
	// SQL_INSTANCE_STATE_UNSPECIFIED
	//   The state of the instance is unknown.
	// RUNNABLE
	//   The instance is running.
	// SUSPENDED
	//   The instance is currently offline, but it may run again in the future.
	// PENDING_DELETE
	//   The instance is being deleted.
	// PENDING_CREATE
	//   The instance is being created.
	// MAINTENANCE
	//   The instance is down for maintenance.
	// FAILED
	//   The instance failed to be created.

	ctx := context.Background()
	cl, err := google.DefaultClient(ctx, sqladmin.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to get google.DefaultClient: %v", err)
	}

	sqladminService, err := sqladmin.New(cl)
	if err != nil {
		return nil, fmt.Errorf("failed to sqladmin.New: %w", err)
	}

	// Project ID of the project for which to list Cloud SQL instances.
	project := i.Project

	req := sqladminService.Instances.List(project)
	var instance *sqladmin.DatabaseInstance
	if err := req.Pages(ctx, func(page *sqladmin.InstancesListResponse) error {
		for _, databaseInstance := range page.Items {
			if databaseInstance.Name == i.Instance {
				if !strings.Contains(i.Instance, "integration-test") {
					return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
				}
				fmt.Printf("NAME:             %s\n", databaseInstance.Name)
				fmt.Printf("DATABASE_VERSION: %s\n", databaseInstance.DatabaseVersion)
				fmt.Printf("LOCATION:         %s\n", databaseInstance.GceZone)
				fmt.Printf("TIER:             %s\n", databaseInstance.Settings.Tier)
				fmt.Printf("STATE:            %s\n", databaseInstance.State)
				fmt.Printf("CONNECTION_NAME:  %s\n", databaseInstance.ConnectionName)

				// For debug
				// fmt.Printf("\n\n%#v\n", *databaseInstance)

				instance = databaseInstance
				return nil
			}
		}
		fmt.Println("no match instance:", i.Instance)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to Pages: %w", err)
	}
	return instance, nil
}

func (i cloudSQLInstance) confirmcloudSQLInstanceStatus(wantStatus string) error {
	if err := retry(30, 20*time.Second, func() error {
		instance, err := i.listInstance()
		if err != nil {
			return fmt.Errorf("failed to listInstance: %w", err)
		}
		if instance.State != wantStatus {
			return fmt.Errorf("not matched. current: %s, expected: %s", instance.State, wantStatus)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to confirm cloud sql: %w", err)
	}
	return nil
}

func (i cloudSQLInstance) createTestDatabase() error {
	if !strings.Contains(i.Instance, "integration-test") {
		return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !i.ExecCmd {
		log.Println("satisfied the condition")
		return nil
	}

	cmd := fmt.Sprintf("gcloud sql databases create %s --instance=%s", i.DatabaseName, i.Instance)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (i cloudSQLInstance) findDatabase() error {
	if !strings.Contains(i.Instance, "integration-test") {
		return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}
	// コマンドは実行せず条件を満たすかどうかだけ返す
	if !i.ExecCmd {
		log.Println("satisfied the condition")
		return nil
	}

	cmd := fmt.Sprintf("gcloud sql databases list --instance=%s", i.Instance)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	//fmt.Println(res.Stdout)

	if err := findDatabaseName(res.Stdout, i.DatabaseName); err != nil {
		return fmt.Errorf("failed to find test database name. list: %s", res.Stdout)
	}
	return nil
}

func findDatabaseName(s, dbName string) error {
	lines := strings.Split(s, "\n") // 改行区切りでlinesに格納
	for _, l := range lines {
		dbNames := strings.Fields(l)
		if dbNames[0] == dbName {
			return nil
		}
	}
	return fmt.Errorf("no match: %s", dbName)
}
