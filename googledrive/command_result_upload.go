package googledrive

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"google.golang.org/api/drive/v3"
)

// CommandResultUpload is struct to command result upload.
type CommandResultUpload struct {
	Srv                *drive.Service
	Command            string
	FileInfo           FileInfo
	Shell              string
	PrintNLinesAtLocal int // n: デバッグ用に末尾n行をローカルに出力. 0: 出力なし. -1: 全部出力
}

// NewCommandResultUpload create new CommandResultUpload.
func NewCommandResultUpload(srv *drive.Service, cmd string, fileInfo FileInfo, printNLinesAtLocal int) (*CommandResultUpload, error) {
	sh, err := getShell()
	if err != nil {
		return nil, fmt.Errorf("failed to getShell: %v", err)
	}

	if printNLinesAtLocal < -1 {
		return nil, fmt.Errorf("invalid PrintNLinesAtLocal: %d. PrintNLinesAtLocal should not be less than -1", printNLinesAtLocal)
	}
	switch printNLinesAtLocal {
	case 0:
		// ローカルには何も出力しない
		fmt.Println("==== Print no lines among upload file ====")
	case -1:
		// ローカルに全部出力
		fmt.Println("==== Print all lines among upload file ====")
	default:
		// ローカルに最後のN行だけ出力
		fmt.Printf("==== Print last %d lines among upload file ====\n", printNLinesAtLocal)
	}

	return &CommandResultUpload{
		Srv:                srv,
		Command:            cmd,
		FileInfo:           fileInfo,
		Shell:              sh,
		PrintNLinesAtLocal: printNLinesAtLocal,
	}, nil
}

func getShell() (string, error) {
	// shellを取得する。alpineではash. ubuntuではbash
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

	// stdoutの中身をローカルにも出力させるためにTeeReaderで分岐させる
	// TeeReaderの返り値はio.Readerなので、io.ReadCloser にするためにioutil.NopCloserを経由させる
	p := localPrinter{lines: c.PrintNLinesAtLocal}
	stdout = ioutil.NopCloser(io.TeeReader(stdout, &p))

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

type localPrinter struct {
	lines int
}

// io.Writer の要件を満たすためにWriteメソッドを実装
func (p *localPrinter) Write(data []byte) (int, error) {
	n := len(data)

	if p.lines == -1 {
		// ローカルに全部出力
		w := os.Stdout
		w.Write(data)
		return n, nil
	}

	// 改行で分割
	contents := strings.Split(string(data), "\n")
	// contents sliceの最後のp.lines分を出力させるために要素を計算する
	// 例：全部で５行の出力のうち、最後の３行を出力したいのであれば、5-3=2なので、 contents[2:]で出力させる
	// 実際にはcontentsの末尾は改行コードが入っているので、さらに-1する必要がある
	// 例えば、元のデータが１～５までの数字が各行にある場合、改行でSplitするとcontentsの中身は以下のようになる
	// contents [1 2 3 4 5 ] <- 最後に空白があるので全部で６要素ある
	lines := len(contents) - p.lines - 1
	if lines < 0 { // p.linesが全要素数より多かったら全要素を出力させる
		lines = 0
	}
	if lines >= len(contents) {
		lines = 0
	}
	for _, v := range contents[lines:] {
		fmt.Fprintf(os.Stdout, "%s\n", v)
	}

	return n, nil
}
