// +build integration

package googledrive

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/api/drive/v3"
)

func TestGoogleDrive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	driveCredential := mustGetenv(t, "CREDENTIAL_FILEPATH")
	srv, err := GetDriveService(ctx, "../"+driveCredential) // rootディレクトリに置いてあるserviceaccountのjsonを使う
	if err != nil {
		t.Fatalf("failed to GetDriveService: %v", err)
	}

	folderName := "gke-stockprice-googledrive-test-folder"
	folderID, err := GetFolderIDOrCreate(srv, folderName, "") // permission共有Gmailは空. この場合ユーザにはUIから見ることはできないことに注意
	if err != nil {
		t.Fatalf("failed to GetFolderIDOrCreate: %v", err)
	}
	t.Log("folderID:", folderID)
	defer func() { // testがおかしくなってもフォルダを削除して終わる
		t.Log("delete folder")
		if err := Delete(srv, folderID); err != nil {
			t.Fatalf("failed to delete folder: %v", err)
		}
	}()

	tests := map[string]struct {
		mimeType string
	}{
		"text_plain": {
			mimeType: "text/plain",
		},
		"application_gzip": {
			mimeType: "application/gzip",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fileName := "googledrive-test-file"
			t.Run("upload_download", func(t *testing.T) {
				fi := FileInfo{
					Name:        fileName,
					Description: "this is googledrive test file to test",
					MimeType:    tc.mimeType,
					ParentID:    folderID,
					Overwrite:   false, // 上書きするかのフラグ
				}
				content := "this is test"
				s := strings.NewReader(content)
				if err := UploadFile(srv, s, fi); err != nil {
					t.Fatalf("failed to UploadFile: %v", err)
				}

				fileCondition := fmt.Sprintf("name = '%s' and mimeType = '%s'", fileName, tc.mimeType)
				uploadedFile, err := List(srv, fileCondition)
				if err != nil {
					t.Fatalf("failed to List: %v. search condition: %+v", err, fileCondition)
				}
				if len(uploadedFile) == 0 {
					t.Fatalf("failed to get file. search condition: %+v", fileCondition)
				}
				if len(uploadedFile) > 1 { // 2つ以上あったらおかしい
					for i, f := range uploadedFile {
						t.Logf("[%d] parents: %s, name: %s, id: %s", i, f.Parents, f.Name, f.Id)
					}
					t.Fatalf("there are duplicated files. search condition: %+v", fileCondition)
				}
				fileID, mimeType := uploadedFile[0].Id, uploadedFile[0].MimeType
				if mimeType != tc.mimeType {
					t.Fatalf("got mimeType: %s, want: %s", mimeType, tc.mimeType)
				}

				donwloaded, err := DownloadFile(srv, fileID, mimeType)
				if err != nil {
					t.Fatalf("failed to DownloadFile: %v", err)
				}
				if donwloaded != content {
					t.Fatalf("downloaded: %s, want: %s", donwloaded, content)
				}
				t.Log("file content:", content)
			})
			// 以下は上書きのテスト
			t.Run("upload_download_overwrite", func(t *testing.T) {
				fi := FileInfo{
					Name:        fileName,
					Description: "this is googledrive test file to test",
					MimeType:    tc.mimeType,
					ParentID:    folderID,
					Overwrite:   true, // 上書きする
				}
				content2 := "this is test2"
				s := strings.NewReader(content2)
				if err := UploadFile(srv, s, fi); err != nil {
					t.Fatalf("failed to UploadFile: %v", err)
				}

				fileCondition := fmt.Sprintf("name = '%s' and mimeType = '%s'", fileName, tc.mimeType)
				uploadedFile, err := List(srv, fileCondition)
				if err != nil {
					t.Fatalf("failed to List: %v. search condition: %+v", err, fileCondition)
				}
				if len(uploadedFile) == 0 {
					t.Fatalf("failed to get file. search condition: %+v", fileCondition)
				}
				if len(uploadedFile) > 1 { // 2つ以上あったらおかしい
					for i, f := range uploadedFile {
						t.Logf("[%d] parents: %s, name: %s, id: %s", i, f.Parents, f.Name, f.Id)
					}
					t.Fatalf("there are duplicated files. search condition: %+v", fileCondition)
				}
				fileID, mimeType := uploadedFile[0].Id, uploadedFile[0].MimeType
				if mimeType != tc.mimeType {
					t.Fatalf("got mimeType: %s, want: %s", mimeType, tc.mimeType)
				}

				donwloaded, err := DownloadFile(srv, fileID, mimeType)
				if err != nil {
					t.Fatalf("failed to DownloadFile: %v", err)
				}
				if donwloaded != content2 {
					t.Fatalf("downloaded: %s, want: %s", donwloaded, content2)
				}
				t.Log("file content:", content2)

				if err := Delete(srv, fileID); err != nil {
					t.Fatalf("failed to delete file: %v", err)
					afterFiles, err := List(srv, fmt.Sprintf("name = '%s'", fileName))
					if err != nil {
						t.Fatalf("failed to List: %v", err)
					}
					if len(afterFiles) > 0 { // ファイルが残っていたらおかしい
						for i, f := range afterFiles {
							t.Logf("[%d] parents: %s, name: %s, id: %s", i, f.Parents, f.Name, f.Id)
						}
						t.Fatalf("file stil exist")
					}
				}
			})
		})
	}
}

