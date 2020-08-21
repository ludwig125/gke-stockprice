// +build integration

package googledrive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"google.golang.org/api/drive/v3"
)

// CommandResultUpload is struct to command result upload.
type CommandResultUpload struct {
	srv      *drive.Service
	command  string
	fileInfo FileInfo
}

// Exec executes command and upload result to googledrive.
func (c CommandResultUpload) Exec(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stdout, stderr, cmdWait, err := commandContextExec(ctx, c.command)
	if err != nil {
		return fmt.Errorf("failed to commandExec: %v", err)
	}
	go func() {
		scan(stderr)
	}()

	if err := UploadFile(c.srv, stdout, c.fileInfo); err != nil {
		// if err := UploadFile(c.srv, stdout, c.fname, c.fdescription, c.mimeType, c.parentID, c.gzipFlg); err != nil {
		cancel()
		return fmt.Errorf("failed to UploadFile: %v", err)
	}
	if err := cmdWait(); err != nil {
		return fmt.Errorf("failed to exec Command Wait: %v", err)
	}
	return nil
}

func commandContextExec(ctx context.Context, command string) (io.ReadCloser, io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to exec Command StdoutPipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to exec Command StderrPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to exec Command Start: %v", err)
	}

	// goroutineでWaitをしたら以下のエラーが出たのでWaitする関数を返してupload後に実行させることにした
	// An error occurred: read |0: file already closed
	return stdout, stderr, func() error {
		return cmd.Wait()
	}, nil
}

func scan(s io.ReadCloser) {
	scanner := bufio.NewScanner(s)
	for scanner.Scan() {
		l := scanner.Text()
		log.Println(l)
	}
}
