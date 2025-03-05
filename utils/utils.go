package utils

import (
	"encoding/json"
	"fmt"
	"log"
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
		Interval:   1,             // 1 hour work interval
		BreakTime:  1,             // 5-minute break
		WorkStatus: types.Working, // Initial status is working
	}
	alarmChan = make(chan bool)
	stopChan = make(chan bool)
	pauseChan = make(chan bool)
	resumeChan = make(chan bool)
}
func formatTimeReadable(t time.Time) string {
	return t.Format("Mon Jan 2 2006 at 3:04:05 PM")
}

// Calculate remaining time based on current state
func RunAlarm() {
	go func() {
		var pauseStart time.Time
		var remainingTime time.Duration
		var currentTimerStart time.Time

		// Initialize the timer when the alarm starts
		currentTimerStart = time.Now()
		alarmTimer = time.NewTimer(time.Duration(status.Interval) * time.Minute)

		for {
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

				// Calculate remaining time
				if alarmTimer != nil {
					alarmTimer.Stop()
					remainingTime = time.Duration(status.Interval)*time.Minute - time.Since(currentTimerStart)
				}
				statusMutex.Unlock()

			case <-resumeChan:
				statusMutex.Lock()
				if status.WorkStatus == types.Paused {
					pauseDuration := time.Since(pauseStart)

					// Adjust remaining time by subtracting pause duration
					if remainingTime > pauseDuration {
						remainingTime -= pauseDuration
					} else {
						remainingTime = 0
					}

					// Restart timer with remaining time
					currentTimerStart = time.Now()
					alarmTimer = time.NewTimer(remainingTime)
					status.WorkStatus = types.Working
				}
				statusMutex.Unlock()

			case <-alarmTimer.C:
				statusMutex.Lock()
				now := time.Now()

				if status.WorkStatus == types.Working {
					// Work interval completed, start break
					status.LastAlarm = now
					status.NextAlarm = now.Add(time.Duration(status.BreakTime) * time.Minute)
					status.WorkStatus = types.OnBreak

					// Trigger break alarm for specified duration
					breakEnd := now.Add(time.Duration(status.BreakTime) * time.Minute)

					statusMutex.Unlock()

					for time.Now().Before(breakEnd) {
						alarmChan <- true
						time.Sleep(1 * time.Second)
					}

					// Return to working state
					statusMutex.Lock()
					status.WorkStatus = types.Working

					// Reset timer for next work interval
					currentTimerStart = time.Now()
					alarmTimer = time.NewTimer(time.Duration(status.Interval) * time.Minute)
					statusMutex.Unlock()

					log.Printf("Break time over. Return to work! Current time: %s", formatTimeReadable(now))
					log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
				} else if status.WorkStatus == types.OnBreak {
					// Break completed, return to work
					status.WorkStatus = types.Working

					// Reset timer for work interval
					currentTimerStart = time.Now()
					alarmTimer = time.NewTimer(time.Duration(status.Interval) * time.Minute)
					statusMutex.Unlock()

					log.Printf("Break time over. Return to work! Current time: %s", formatTimeReadable(now))
				} else {
					statusMutex.Unlock()
				}
			}
		}
	}()
}

// Method for reset alarm
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

// Method for stop alarm
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

// Method for pause
func PauseAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running && status.WorkStatus != types.Paused {
		pauseChan <- true
		log.Printf("Alarm paused at: %s", formatTimeReadable(time.Now()))
	}

	json.NewEncoder(w).Encode(status)
}

// Method for resume alarm
func ResumeAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running && status.WorkStatus == types.Paused {
		resumeChan <- true
		log.Printf("Alarm resumed at: %s", formatTimeReadable(time.Now()))
	}

	json.NewEncoder(w).Encode(status)
}

// Method for reset alarm
func ResetAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	// If running, stop it first
	if status.Running {
		stopChan <- true
		status.Running = false
	}

	// Then start it again
	status.Running = true
	now := time.Now()
	status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)

	// Start the alarm goroutine
	go RunAlarm()

	log.Printf("Alarm reset at: %s", formatTimeReadable(now))
	log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
	json.NewEncoder(w).Encode(status)
}

// Method for getting current alarm status and time information
func GetStatus(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	json.NewEncoder(w).Encode(status)
}

// Method for updating the break time
func UpdateInterval(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Interval int64 `json:"interval"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	if requestData.Interval < 1 {
		http.Error(w, "Interval must be at least 1 minute", http.StatusBadRequest)
		return
	}

	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Update the interval
	status.Interval = requestData.Interval

	// If running, reset the timer
	if status.Running {
		stopChan <- true
		status.Running = true
		now := time.Now()
		status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)
		go RunAlarm()
	}

	log.Printf("Interval updated to %d minutes at: %s", requestData.Interval, formatTimeReadable(time.Now()))
	if status.Running {
		log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
	}

	json.NewEncoder(w).Encode(status)
}

// SSE endpoint for real-time alarm notifications
func AlarmEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel for client disconnection
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
