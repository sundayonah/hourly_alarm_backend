package types

import "time"

// AlarmStatus represents the current state of the alarm
type AlarmStatus struct {
	Running   bool      `json:"running"`
	NextAlarm time.Time `json:"nextAlarm"`
	LastAlarm time.Time `json:"lastAlarm"`
	Interval  int64     `json:"interval"` // in minutes
}