func TestSort(t *testing.T) {
	pDriveFiles := func(fs []drive.File) []*drive.File {
		pfs := make([]*drive.File, len(fs))
		for i, f := range fs {
			pfs[i] = pDriveFile(f)
		}
		// for i, f := range pfs {
		// 	fmt.Printf(" %d fs Name: %s\n", i, f.Name)
		// }

		return pfs
	}

	driveFilesList := func(fs []*drive.File) string {
		var s string
		for _, f := range fs {
			s += fmt.Sprintf("Name: %s, CreatedTime: %s, ModifiedTime: %s\n", f.Name, f.CreatedTime, f.ModifiedTime)
		}
		return s
	}
	tests := map[string]struct {
		files   []*drive.File
		field   string
		wants   []*drive.File
		wantErr bool
	}{
		"sort_by_name": {
			files: pDriveFiles([]drive.File{
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
			}),
			field: "name",
			wants: pDriveFiles([]drive.File{
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
			}),
			wantErr: false,
		},
		"sort_by_createdTime": {
			files: pDriveFiles([]drive.File{
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
			}),
			field: "createdTime",
			wants: pDriveFiles([]drive.File{
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
			}),
			wantErr: false,
		},
		"sort_by_modifiedTime": {
			files: pDriveFiles([]drive.File{
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
			}),
			field: "modifiedTime",
			wants: pDriveFiles([]drive.File{
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
			}),
			wantErr: false,
		},
		"invalid_sort": {
			files: pDriveFiles([]drive.File{
				{Name: "b", CreatedTime: "2020-01-01T00:00:00.003Z", ModifiedTime: "2020-01-01T00:00:00.003Z"},
				{Name: "d", CreatedTime: "2020-01-01T00:00:00.001Z", ModifiedTime: "2020-01-01T00:00:00.004Z"},
				{Name: "c", CreatedTime: "2020-01-01T00:00:00.002Z", ModifiedTime: "2020-01-01T00:00:00.002Z"},
				{Name: "a", CreatedTime: "2020-01-01T00:00:00.004Z", ModifiedTime: "2020-01-01T00:00:00.001Z"},
			}),
			field:   "abc",
			wants:   []*drive.File{},
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			driveCredential := mustGetenv(t, "CREDENTIAL_FILEPATH")
			srv, err := GetDriveService(ctx, "../"+driveCredential) // rootディレクトリに置いてあるserviceaccountのjsonを使う
			if err != nil {
				t.Fatalf("failed to GetDriveService: %v", err)
			}
			got, err := Sort(srv, tc.files, tc.field)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("gotErr %t, wantErr %t", err, tc.wantErr)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.wants) {
				// diff := cmp.Diff(driveFilesList(got), driveFilesList(tc.wants))
				// t.Errorf("got %#v\nwant %#v\n%v", got, tc.wants, diff)
				t.Errorf("got %#v\nwant %#v", driveFilesList(got), driveFilesList(tc.wants))
			}
		})
	}
}

func pDriveFiles(fs []drive.File) []*drive.File {
	pfs := make([]*drive.File, len(fs))
	for i, f := range fs {
		pfs[i] = pDriveFile(f)
	}
	return pfs
}

func driveFilesList(fs []*drive.File) string {
	var s string
	for _, f := range fs {
		s += fmt.Sprintf("Name: %s, CreatedTime: %s, ModifiedTime: %s\n", f.Name, f.CreatedTime, f.ModifiedTime)
	}
	return s
}

func pDriveFile(f drive.File) *drive.File {
	return &f
}

func mustGetenv(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("%s environment variable not set", k)
	}
	return v
}
