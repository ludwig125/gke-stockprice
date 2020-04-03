package command

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"testing"
)

func TestExec(t *testing.T) {
	//　ログの出力先を/dev/nullにして捨てる
	log.SetOutput(ioutil.Discard)
	//　終わったらログ出力先を標準出力に戻す
	defer log.SetOutput(os.Stdout)

	cases := []struct {
		name            string
		cmd             string
		wantCmdStdout   string
		wantCmdStderr   string
		wantCmdStderrEn string
		wantCmdErrMsg   string
	}{
		{
			name:          "exit_status_0",
			cmd:           "echo 'a' && echo 'b' 1>&2",
			wantCmdStdout: "a",
			wantCmdStderr: "b",
			wantCmdErrMsg: "",
		},
		{
			name:          "exit_status_1",
			cmd:           "ls | grep abcdefghijklm",
			wantCmdStdout: "",
			wantCmdStderr: "",
			wantCmdErrMsg: "failed to exec Command Wait: exit status 1",
		},
		{
			name:            "exit_status_2",
			cmd:             "ls abc",
			wantCmdStdout:   "",
			wantCmdStderr:   "ls: 'abc' にアクセスできません: そのようなファイルやディレクトリはありません",
			wantCmdStderrEn: "ls: cannot access 'abc': No such file or directory",
			wantCmdErrMsg:   "failed to exec Command Wait: exit status 2",
		},
		{
			name:          "contains_\\n",
			cmd:           "echo 'a\nb\nc'",
			wantCmdStdout: "a\nb\nc",
			wantCmdStderr: "",
			wantCmdErrMsg: "",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			gotCh, err := Exec(tt.cmd)
			if err != nil {
				t.Fatalf("cmd error: %v", err)
			}
			for got := range gotCh {
				if got.Err != nil {
					var wantCmdErrType *exec.ExitError
					// *exec.ExitError型であるはず
					if !errors.As(got.Err, &wantCmdErrType) {
						t.Fatalf("got error type: %v; want type: %v", got.Err, wantCmdErrType)
					}
					if got.Err.Error() != tt.wantCmdErrMsg {
						t.Fatalf("got error message: %v; want message: %v", got.Err, tt.wantCmdErrMsg)
					}
				}
				if got.Stdout != tt.wantCmdStdout {
					t.Errorf("got Stdout: %s; want Stdout: %s", got.Stdout, tt.wantCmdStdout)
				}
				if got.Stderr != tt.wantCmdStderr {
					if got.Stderr != tt.wantCmdStderrEn {
						t.Errorf("got Stderr: %s; want Stderr: %s or %s", got.Stderr, tt.wantCmdStderr, tt.wantCmdStderrEn)
					}
				}
			}
		})
	}
}

func TestExecAndWait(t *testing.T) {
	//　ログの出力先を/dev/nullにして捨てる
	log.SetOutput(ioutil.Discard)
	//　終わったらログ出力先を標準出力に戻す
	defer log.SetOutput(os.Stdout)

	cases := []struct {
		name         string
		cmd          string
		wantResult   Result
		wantResultEn Result // 実行環境によってエラーメッセージが日本語か英語か変わる
		wantErr      error
	}{
		{
			name: "exit_status_0",
			cmd:  "echo 'a' && echo 'b' 1>&2",
			wantResult: Result{
				Stdout: "a",
				Stderr: "b",
				Err:    nil,
			},
			wantErr: nil,
		},
		{
			name: "exit_status_1",
			cmd:  "ls | grep abcdefghijklm",
			wantResult: Result{
				Stdout: "",
				Stderr: "",
				Err:    errors.New("failed to exec Command Wait: exit status 1"),
			},
			wantErr: errors.New("failed to Exec as a result: failed to exec Command Wait: exit status 1"),
		},
		{
			name: "exit_status_2",
			cmd:  "ls abc",
			wantResult: Result{
				Stdout: "",
				Stderr: "ls: 'abc' にアクセスできません: そのようなファイルやディレクトリはありません",
				Err:    errors.New("failed to exec Command Wait: exit status 2"),
			},
			wantResultEn: Result{
				Stdout: "",
				Stderr: "ls: cannot access 'abc': No such file or directory",
				Err:    errors.New("failed to exec Command Wait: exit status 2"),
			},
			wantErr: errors.New("failed to Exec as a result: failed to exec Command Wait: exit status 2"),
		},
		{
			name: "contains_\\n",
			cmd:  "echo 'a\nb\nc'",
			wantResult: Result{
				Stdout: "a\nb\nc",
				Stderr: "",
				Err:    nil,
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExecAndWait(tt.cmd)
			if err != nil {
				if err.Error() != tt.wantErr.Error() {
					t.Fatalf("got error message: %v; want message: %v", err, tt.wantErr)
				}
			}
			if got.Err != nil {
				var wantCmdErrType *exec.ExitError
				// *exec.ExitError型であるはず
				if !errors.As(got.Err, &wantCmdErrType) {
					t.Fatalf("got error type: %v; want type: %v", got.Err, wantCmdErrType)
				}
				if got.Err.Error() != tt.wantResult.Err.Error() {
					t.Fatalf("got error message: %v; want message: %v", got.Err, tt.wantResult.Err)
				}
			}
			if got.Stdout != tt.wantResult.Stdout {
				t.Errorf("got Stdout: %s; want Stdout: %s", got.Stdout, tt.wantResult.Stdout)
			}
			if got.Stderr != tt.wantResult.Stderr {
				if got.Stderr != tt.wantResultEn.Stderr {
					t.Errorf("got Stderr: %s; want Stderr: %s or %s", got.Stderr, tt.wantResult.Stderr, tt.wantResultEn.Stderr)
				}
			}
		})
	}
}
