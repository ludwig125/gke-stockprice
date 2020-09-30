package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFormatPath(t *testing.T) {
	tests := map[string]struct {
		directoryPath string
		want          string
	}{
		"directory": {
			directoryPath: "./tmp",
			want:          "tmp",
		},
		"directory2": {
			directoryPath: "./tmp/",
			want:          "tmp",
		},
		"directory3": {
			directoryPath: "tmp/",
			want:          "tmp",
		},
		"sub_directory": {
			directoryPath: "./1/2/3/4",
			want:          "1/2/3/4",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := formatPath(tc.directoryPath)
			if got != tc.want {
				t.Errorf("got: %s, want: %s", got, tc.want)
			}
		})
	}
}

func TestCreateFiles(t *testing.T) {
	directory := "1/2/3/4"
	directoryPath := formatPath(directory)
	tests := map[string]struct {
		// directoryPath string
		files []File
		wants []string
	}{
		"normal": {
			// directoryPath: "1/2/3/4",
			files: []File{
				{Name: "test1.txt", Content: "test1"},
				{Name: "test2.txt", Content: "test2"},
				{Name: "test3.txt", Content: "test3"},
			},
			wants: []string{"test1", "test2", "test3"},
		},
		"contains_new_line": {
			// directoryPath: "1/2/3/4",
			files: []File{
				{
					Name: "test4.txt",
					Content: `aa
bb`,
				},
			},
			wants: []string{`aa
bb`},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if err := os.MkdirAll(directoryPath, 0777); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.RemoveAll("1"); err != nil {
					t.Fatal(err)
				}
			}()
			if err := CreateFiles(directoryPath, tc.files...); err != nil {
				t.Errorf("failed to CreateFiles: %v", err)
			}

			got, err := getFileContents(directoryPath, tc.files...)
			if err != nil {
				t.Errorf("failed to getFileContents: %v", err)
			}
			t.Logf("got: %v, want: %v", got, tc.wants)
			if !reflect.DeepEqual(got, tc.wants) {
				t.Errorf("got: %v, want: %v", got, tc.wants)
			}
		})
	}
}

func getFileContents(dirpath string, fs ...File) ([]string, error) {
	var contents []string
	for _, f := range fs {
		c, err := readFile(filepath.Join(formatPath(dirpath), f.Name))
		if err != nil {
			return nil, err
		}
		contents = append(contents, c)
	}
	return contents, nil
}

func readFile(file string) (string, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	return string(b), nil
}
