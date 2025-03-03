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
	statusMutex sync.Mutex
)

func init() {
	status = types.AlarmStatus{
		Running:  false,
		Interval: 60, // Default to 1 minute for testing
	}
	alarmChan = make(chan bool)
	stopChan = make(chan bool)
}

func formatTimeReadable(t time.Time) string {
	return t.Format("Mon Jan 2 2006 at 3:04:05 PM")
}

func RunAlarm() {
	go func() {
		for {
			select {
			case <-stopChan:
				statusMutex.Lock()
				status.Running = false
				statusMutex.Unlock()
				return
			case <-time.After(time.Duration(status.Interval) * time.Minute):
				statusMutex.Lock()
				now := time.Now()
				status.LastAlarm = now
				status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)
				statusMutex.Unlock()

				// Trigger alarm
				alarmChan <- true

				// Print to console with human readable time
				log.Printf("BEEP! Time to take a break! Current time: %s", formatTimeReadable(now))
				log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
			}
		}
	}()
}

func StartAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if !status.Running {
		status.Running = true
		now := time.Now()
		status.NextAlarm = now.Add(time.Duration(status.Interval) * time.Minute)

		// Start the alarm goroutine
		go RunAlarm()

		log.Printf("Alarm started at: %s", formatTimeReadable(now))
		log.Printf("Next alarm scheduled for: %s", formatTimeReadable(status.NextAlarm))
	}

	json.NewEncoder(w).Encode(status)
}

func StopAlarm(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	if status.Running {
		stopChan <- true
		status.Running = false
		log.Printf("Alarm stopped at: %s", formatTimeReadable(time.Now()))
	}

	json.NewEncoder(w).Encode(status)
}

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

func GetStatus(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	json.NewEncoder(w).Encode(status)
}

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
