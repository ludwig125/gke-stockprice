package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func formatPath(path string) string {
	var formated string
	for _, p := range filepath.SplitList(path) {
		formated = filepath.Join(formated, p)
	}
	return formated
}

// File has file name and file content.
type File struct {
	Name    string
	Content string
}

// CreateFiles create multiple files.
func CreateFiles(path string, files ...File) error {
	path = formatPath(path)
	if !fileExists(path) {
		return fmt.Errorf("failed to find directory: %s", path)
	}

	for _, f := range files {
		filePath := filepath.Join(path, f.Name)
		if err := create(filePath, f.Content); err != nil {
			return fmt.Errorf("failed to createFile: %v", err)
		}
	}
	return nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func create(filepath, content string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to Create: %v", err)
	}
	defer f.Close()
	// 改行入れると正しく認識されないので改行を削る
	// 例
	//   2020/01/11 22:50:08 errors parsing config:
	//   googleapi: Error 400: Invalid request: instance name (gke-stockprice-integration-test-202001100551
	//   )., invalid
	if _, err := io.WriteString(f, strings.TrimRight(content, "\n")); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}
	return nil
}

// ShowFiles print all files under path.
func ShowFiles(path string) error {
	path = formatPath(path)
	if !fileExists(path) {
		return fmt.Errorf("failed to find directory: %s", path)
	}
	files, _ := filepath.Glob(path + "/*")
	for _, f := range files {
		printPathAndSize(f)
	}
	return nil
}

func printPathAndSize(path string) {
	// ファイルサイズの取得
	var s syscall.Stat_t
	syscall.Stat(path, &s)

	fmt.Print(path)
	fmt.Print(": ")
	fmt.Print(s.Size)
	fmt.Println(" bytes")

}
