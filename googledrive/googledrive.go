package googledrive

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"sort"

	"golang.org/x/net/context"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// GetDriveService gets drive client.
func GetDriveService(ctx context.Context, driveCredential string) (*drive.Service, error) {

	srv, err := drive.NewService(ctx, option.WithCredentialsFile(driveCredential), option.WithScopes(drive.DriveScope))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Drives Client: %v", err)
	}
	return srv, nil
}

// GetFolderIDOrCreate returns folder id. If folder is not yet, create it and returns folder id.
func GetFolderIDOrCreate(srv *drive.Service, folderName, permissionTargetGmail string) (string, error) {
	id, err := getFolderID(srv, folderName)
	if err != nil {
		return "", fmt.Errorf("failed to getFolderID: %v", err)
	}
	if id != "" {
		log.Printf("folder already exists. %s(%s)", folderName, id)
		return id, nil
	}

	// folderがまだない場合は作る
	log.Println(folderName, "folder not exists yet, create it")
	id, err = createFolder(srv, folderName, permissionTargetGmail)
	if err != nil {
		return "", fmt.Errorf("failed to createFolder: %v", err)
	}
	return id, nil
}

// getFolderID returns folder id.
func getFolderID(srv *drive.Service, folderName string) (string, error) {
	return getObjectID(srv, folderName, "application/vnd.google-apps.folder", "folder", "")
}

// getFileID returns file id.
func getFileID(srv *drive.Service, fileName, mimeType, parentID string) (string, error) {
	return getObjectID(srv, fileName, mimeType, "file", parentID)
}

func getObjectID(srv *drive.Service, name, mimeType, objectType, parentID string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%s name is empty", objectType)
	}

	// Qの指定対象はここで調べる：https://developers.google.com/drive/api/v3/reference/files
	// 検索方法参考: https://developers.google.com/drive/api/v3/search-files
	q := fmt.Sprintf(`name="%s" and mimeType="%s"`, name, mimeType)
	if parentID != "" {
		q += fmt.Sprintf(` and "%s" in parents`, parentID)
	}

	r, err := srv.Files.List().
		// Fields("nextPageToken, files(id, name)").
		Q(q).
		//MaxResults(1).
		Do()
	if err != nil {
		log.Printf("failed to List %ss: %v. target: %s. mimeType: %s", err, objectType, name, mimeType)
		return "", nil // まだ作られていない場合は普通にあり得るのでエラーは返さない
	}
	if len(r.Files) == 0 { // まだ作られていない場合は普通にあり得るのでエラーは返さない
		log.Printf("got no %ss. target: %s. mimeType: %s", objectType, name, mimeType)
		return "", nil
	}
	if len(r.Files) >= 2 { // 2つ以上見つかった場合はWARN
		log.Printf("WARN: there are duplicate %ss. target: %s. mimeType: %s", objectType, name, mimeType)
		for i, f := range r.Files {
			log.Printf("  [%d] name: %s, id: %s", i+1, f.Name, f.Id)
		}
		log.Println("  -> chose", r.Files[0].Id)
	}
	return r.Files[0].Id, nil
}

func createFolder(srv *drive.Service, folderName, permissionTargetGmail string) (string, error) {
	f := &drive.File{
		Name:     folderName,
		MimeType: "application/vnd.google-apps.folder",
	}
	res, err := srv.Files.
		Create(f).
		ProgressUpdater(func(now, size int64) { fmt.Printf("%d, %d\r", now, size) }).
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to create: %v", err)
	}
	folderID := res.Id
	log.Println("created folder:", res.Name, res.Id)

	if permissionTargetGmail != "" {
		// service accountで作成したファイル、フォルダは通常のユーザには見られない
		// 以下でパーミッションを変更して所有者を自分に変更する
		// 参考：https://teratail.com/questions/152937
		permission := &drive.Permission{
			EmailAddress: permissionTargetGmail, // folderの共有先のGmail（自分のGmailアドレス）
			Role:         "owner",
			Type:         "user",
		}
		if _, err := srv.Permissions.Create(folderID, permission).TransferOwnership(true).Do(); err != nil {
			return "", fmt.Errorf("failed to create permission: %v", err)
		}
		log.Println("created folder permission:", res.Name, res.Id, permissionTargetGmail)
	}
	return folderID, nil
}

