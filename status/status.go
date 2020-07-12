package status

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/ludwig125/gke-stockprice/sheet"
)

// Status is struct to control spreadsheet.
type Status struct {
	Sheet sheet.Sheet
	//task  Task
}

// Task is struct to manage task status.
type Task struct {
	task     string
	unixtime int64
	jst      string
}

// ClearStatus clears spreadsheet status data.
func (s Status) ClearStatus() error {
	if err := s.Sheet.Clear(); err != nil {
		return fmt.Errorf("failed to clear sheet: %w", err)
	}
	return nil
}

// UpdateStatus updates spreadsheet status.
func (s Status) UpdateStatus(task string, t time.Time) error {
	status := [][]string{
		{task, fmt.Sprintf("%d", t.Unix()), t.Format("2006-01-02 15:04:05")},
	}

	if err := s.Sheet.Insert(status); err != nil {
		return fmt.Errorf("failed to sheet Update: %w", err)
	}
	return nil
}

// FetchStatus fetches spreadsheet status.
func (s Status) FetchStatus() ([][]string, error) {
	tasks, err := s.Sheet.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}
	return tasks, nil
}

func taskStatus(tasks [][]string, task string) (Task, error) {
	for i := len(tasks) - 1; i >= 0; i-- { // taskが新旧重複している可能性があるのでstatusの下の行から見ていく
		t := tasks[i]
		//fmt.Println("tasks status", tasks[i])
		if t[0] == task {
			u, err := strconv.Atoi(t[1])
			if err != nil {
				return Task{}, fmt.Errorf("failed to convert %s to int: %v", t[1], err)
			}
			return Task{task: t[0], unixtime: int64(u), jst: t[2]}, nil
		}
	}
	return Task{}, fmt.Errorf("failed to fetch task '%s' from status sheet", task)
}

// IsTaskDoneAfter returns true when task is done after u(midnight unixtime)
func (s Status) IsTaskDoneAfter(task string, u int64) (bool, error) {
	tasks, err := s.FetchStatus()
	if err != nil {
		return false, fmt.Errorf("failed to FetchStatus: %v", err)
	}
	if len(tasks) == 0 {
		log.Println("status is empty")
		return false, nil
	}
	t, err := taskStatus(tasks, task)
	if err != nil {
		log.Println("failed to taskStatus:", err)
		return false, nil
	}

	if t.unixtime < u { // 指定したUnixTimeよりもTaskの完了時刻が前であればFalse
		return false, nil
	}
	return true, nil
}

// ExecIfIncompleteThisDay executes task when it is not done this day.
func (s Status) ExecIfIncompleteThisDay(task string, thisTime time.Time, fn func() error) error {
	ok, err := s.IsTaskDoneAfter(task, thisTime.Unix())
	if err != nil {
		return fmt.Errorf("failed to IsTaskDoneAfter: %v", err)
	}
	if ok {
		return nil
	}
	//　taskがまだ完了済みでなければ実行
	if err := fn(); err != nil {
		return fmt.Errorf("failed to fn: %v", err)
	}

	if err := s.UpdateStatus(task, now()); err != nil {
		return fmt.Errorf("failed to UpdateStatus: %v", err)
	}
	return nil
}

// 与えられた時刻の0時0分0秒のUnixTimeを取得する
func getMidnightUnixtime(t time.Time) int64 {
	year, month, day := t.Date()
	m := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	return m.Unix()
}

var now = func() time.Time {
	return time.Now().In(time.Local)
}
