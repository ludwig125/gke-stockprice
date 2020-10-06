// +build integration

package cloudsql

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ludwig125/gke-stockprice/command"
	"github.com/ludwig125/gke-stockprice/retry"
)

// CloudSQLInstance has cloudsql instance information.
type CloudSQLInstance struct {
	Instance string
	Region   string
	Tier     string
	Database string
}

// NewCloudSQLInstance CloudSQLInstance constructor.
func NewCloudSQLInstance(instanceName, region, tier, databaseName string) (*CloudSQLInstance, error) {
	if instanceName == "" {
		return nil, errors.New("instanceName is empty")
	}
	if region == "" {
		return nil, errors.New("region is empty")
	}
	if tier == "" {
		return nil, errors.New("tier is empty")
	}
	if databaseName == "" {
		return nil, errors.New("databaseName is empty")
	}

	// 万が一にも本番インスタンスを消してはいけないので冗長なチェックを入れている
	if !strings.Contains(instanceName, "integration-test") {
		return nil, fmt.Errorf("instance name should contains 'integration-test'. instance: %s", instanceName)
	}
	if strings.Contains(instanceName, "prod") {
		return nil, fmt.Errorf("instance name should not contains 'prod'. instance: %s", instanceName)
	}
	if !strings.Contains(databaseName, "dev") {
		return nil, fmt.Errorf("databaseName name should contains 'dev'. databaseName: %s", databaseName)
	}

	return &CloudSQLInstance{
		Instance: instanceName,
		Region:   region,
		Tier:     tier,
		Database: databaseName,
	}, nil
}

func (i CloudSQLInstance) CreateInstance() error {
	cmd := i.createInstanceCommand()
	res, err := command.ExecAndWait(cmd) // コマンドの完了を待つ
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s,res: %#v", err, cmd, res)
	}
	log.Printf("CreateInstance result: %#v", res)
	return nil
}

func (i CloudSQLInstance) createInstanceCommand() string {
	// 作るかどうかy/n の入力を待たないように-quietオプションつけている
	return fmt.Sprintf("gcloud --quiet sql instances create %s --tier=%s --region=%s --storage-auto-increase --no-backup", i.Instance, i.Tier, i.Region)
}

func (i CloudSQLInstance) CreateInstanceIfNotExist() error {
	// すでにSQLInstanceが存在するかどうか確認
	ok, err := i.ExistCloudSQLInstance()
	if err != nil {
		return fmt.Errorf("failed to ExistCloudSQLInstance: %#v", err)
	}
	if ok {
		log.Println("SQL Instance already exists. no need to create")
		return nil
	}

	// SQLInstanceがないなら作る
	log.Println("SQL Instance does not exists. trying to create...")
	if err := i.CreateInstance(); err != nil {
		return fmt.Errorf("failed to CreateInstance: %#v", err)
	}
	return nil
}

