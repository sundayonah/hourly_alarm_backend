package types

import "time"

// WorkStatus represents the current work/break state
type WorkStatus string

const (
	Working WorkStatus = "working"
	OnBreak WorkStatus = "on_break"
	Stopped WorkStatus = "stopped"
	Paused  WorkStatus = "paused"
)

// AlarmStatus represents the current state of the alarm
type AlarmStatus struct {
	Running       bool       `json:"running"`
	NextAlarm     time.Time  `json:"nextAlarm"`
	LastAlarm     time.Time  `json:"lastAlarm"`
	Interval      int64      `json:"interval"`
	BreakTime     int64      `json:"breakTime"`
	WorkStatus    WorkStatus `json:"workStatus"`
	RemainingTime int64      `json:"remainingTime"`
}
