// +build integration

package googledrive

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCommandResultUpload(t *testing.T) {
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

	// 以下のスクリプトを実行した結果をファイルとしてUploadする
	cmd := fmt.Sprintf("for i in $(seq 1 5); do echo $i; done")

	// for i in $(seq 1 5); do echo $i; done の結果がファイルに書き込まれているはず
	wantContent := `1
2
3
4
5
`
	tests := map[string]struct {
		localPrintLines int
		wantOutput      string
	}{
		"no_output_to_Stdout": {
			localPrintLines: 0,
			wantOutput:      "",
		},
		"all_output_to_Stdout": {
			localPrintLines: -1,
			wantOutput: `print all lines among upload file
1
2
3
4
5`, // 全件出力する
		},
		"last3_output_to_Stdout": {
			localPrintLines: 3,
			wantOutput: `print last 3 lines among upload file
3
4
5`, // 最後の３行のみ出力する
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// 以下はStdoutの結果をテストするため
			tmpStdout := os.Stdout
			defer func() {
				os.Stdout = tmpStdout
			}()
			r, w, _ := os.Pipe()
			os.Stdout = w
			// Stdoutの結果テスト用ここまで

			fileName := "googledrive-test-file"
			mimeType := "text/plain"
			fi := FileInfo{
				Name:        fileName,
				Description: "this is googledrive test file to test",
				MimeType:    mimeType,
				ParentID:    folderID,
				Overwrite:   true, // 上書きするかのフラグ(すでにファイルがなくてもエラーにはならない)
			}

			c, err := NewCommandResultUpload(srv, cmd, fi, tc.localPrintLines)
			if err != nil {
				t.Fatalf("failed to NewCommandResultUpload: %v", err)
			}
			if err := c.Exec(ctx); err != nil {
				t.Fatalf("failed to Exec: %v", err)
			}

			fileCondition := fmt.Sprintf("name = '%s' and mimeType = '%s'", fileName, mimeType)
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
			if mimeType != mimeType {
				t.Fatalf("got mimeType: %s, want: %s", mimeType, mimeType)
			}

			donwloaded, err := DownloadFile(srv, fileID, mimeType)
			if err != nil {
				t.Fatalf("failed to DownloadFile: %v", err)
			}

			if donwloaded != wantContent {
				t.Fatalf("downloaded: %s, want: %s", donwloaded, wantContent)
			}
			t.Log("file content:", wantContent)

			// 以下はStdoutの結果をテストするため
			w.Close() // クローズしないと以下のbuf.ReadFromが永遠に読み込み待ち状態になる
			var buf bytes.Buffer
			buf.ReadFrom(r)
			gotStdout := strings.TrimRight(buf.String(), "\n")
			if gotStdout != tc.wantOutput {
				t.Errorf("gotStdout: '%s', want: '%s'", gotStdout, tc.wantOutput)
			}
		})
	}
}
