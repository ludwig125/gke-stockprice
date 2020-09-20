package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ludwig125/gke-stockprice/date"
	"github.com/ludwig125/gke-stockprice/googledrive"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/api/drive/v3"
)

// DumpConf has informations to dump table and upload google drive.
type DumpConf struct {
	DumpExecuteDays       []string // dumpを実行する曜日
	FolderName            string   // Google driveのフォルダの名前
	PermissionTargetGmail string   // フォルダを共有するユーザのGmail（共有しないとサービスアカウントで作ったファイルはユーザからは見られない）
	MimeType              string
	DumpTime              time.Time
	NeedToBackup          int
	DBUser                string
	DBPassword            string
	Host                  string
	Port                  string
	DBName                string
	TableNames            []string
}

// NewMySQLDumper create new MySQLDumper.
func NewMySQLDumper(srv *drive.Service, c DumpConf) (*MySQLDumper, error) {
	isEmpty := func(s string) bool {
		if s == "" {
			return true
		}
		return false
	}
	for _, s := range []string{c.FolderName, c.Host, c.Port, c.DBName} {
		if isEmpty(s) {
			return nil, fmt.Errorf("%s is empty", s)
		}
	}
	if len(c.TableNames) == 0 {
		return nil, errors.New("tableNames is empty")
	}

	dumpExecuteDays := []string{"Sunday"}
	if len(c.DumpExecuteDays) != 0 {
		dumpExecuteDays = append([]string(nil), c.DumpExecuteDays...)
	}
	permissionTargetGmail := ""
	if c.PermissionTargetGmail != "" {
		permissionTargetGmail = c.PermissionTargetGmail
	}

	mimeType := "text/plain"
	if c.MimeType != "" {
		mimeType = c.MimeType
	}
	dumpTime := now()
	if !c.DumpTime.IsZero() {
		dumpTime = c.DumpTime
	}
	needToBackup := 3 // default: 3
	if c.NeedToBackup != 0 {
		needToBackup = c.NeedToBackup
	}
	dbUser := "root"
	if c.DBUser != "" {
		dbUser = c.DBUser
	}
	return &MySQLDumper{
		Service:               srv,
		DumpExecuteDays:       dumpExecuteDays,
		FolderName:            c.FolderName,
		PermissionTargetGmail: permissionTargetGmail,
		MimeType:              mimeType,
		DumpTime:              dumpTime,
		NeedToBackup:          needToBackup,
		DBUser:                dbUser,
		DBPassword:            c.DBPassword,
		Host:                  c.Host,
		Port:                  c.Port,
		DBName:                c.DBName,
		TableNames:            c.TableNames,
	}, nil
}

// MySQLDumper is struct to mysqldump and upload google drive.
type MySQLDumper struct {
	Service               *drive.Service
	DumpExecuteDays       []string
	FolderName            string
	PermissionTargetGmail string
	MimeType              string
	DumpTime              time.Time
	NeedToBackup          int
	DBUser                string
	DBPassword            string
	Host                  string
	Port                  string
	DBName                string
	TableNames            []string
}

func baseFileName(dbName, tableName string) string {
	return fmt.Sprintf("%s-%s", dbName, tableName)
}

func fileName(dbName, tableName string, dumpTime time.Time) string {
	return fmt.Sprintf("%s-%s.sql", baseFileName(dbName, tableName), dumpTime.Format("2006-01-02"))
}
func fileDescription(dbName, tableName string, dumpTime time.Time) string {
	return fmt.Sprintf("%s %s dumpdate: %s", dbName, tableName, dumpTime.Format("2006-01-02"))
}

// MySQLDumpToGoogleDrive execute mysqldump and upload to google drive.
func (m MySQLDumper) MySQLDumpToGoogleDrive(ctx context.Context) error {
	folderID, err := googledrive.GetFolderIDOrCreate(m.Service, m.FolderName, m.PermissionTargetGmail) // permission共有Gmailを空にするとユーザにはUIから見ることはできないことに注意
	if err != nil {
		return fmt.Errorf("failed to GetFolderIDOrCreate: %v, folderName(parent folder): %s", err, m.FolderName)
	}

	for _, tableName := range m.TableNames {
		log.Printf("trying to mysqldump %s & upload", tableName)
		if err := m.backup(ctx, folderID, tableName); err != nil {
			return fmt.Errorf("failed to backup: %v", err)
		}

		if err := m.checkBackup(folderID, tableName); err != nil {
			return fmt.Errorf("failed to checkBackup: %v", err)
		}
		log.Printf("mysqldump %s & upload to google drive successfully", tableName)
	}
	return nil
}

func (m MySQLDumper) backup(ctx context.Context, folderID, tableName string) error {
	lastUpdatedTime, err := m.getLastUpdatedTime(folderID, tableName)
	if err != nil {
		return fmt.Errorf("failed to lastUpdatedTime: %v", err)
	}

	// DumpExecuteDaysで指定された曜日の場合は無条件でUploadする
	// 直前のDumpExecuteDaysの曜日からこのプログラムが実行されるまでの間に成功していなければUploadする
	ok, err := m.whetherOrNotUpload(lastUpdatedTime)
	if err != nil {
		return fmt.Errorf("failed to whetherOrNotUpload: %v", err)
	}
	if !ok {
		log.Println("no need to upload")
		return nil
	}
	log.Println("tyring to dump and upload")
	if err := m.execCmdAndUpload(ctx, folderID, tableName); err != nil {
		return fmt.Errorf("failed to upload: %v", err)
	}
	log.Println("file was uploaded successfully")
	return nil
}

