package status

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/ludwig125/gke-stockprice/date"
	"github.com/ludwig125/gke-stockprice/sheet"
)

// Status is struct to control spreadsheet.
type Status struct {
	Sheet sheet.Sheet
	//task  Task
}

// Task is struct to manage task status.
type Task struct {
	task       string
	unixtime   int64
	jst        string
	turnaround string
}

// ClearStatus clears spreadsheet status data.
func (s Status) ClearStatus() error {
	if err := s.Sheet.Clear(); err != nil {
		return fmt.Errorf("failed to clear sheet: %w", err)
	}
	return nil
}

// InsertStatus updates spreadsheet status.
func (s Status) InsertStatus(task string, t time.Time, t2 time.Duration) error {
	status := [][]string{
		{task, fmt.Sprintf("%d", t.Unix()), t.Format("2006-01-02 15:04:05"), fmt.Sprintf("%v", t2)},
	}

	if err := s.Sheet.Insert(status); err != nil {
		return fmt.Errorf("failed to sheet Update: %w", err)
	}
	return nil
}

// FetchStatus fetches spreadsheet status all tasks.
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

		if t[0] != task {
			continue
		}
		u, err := strconv.Atoi(t[1])
		if err != nil {
			return Task{}, fmt.Errorf("failed to convert %s to int: %v", t[1], err)
		}

		var jst, turnaround string
		if len(t) >= 4 {
			turnaround = t[3]
		}
		if len(t) >= 3 {
			jst = t[2]
		}

		return Task{task: t[0], unixtime: int64(u), jst: jst, turnaround: turnaround}, nil
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
	start := time.Now()
	// thisTimeの日の0時0分0秒より後にtaskが完了したかどうかを確認する
	midnight, err := getLocalMidnightUnixTime(thisTime)
	if err != nil {
		return fmt.Errorf("failed to getLocalMidnightUnixTime: %v", err)
	}
	ok, err := s.IsTaskDoneAfter(task, midnight)
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

	turnaround := time.Since(start)
	// InsertStatusにはtask名、現在時刻（thisTImeとして渡された時刻＋所要時間）、所要時間（turnaround）を設定
	if err := s.InsertStatus(task, thisTime.Add(turnaround), turnaround); err != nil {
		return fmt.Errorf("failed to InsertStatus: %v", err)
	}
	return nil
}

func getLocalMidnightUnixTime(t time.Time) (int64, error) {
	t, err := date.GetMidnight(t, "Asia/Tokyo")
	if err != nil {
		return 0, fmt.Errorf("failed to GetMidnight")
	}
	return t.Unix(), nil
}
