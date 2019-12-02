package command

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Result contains result of exec.Command.
type Result struct {
	Stdout string
	Stderr string
	Err    error
}

// Exec execute external command without waiting.
func Exec(c string) (chan Result, error) {
	fmt.Println("----command----")
	fmt.Println(c)
	fmt.Println("--------------")
	cmd := exec.Command("bash", "-c", c)

	cmdResCh := make(chan Result)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		close(cmdResCh)
		return cmdResCh, fmt.Errorf("failed to exec Command StdoutPipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		close(cmdResCh)
		return cmdResCh, fmt.Errorf("failed to exec Command StderrPipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		close(cmdResCh)
		return cmdResCh, fmt.Errorf("failed to exec Command Start: %w", err)
	}

	go func() {
		defer close(cmdResCh)

		// scanで結果を画面に出力すると同時にレスポンス用に格納する
		fmt.Println("----stdout----")
		cmdStdout := scan(stdout)
		//cmdStdout := read(stdout)
		fmt.Println("--------------")
		fmt.Println("----stderr----")
		cmdStderr := scan(stderr)
		fmt.Println("--------------")

		var cmdErr error
		if err := cmd.Wait(); err != nil {
			cmdErr = fmt.Errorf("failed to exec Command Wait: %w", err)
		}
		cmdResCh <- Result{
			Stdout: cmdStdout,
			Stderr: cmdStderr,
			Err:    cmdErr,
		}
	}()
	return cmdResCh, nil
}

// scan prints messages in display and save its in res.
func scan(s io.ReadCloser) string {
	var res string // scanした結果を格納
	scanner := bufio.NewScanner(s)
	for scanner.Scan() {
		l := scanner.Text()
		fmt.Printf("%#v\n", l)
		res += fmt.Sprintf("%s\n", l) // Textは改行を削除してしまうので改行を付与
	}
	return strings.TrimRight(res, "\n") // 末尾の改行だけ削る
}

// ExecAndWait execute external command and wait it until done.
func ExecAndWait(cmd string) (Result, error) {
	ch, err := Exec(cmd)
	if err != nil {
		return Result{}, fmt.Errorf("failed to Exec before start: %w", err)
	}
	res := <-ch
	// Execの結果にエラーがあったらエラーとして返す
	if res.Err != nil {
		return res, fmt.Errorf("failed to Exec as a result: %w", res.Err)
	}
	return res, nil
}