// FileInfo is struct for upload file.
type FileInfo struct {
	Name        string
	Description string
	// MimeType: text/plain or application/gzip
	// ref: https://developer.mozilla.org/ja/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types
	// google drive mimeType: https://developers.google.com/drive/api/v3/mime-types
	// mimeType official: http://www.iana.org/assignments/media-types/media-types.xhtml
	MimeType  string
	ParentID  string
	Overwrite bool // 上書きするかのフラグ
}

// UploadFile uploads file.
func UploadFile(srv *drive.Service, content io.Reader, fi FileInfo) error {
	log.Printf("upload target name: %s, mimeType: %s, parentID: %v, overwrite: %v", fi.Name, fi.MimeType, fi.ParentID, fi.Overwrite)
	if fi.MimeType == "application/gzip" { // これをすると一時的にメモリにcontentが全部載るので、リソースを食うはず
		log.Println("mimeType is application/gzip. try to gzip")
		d, err := gzipData(ioReaderToBytes(content))
		if err != nil {
			return fmt.Errorf("failed to gzip: %v", err)
		}
		// []byte からio.Readerに戻す
		content = bytes.NewReader(d)
	} else if fi.MimeType != "text/plain" {
		return fmt.Errorf("invalid mimeType: %s. you can use text/plain or application/gzip", fi.MimeType)
	}

	// ref. https://pkg.go.dev/google.golang.org/api/googleapi?tab=doc#ProgressUpdater
	progressUpdater := func(current, total int64) { log.Printf("%dB / %dB total\n", current, total) }

	log.Println("start upload")
	fileID, err := getFileID(srv, fi.Name, fi.MimeType, fi.ParentID)
	if err != nil {
		return fmt.Errorf("failed to getFileID, target file: %s. mimeType: %s, err: %v", fi.Name, fi.MimeType, err)
	}

	// overwriteがtrueでfileIDが空でなければ（すでにあれば）上書きする
	// そうでないのなら新規作成
	if fi.Overwrite && fileID != "" {
		log.Printf("overwrite target: Name: %s, ID: %s, parentID: %s", fi.Name, fileID, fi.ParentID)
		f := &drive.File{Name: fi.Name, Description: fi.Description, MimeType: fi.MimeType}
		// Updateメソッドを使うときはAddParentsでparentsを指定しないと以下のエラーになる
		// googleapi: Error 403: The parents field is not directly writable in update requests. Use the addParents and removeParents parameters instead., fieldNotWritable
		// ref: https://github.com/googleapis/google-api-go-client/blob/a87a0974131ca1aa879e0dd1d89726d577540c28/drive/v3/drive-gen.go#L1335-L1341
		// > Update requests must use the addParents and
		// > removeParents parameters to modify the parents list.
		r, err := srv.Files.Update(fileID, f).
			AddParents(fi.ParentID).
			Media(content).
			ProgressUpdater(progressUpdater).
			Do()
		if err != nil {
			return fmt.Errorf("error occurred in upload(update): %v", err)
		}
		log.Printf("upload(update) Done. ID : %s\n", r.Id)
		return nil
	}
	log.Println("create target:", fi.Name)
	f := &drive.File{Name: fi.Name, Description: fi.Description, MimeType: fi.MimeType}
	if fi.ParentID != "" {
		f.Parents = []string{fi.ParentID}
	}
	r, err := srv.Files.Create(f).
		Media(content).
		ProgressUpdater(progressUpdater).
		Do()
	if err != nil {
		return fmt.Errorf("error occurred in upload(create): %v", err)
	}
	log.Printf("upload(create) Done. ID : %s\n", r.Id)
	return nil
}

