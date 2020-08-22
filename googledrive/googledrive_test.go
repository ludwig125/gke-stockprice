// +build integration

package googledrive

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
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

func mustGetenv(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("%s environment variable not set", k)
	}
	return v
}
