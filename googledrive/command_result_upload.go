package googledrive

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"google.golang.org/api/drive/v3"
)

// CommandResultUpload is struct to command result upload.
type CommandResultUpload struct {
	Srv      *drive.Service
	Command  string
	FileInfo FileInfo
	Shell    string
}

// NewCommandResultUpload create new CommandResultUpload.
func NewCommandResultUpload(srv *drive.Service, cmd string, fileInfo FileInfo) (*CommandResultUpload, error) {
	sh, err := getShell()
	if err != nil {
		return nil, fmt.Errorf("failed to getShell: %v", err)
	}
	return &CommandResultUpload{
		Srv:      srv,
		Command:  cmd,
		FileInfo: fileInfo,
		Shell:    sh,
	}, nil
}

func getShell() (string, error) {
	// shellを取得する。alpineではash
	for _, sh := range []string{"bash", "ash"} {
		// errを確認すると、whichでbashが見つからないとエラーが返るので無視する
		out, _ := exec.Command("which", sh).Output()
		if strings.TrimRight(string(out), "\n") == fmt.Sprintf("/bin/%s", sh) {
			log.Printf("use %s for shell", sh)
			return sh, nil
		}
	}
	return "", errors.New("failed to find shell")
}

// Exec executes command and upload result to googledrive.
func (c CommandResultUpload) Exec(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stdout, stderr, cmdWait, err := c.commandContextExec(ctx)
	if err != nil {
		return fmt.Errorf("failed to commandExec: %v", err)
	}
	go func() {
		scan(stderr)
	}()

	if err := UploadFile(c.Srv, stdout, c.FileInfo); err != nil {
		cancel()
		return fmt.Errorf("failed to UploadFile: %v", err)
	}
	if err := cmdWait(); err != nil {
		return fmt.Errorf("failed to exec Command Wait: %v", err)
	}
	return nil
}

func (c CommandResultUpload) commandContextExec(ctx context.Context) (io.ReadCloser, io.ReadCloser, func() error, error) {
	// 'command' example: "for i in `seq 1 20`; do echo $i; sleep 1;done"
	cmd := exec.CommandContext(ctx, c.Shell, "-c", c.Command)
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