// // UploadFile uploads file.
// func UploadFile(srv *drive.Service, content io.Reader, fi FileInfo) error {
// 	log.Printf("upload target name: %s, mimeType: %s, parentID: %v, overwrite: %v", fi.Name, fi.MimeType, fi.ParentID, fi.Overwrite)
// 	if fi.MimeType == "application/gzip" { // これをすると一時的にメモリにcontentが全部載るので、リソースを食うはず
// 		log.Println("mimeType is application/gzip. try to gzip")
// 		d, err := gzipData(ioReaderToBytes(content))
// 		if err != nil {
// 			return fmt.Errorf("failed to gzip: %v", err)
// 		}
// 		// []byte からio.Readerに戻す
// 		content = bytes.NewReader(d)
// 	} else if fi.MimeType != "text/plain" {
// 		return fmt.Errorf("invalid mimeType: %s. you can use text/plain or application/gzip", fi.MimeType)
// 	}

// 	// ref. https://pkg.go.dev/google.golang.org/api/googleapi?tab=doc#ProgressUpdater
// 	progressUpdater := func(current, total int64) { log.Printf("%dB / %dB total\n", current, total) }

// 	var uploadedID string
// 	log.Println("start upload")
// 	if fi.Overwrite {
// 		fileID, err := getFileID(srv, fi.Name, fi.MimeType)
// 		if err != nil {
// 			return fmt.Errorf("failed to getFileID, target file: %s. mimeType: %s, err: %v", fi.Name, fi.MimeType, err)
// 		}
// 		if fileID == "" {
// 			return fmt.Errorf("no target file: %s. mimeType: %s. fileID is empty", fi.Name, fi.MimeType)
// 		}
// 		log.Println("overwrite target:", fi.Name, fileID)
// 		f := &drive.File{Name: fi.Name, Description: fi.Description, MimeType: fi.MimeType}
// 		// Updateメソッドを使うときはAddParentsでparentsを指定しないと以下のエラーになる
// 		// googleapi: Error 403: The parents field is not directly writable in update requests. Use the addParents and removeParents parameters instead., fieldNotWritable
// 		// ref: https://github.com/googleapis/google-api-go-client/blob/a87a0974131ca1aa879e0dd1d89726d577540c28/drive/v3/drive-gen.go#L1335-L1341
// 		// > Update requests must use the addParents and
// 		// > removeParents parameters to modify the parents list.
// 		r, err := srv.Files.Update(fileID, f).
// 			AddParents(fi.ParentID).
// 			Media(content).
// 			ProgressUpdater(progressUpdater).
// 			Do()
// 		if err != nil {
// 			return fmt.Errorf("error occurred in upload(update): %v", err)
// 		}
// 		uploadedID = r.Id
// 	} else {
// 		f := &drive.File{Name: fi.Name, Description: fi.Description, MimeType: fi.MimeType}
// 		if fi.ParentID != "" {
// 			f.Parents = []string{fi.ParentID}
// 		}
// 		r, err := srv.Files.Create(f).
// 			Media(content).
// 			ProgressUpdater(progressUpdater).
// 			Do()
// 		if err != nil {
// 			return fmt.Errorf("error occurred in upload(create): %v", err)
// 		}
// 		uploadedID = r.Id
// 	}

// 	log.Printf("upload Done. ID : %s\n", uploadedID)
// 	return nil
// }