func (i CloudSQLInstance) DeleteInstance() error {
	log.Printf("trying to delete sql instance: %s...", i.Instance)
	cmd, err := i.deleteInstanceCommand()
	if err != nil {
		return fmt.Errorf("failed to deleteInstanceCommand: %v", err)
	}
	res, err := command.ExecAndWait(cmd) // 削除完了を待つ
	if err != nil {
		return fmt.Errorf("failed to Exec: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	log.Printf("DeleteInstance result: %#v", res)
	return nil
}

func (i CloudSQLInstance) deleteInstanceCommand() (string, error) {
	if !strings.Contains(i.Instance, "integration-test") {
		return "", fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}
	if strings.Contains(i.Instance, "prod") {
		return "", fmt.Errorf("instance name should not contains 'prod'. instance: %s", i.Instance)
	}
	return fmt.Sprintf("gcloud --quiet sql instances delete %s", i.Instance), nil
}

func (i CloudSQLInstance) DeleteInstanceIfExist() error {
	// すでにSQLInstanceが存在するかどうか確認
	ok, err := i.ExistCloudSQLInstance()
	if err != nil {
		return fmt.Errorf("failed to ExistCloudSQLInstance: %#v", err)
	}
	if !ok {
		log.Println("SQL Instance does not exist. no need to delete")
		return nil
	}

	// SQLInstanceがあるなら削除する
	log.Println("SQL Instance exists. trying to delete...")
	if err := i.DeleteInstance(); err != nil {
		return fmt.Errorf("failed to DeleteInstance: %#v", err)
	}
	return nil
}

// このAPIはサービスアカウントでは使えない
// 以下のエラーが出る
// failed to DescribeInstance: &errors.errorString{s:"failed to get google.DefaultClient: google: could not find default credentials. See https://developers.google.com/accounts/docs/application-default-credentials for more information."}
// func (i CloudSQLInstance) DescribeInstance() (*sqladmin.DatabaseInstance, error) {
// 	// 参考
// 	// list API: https://cloud.google.com/sql/docs/mysql/admin-api/v1beta4/operations/list?hl=ja
// 	// 取れる情報: https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#DatabaseInstance
// 	// APIのgithub: https://github.com/googleapis/google-api-go-client/blob/master/sqladmin/v1beta4/sqladmin-gen.go
// 	// Stateの意味: https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#SqlInstanceState
// 	// SQL_INSTANCE_STATE_UNSPECIFIED
// 	//   The state of the instance is unknown.
// 	// RUNNABLE
// 	//   The instance is running.
// 	// SUSPENDED
// 	//   The instance is currently offline, but it may run again in the future.
// 	// PENDING_DELETE
// 	//   The instance is being deleted.
// 	// PENDING_CREATE
// 	//   The instance is being created.
// 	// MAINTENANCE
// 	//   The instance is down for maintenance.
// 	// FAILED
// 	//   The instance failed to be created.

// 	ctx := context.Background()
// 	cl, err := google.DefaultClient(ctx, sqladmin.CloudPlatformScope)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get google.DefaultClient: %v", err)
// 	}

// 	sqladminService, err := sqladmin.New(cl)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to sqladmin.New: %w", err)
// 	}

// 	// Project ID of the project for which to list Cloud SQL instances.
// 	project := i.Project

// 	req := sqladminService.Instances.List(project)
// 	var instance *sqladmin.DatabaseInstance
// 	if err := req.Pages(ctx, func(page *sqladmin.InstancesListResponse) error {
// 		for _, databaseInstance := range page.Items {
// 			if databaseInstance.Name == i.Instance {
// 				if !strings.Contains(i.Instance, "integration-test") {
// 					return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
// 				}
// 				fmt.Printf("NAME:             %s\n", databaseInstance.Name)
// 				fmt.Printf("DATABASE_VERSION: %s\n", databaseInstance.DatabaseVersion)
// 				fmt.Printf("LOCATION:         %s\n", databaseInstance.GceZone)
// 				fmt.Printf("TIER:             %s\n", databaseInstance.Settings.Tier)
// 				fmt.Printf("STATE:            %s\n", databaseInstance.State)
// 				fmt.Printf("CONNECTION_NAME:  %s\n", databaseInstance.ConnectionName)

// 				// For debug
// 				fmt.Printf("\n\n%#v\n", *databaseInstance)

// 				instance = databaseInstance
// 				return nil
// 			}
// 		}
// 		fmt.Println("no match instance:", i.Instance)
// 		return nil
// 	}); err != nil {
// 		return nil, fmt.Errorf("failed to Pages: %w", err)
// 	}
// 	return instance, nil
// }

type CloudSQLDatabaseInstance struct {
	Name            string
	DatabaseVersion string
	Location        string
	Tier            string
	State           string
	ConnectionName  string
}

type column string

func (c *column) find(line, target string) {
	if string(*c) != "" {
		//log.Println("s is already exist", *c, " target:", target, " line", line)
		return
	}
	if strings.Contains(line, target) {
		l := strings.SplitAfterN(line, ":", 2)
		// "name: abc" -> abcを抽出
		*c = column(strings.Trim(l[1], " "))
	}
}

func (i CloudSQLInstance) DescribeInstance() (*CloudSQLDatabaseInstance, error) {
	if !strings.Contains(i.Instance, "integration-test") {
		return nil, fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}

	ok, err := i.ExistCloudSQLInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to ExistCloudSQLInstance: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("no match instance: %v", i.Instance)
	}
	log.Printf("found instance: %s successfully", i.Instance)

	log.SetOutput(ioutil.Discard)  // 鍵情報などを出したくないので/dev/nullに出力
	defer log.SetOutput(os.Stdout) // 出力先を戻す

	res, err := doDescribeInstance(i.Instance)
	if err != nil {
		return nil, fmt.Errorf("failed to doDescribeInstance: %v", err)
	}
	var name, version, loc, tier, state, connectionName column

	for _, l := range strings.Split(res, "\n") {
		name.find(l, "name:")
		version.find(l, "databaseVersion:")
		loc.find(l, "gceZone:")
		tier.find(l, "tier:")
		state.find(l, "state:")
		connectionName.find(l, "connectionName:")
	}
	fmt.Println("NAME:             ", name)
	fmt.Println("DATABASE_VERSION: ", version)
	fmt.Println("LOCATION:         ", loc)
	fmt.Println("TIER:             ", tier)
	fmt.Println("STATE:            ", state)
	fmt.Println("CONNECTION_NAME:  ", connectionName)

	return &CloudSQLDatabaseInstance{
		Name:            string(name),
		DatabaseVersion: string(version),
		Location:        string(loc),
		Tier:            string(tier),
		State:           string(state),
		ConnectionName:  string(connectionName),
	}, nil
}

var doDescribeInstance = func(instance string) (string, error) {
	cmd := "gcloud sql instances describe " + instance
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return res.Stdout, nil
}

// ConnectionName returns sql instance connection name.
func (i CloudSQLInstance) ConnectionName() (string, error) {
	ist, err := i.DescribeInstance()
	if err != nil {
		return "", fmt.Errorf("failed to DescribeInstance: %#v", err)
	}
	return ist.ConnectionName, nil
}

func (i CloudSQLInstance) ExistCloudSQLInstance() (bool, error) {
	cmd := "gcloud sql instances list"
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	for _, l := range strings.Split(res.Stdout, "\n") {
		log.Printf("list: %s\ntarget:%s", l, i.Instance)
		if strings.Contains(l, i.Instance) {
			return true, nil
		}
	}
	return false, nil
}

func (i CloudSQLInstance) ConfirmCloudSQLInstanceStatusRunnable() error {
	if err := retry.Retry(30, 20*time.Second, func() error {
		instance, err := i.DescribeInstance()
		if err != nil {
			return fmt.Errorf("failed to DescribeInstance: %w", err)
		}
		wantStatus := "RUNNABLE"
		if instance.State != wantStatus {
			return fmt.Errorf("not matched. current: %s, expected: %s", instance.State, wantStatus)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to confirm cloud sql: %w", err)
	}
	return nil
}

func (i CloudSQLInstance) createTestDatabase() error {
	if !strings.Contains(i.Instance, "integration-test") {
		return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}

	cmd := fmt.Sprintf("gcloud sql databases create %s --instance=%s", i.Database, i.Instance)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	return nil
}

func (i CloudSQLInstance) findDatabase() error {
	if !strings.Contains(i.Instance, "integration-test") {
		return fmt.Errorf("instance name should contains 'integration-test'. instance: %s", i.Instance)
	}

	cmd := fmt.Sprintf("gcloud sql databases list --instance=%s", i.Instance)
	res, err := command.ExecAndWait(cmd)
	if err != nil {
		return fmt.Errorf("failed to ExecAndWait: %v, cmd: %s, res: %#v", err, cmd, res)
	}
	//fmt.Println(res.Stdout)

	if err := findDatabase(res.Stdout, i.Database); err != nil {
		return fmt.Errorf("failed to find test database name. list: %s", res.Stdout)
	}
	return nil
}

func findDatabase(s, dbName string) error {
	lines := strings.Split(s, "\n") // 改行区切りでlinesに格納
	for _, l := range lines {
		dbNames := strings.Fields(l)
		if dbNames[0] == dbName {
			return nil
		}
	}
	return fmt.Errorf("no match: %s", dbName)
}
