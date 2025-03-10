package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/sundayonah/hourly_alarm/types"
)

var (
	status      types.AlarmStatus
	alarmChan   chan bool
	stopChan    chan bool
	pauseChan   chan bool
	resumeChan  chan bool
	statusMutex sync.Mutex
	alarmTimer  *time.Timer
)

func init() {
	status = types.AlarmStatus{
		Running:    false,
		Interval:   2,             // 5-minute work interval
		BreakTime:  1,             // 1-minute break with alarm
		WorkStatus: types.Stopped, // Initial status is stopped
	}
	alarmChan = make(chan bool)
	stopChan = make(chan bool)
	pauseChan = make(chan bool)
	resumeChan = make(chan bool)
}

func formatTimeReadable(t time.Time) string {
	return t.Format("Mon Jan 2 2006 at 3:04:05 PM")
}

func RunAlarm() {
	go func() {
		var pauseStart time.Time
		var remainingTime time.Duration
		var currentTimerStart time.Time

		for {
			statusMutex.Lock()
			if !status.Running {
				statusMutex.Unlock()
				return
			}
			// Start work period
			currentTimerStart = time.Now()
			alarmTimer = time.NewTimer(time.Duration(status.Interval) * time.Minute)
			status.WorkStatus = types.Working
			status.NextAlarm = currentTimerStart.Add(time.Duration(status.Interval) * time.Minute)
			statusMutex.Unlock()

			select {
			case <-stopChan:
				statusMutex.Lock()
				status.Running = false
				status.WorkStatus = types.Stopped
				if alarmTimer != nil {
					alarmTimer.Stop()
				}
				statusMutex.Unlock()
				return

			case <-pauseChan:
				statusMutex.Lock()
				pauseStart = time.Now()
				status.WorkStatus = types.Paused
				if alarmTimer != nil {
					alarmTimer.Stop()
					remainingTime = time.Duration(status.Interval)*time.Minute - time.Since(currentTimerStart)
				}
				statusMutex.Unlock()

			case <-resumeChan:
				statusMutex.Lock()
				if status.WorkStatus == types.Paused {
					pauseDuration := time.Since(pauseStart)
					if remainingTime > pauseDuration {
						remainingTime -= pauseDuration
					} else {
						remainingTime = 0
					}
					currentTimerStart = time.Now()
					alarmTimer = time.NewTimer(remainingTime)
					status.WorkStatus = types.Working
					status.NextAlarm = currentTimerStart.Add(remainingTime)
				}
				statusMutex.Unlock()

			case <-alarmTimer.C:
				statusMutex.Lock()
				now := time.Now()
				status.LastAlarm = now
				status.NextAlarm = now.Add(time.Duration(status.BreakTime) * time.Minute)
				status.WorkStatus = types.OnBreak
				breakEnd := now.Add(time.Duration(status.BreakTime) * time.Minute)
				statusMutex.Unlock()

				// Trigger alarm for 1 minute
				for time.Now().Before(breakEnd) {
					alarmChan <- true
					time.Sleep(250 * time.Millisecond) // Send alarm signal 4 times per second
				}

				// Transition back to work
				statusMutex.Lock()
				now = time.Now()
				status.WorkStatus = types.Working
				currentTimerStart = now
				status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)
				alarmTimer = time.NewTimer(time.Duration(status.Interval) * time.Minute)
				statusMutex.Unlock()

				log.Printf("Break ended. Back to work! Current time: %s", formatTimeReadable(now))
				log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
			}
		}
	}()
}

// Method for starting alarm
func StartAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if !status.Running {
		status.Running = true
		status.WorkStatus = types.Working
		now := time.Now()
		status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)
		go RunAlarm()
		log.Printf("Alarm started at: %s", formatTimeReadable(now))
		log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
	}
	json.NewEncoder(w).Encode(status)
}

// Method for stopping alarm
func StopAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running {
		stopChan <- true
		status.Running = false
		status.WorkStatus = types.Stopped
		log.Printf("Alarm stopped at: %s", formatTimeReadable(time.Now()))
	}
	json.NewEncoder(w).Encode(status)
}

// Method for pausing alarm
func PauseAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running && status.WorkStatus != types.Paused {
		pauseChan <- true
		log.Printf("Alarm paused at: %s", formatTimeReadable(time.Now()))
	}
	json.NewEncoder(w).Encode(status)
}

// Method for resuming alarm
func ResumeAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running && status.WorkStatus == types.Paused {
		resumeChan <- true
		log.Printf("Alarm resumed at: %s", formatTimeReadable(time.Now()))
	}
	json.NewEncoder(w).Encode(status)
}

// Method for resetting alarm
func ResetAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running {
		stopChan <- true
	}
	status.Running = true
	now := time.Now()
	status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)
	status.WorkStatus = types.Working
	go RunAlarm()
	log.Printf("Alarm reset at: %s", formatTimeReadable(now))
	log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
	json.NewEncoder(w).Encode(status)
}

// Method for getting current alarm status
func GetStatus(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running && status.WorkStatus != types.Paused {
		remaining := time.Until(status.NextAlarm)
		status.RemainingTime = int64(math.Round(remaining.Seconds())) // Use seconds for finer granularity
	} else {
		status.RemainingTime = 0
	}
	json.NewEncoder(w).Encode(status)
}

// Method for updating interval
func UpdateInterval(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Interval  *int64 `json:"interval,omitempty"`  // Optional field
		BreakTime *int64 `json:"breakTime,omitempty"` // Optional field
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Update interval if provided
	if requestData.Interval != nil {
		if *requestData.Interval < 1 || *requestData.Interval > 1440 {
			http.Error(w, "Interval must be between 1 and 1440 minutes", http.StatusBadRequest)
			return
		}
		status.Interval = *requestData.Interval
	}

	// Update breakTime if provided
	if requestData.BreakTime != nil {
		if *requestData.BreakTime < 1 || *requestData.BreakTime > 1440 {
			http.Error(w, "Break time must be between 1 and 1440 minutes", http.StatusBadRequest)
			return
		}
		status.BreakTime = *requestData.BreakTime
	}

	// If the alarm is running, restart it with new settings
	if status.Running {
		stopChan <- true
		status.Running = true
		now := time.Now()
		status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)
		status.WorkStatus = types.Working
		go RunAlarm()
	}

	log.Printf("Settings updated - Interval: %d minutes, Break: %d minutes at: %s",
		status.Interval,
		status.BreakTime,
		formatTimeReadable(time.Now()))
	json.NewEncoder(w).Encode(status)
}

// SSE endpoint for real-time alarm notifications
func AlarmEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientGone := r.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case <-alarmChan:
			now := time.Now()
			fmt.Fprintf(w, "data: {\"alarm\": true, \"time\": \"%s\", \"readableTime\": \"%s\"}\n\n",
				now.Format(time.RFC3339),
				formatTimeReadable(now))
			w.(http.Flusher).Flush()
		}
	}
}