// DownloadFile uploads file.
func DownloadFile(srv *drive.Service, fileID, mimeType string) (string, error) {
	log.Println("start download")

	// v2のしかGoのサンプルがないのでなんとなくで作った
	// https://developers.google.com/drive/api/v2/reference/files/get

	// こういうDownload方法もある：https://stackoverflow.com/questions/18177419/download-public-file-from-google-drive-golang
	resp, err := srv.Files.
		Get(fileID).
		Download()
	if err != nil {
		return "", fmt.Errorf("error occurred in download: %v. fileID: %s", err, fileID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// resp.Bodyを読み込んで捨てる
		if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
			return "", fmt.Errorf("failed to ioutil.Discard: %v", err)
		}
		return "", fmt.Errorf("status error: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	log.Printf("download Done. ID : %s\n", fileID)

	if mimeType == "application/gzip" {
		log.Println("mimeType is application/gzip. try to gunzip")
		d, err := gunzipData(body)
		if err != nil {
			return "", fmt.Errorf("failed to gunzip: %v", err)
		}
		return string(d), nil
	} else if mimeType != "text/plain" {
		return "", fmt.Errorf("invalid mimeType: %s. you can use text/plain or application/gzip", mimeType)
	}

	return string(body), nil
}

func ioReaderToBytes(r io.Reader) []byte {
	buf := new(bytes.Buffer)
	io.Copy(buf, r)
	return buf.Bytes()
}

func gzipData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return nil, fmt.Errorf("failed to gzip Write: %v", err)
	}
	if err := gz.Flush(); err != nil {
		return nil, fmt.Errorf("failed to gzip Flush: %v", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to gzip Close: %v", err)
	}

	return b.Bytes(), nil
}

func gunzipData(data []byte) ([]byte, error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, fmt.Errorf("failed to create NewReader: %v", err)
	}

	var res bytes.Buffer
	_, err = res.ReadFrom(r)
	if err != nil {
		return nil, fmt.Errorf("failed to ReadFrom: %v", err)
	}

	return res.Bytes(), nil
}

// PrintList prints all file.
func PrintList(srv *drive.Service, cond string) error {
	l, err := List(srv, cond)
	if err != nil {
		return fmt.Errorf("failed to List: %v", err)
	}
	fmt.Println("Files:")
	if len(l) == 0 {
		fmt.Println("No files found.")
	} else {
		for _, f := range l {
			fmt.Printf("%s (%s) parents: %v\n", f.Name, f.Id, f.Parents)
		}
	}
	return nil
}

// List returns all files which satisfy search condition with all file fields.
func List(srv *drive.Service, cond string) ([]*drive.File, error) {
	// 以下のFieldsで指定した項目がFileの中身に返ってくる
	// Fieldsで指定していない項目については、File.Nameのように参照しようとしても空なので注意
	// Fieldsの項目の定義：https://developers.google.com/drive/api/v3/reference/files
	listCall := srv.Files.List().
		Fields("nextPageToken, files(parents, id, name, kind, size, mimeType, lastModifyingUser, createdTime, modifiedTime, iconLink, owners, folderColorRgb, shared, webViewLink, webContentLink)")

	// 検索条件がある場合はそれを指定する
	// 検索例：https://developers.google.com/drive/api/v3/search-files
	// 検索に使用できる項目：https://developers.google.com/drive/api/v3/ref-search-terms
	if cond != "" {
		listCall = listCall.Q(cond)
	}

	r, err := listCall.
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve files: %v", err)
	}
	return r.Files, nil
}

// Delete deletes all files from google drive.
func Delete(srv *drive.Service, ids ...string) error {
	// ref: https://developers.google.com/drive/api/v2/reference/files/delete
	// v3にはGoのサンプルがなかったけど大体同じだった

	var e string
	for _, id := range ids {
		if err := srv.Files.Delete(id).Do(); err != nil {
			e += fmt.Sprintf("failed to delete file: %v ", err)
		}
		log.Printf("delete Done. ID : %s\n", id)
	}
	if e != "" { // まとめて最後にエラーとして返す
		return errors.New(e)
	}
	return nil
}

// Sort sort drive.File by file field.
func Sort(srv *drive.Service, fs []*drive.File, field string) ([]*drive.File, error) {
	switch field {
	case "name":
		sort.Slice(fs, func(i, j int) bool {
			return fs[i].Name < fs[j].Name
		})
		return fs, nil
	case "createdTime":
		sort.Slice(fs, func(i, j int) bool {
			return fs[i].CreatedTime < fs[j].CreatedTime
		})
		return fs, nil
	case "modifiedTime":
		sort.Slice(fs, func(i, j int) bool {
			return fs[i].ModifiedTime < fs[j].ModifiedTime
		})
		return fs, nil
	}
	return nil, fmt.Errorf("invalid sort field: %s", field)
}