func (m MySQLDumper) getLastUpdatedTime(folderID, tableName string) (time.Time, error) {
	fileCondition := fmt.Sprintf("name contains '%s' and mimeType = '%s' and '%s' in parents", baseFileName(m.DBName, tableName), m.MimeType, folderID)
	fs, err := googledrive.List(m.Service, fileCondition)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to List file: %v. condition: %s", err, fileCondition)
	}
	if len(fs) == 0 {
		return time.Time{}, nil
	}

	sortedFiles, err := googledrive.Sort(m.Service, fs, "modifiedTime")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to Sort: %v", err)
	}

	latestModifiedFile := sortedFiles[len(sortedFiles)-1]
	lastUpdatedTime, err := date.ParseRFC3339(latestModifiedFile.ModifiedTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parseRFC3339: %v. target ModifiedTime: %v", err, latestModifiedFile.ModifiedTime)
	}
	lastUpdatedTimeJST, err := date.TimeIn(lastUpdatedTime, "Asia/Tokyo")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to TimeIn: %v", err)
	}
	return lastUpdatedTimeJST, nil
}

func (m MySQLDumper) whetherOrNotUpload(lastUpdated time.Time) (bool, error) {
	ok, err := date.IsTargetWeekday(m.DumpTime, m.DumpExecuteDays)
	if err != nil {
		return false, fmt.Errorf("failed to IsTargetWeekday: %v", err)
	}
	if ok { // DumpExecuteDaysの曜日の場合は無条件でTrue
		return true, nil
	}

	// 今から一番最近のX曜日の日付を取得（時分秒はdumpTimeと同じ）
	d, err := date.GetLatestTargetWeekday(m.DumpTime, m.DumpExecuteDays)
	if err != nil {
		return false, fmt.Errorf("failed to GetLatestTargetWeekday: %v", err)
	}
	// その日の午前０時０分０秒を取得
	dMidnight, err := date.GetMidnight(d, "Asia/Tokyo")
	if err != nil {
		return false, fmt.Errorf("failed to GetMidnight: %v", err)
	}
	// lastUpdatedが直近のX曜日午前０時０分０秒以前であればTrue（更新する必要がある）
	if lastUpdated.Before(dMidnight) {
		log.Printf("lastUpdated: %v, last targetWeekday date: %v. need to update", lastUpdated, dMidnight)
		return true, nil
	}
	log.Printf("lastUpdated: %v, last targetWeekday date: %v. no need to update", lastUpdated, dMidnight)
	return false, nil
}

func (m MySQLDumper) execCmdAndUpload(ctx context.Context, folderID, tableName string) error {
	// cmd := "mysqldump -u root -p --host 127.0.0.1 --port 3307 stockprice daily"
	password := ""
	// passwordが設定されていればそれを使う
	if m.DBPassword != "" {
		password = fmt.Sprintf("--password=%s", m.DBPassword)
	}
	cmd := fmt.Sprintf("mysqldump -u %s %s --host %s --port %s %s %s", m.DBUser, password, m.Host, m.Port, m.DBName, tableName)

	fi := googledrive.FileInfo{
		Name:        fileName(m.DBName, tableName, m.DumpTime),
		Description: fileDescription(m.DBName, tableName, m.DumpTime),
		MimeType:    m.MimeType,
		ParentID:    folderID,
		Overwrite:   false,
	}

	c, err := googledrive.NewCommandResultUpload(m.Service, cmd, fi)
	if err != nil {
		return fmt.Errorf("failed to NewCommandResultUpload: %v", err)
	}
	if err := c.Exec(ctx); err != nil {
		return fmt.Errorf("failed to Exec: %v", err)
	}
	return nil
}

func (m MySQLDumper) checkBackup(folderID, tableName string) error {
	// ファイルがバックアップとして必要な数だけあるか？
	// バックアップとして必要な数より多ければ、古いファイルを消す
	log.Println("check required backup file count")
	driveFiles, err := m.getDriveFiles(folderID, tableName)
	if err != nil {
		return fmt.Errorf("failed to  getDriveFiles: %v", err)
	}
	if len(driveFiles) <= m.NeedToBackup {
		log.Printf("driveFiles num: %d does not over needToBackup: %d. no need to delete oldest file.", len(driveFiles), m.NeedToBackup)
		return nil
	}
	log.Println("backuped files are bellow")
	for i, f := range driveFiles {
		fmt.Printf("[%d] file: %s(%s) modifiedTime: %s", i, f.Name, f.Id, f.ModifiedTime)
	}
	if err := m.deleteOldestDriveFile(driveFiles); err != nil {
		return fmt.Errorf("failed to  deleteOldestDriveFile: %v", err)
	}
	log.Println("oldest file was deleted successfully")
	return nil
}

func (m MySQLDumper) getDriveFiles(folderID, tableName string) ([]*drive.File, error) {
	fileCondition := fmt.Sprintf("name contains '%s' and mimeType = '%s' and '%s' in parents", baseFileName(m.DBName, tableName), m.MimeType, folderID)
	fs, err := googledrive.List(m.Service, fileCondition)
	if err != nil {
		return nil, fmt.Errorf("failed to List file: %v. condition: %s", err, fileCondition)
	}
	if len(fs) == 0 {
		return nil, errors.New("no files")
	}
	return fs, nil
}

func (m MySQLDumper) deleteOldestDriveFile(fs []*drive.File) error {
	sorted, err := googledrive.Sort(m.Service, fs, "modifiedTime")
	if err != nil {
		return fmt.Errorf("failed to Sort: %v", err)
	}
	oldest := sorted[0]
	log.Printf("trying to delete file: %s(%s) modifiedTime: %s", oldest.Name, oldest.Id, oldest.ModifiedTime)
	if err := googledrive.Delete(m.Service, oldest.Id); err != nil {
		return fmt.Errorf("failed to Delete oldest file: %v, oldest file: %+v", err, oldest)
	}
	return nil
}
