package routes

import (
	"net/http"

	"github.com/sundayonah/hourly_alarm/middleware"
	"github.com/sundayonah/hourly_alarm/utils"
)

func Routes() {
	// Setup CORS middleware
	corsMiddleware := middleware.CORS

	// Setup API routes
	http.Handle("/api/start", corsMiddleware(http.HandlerFunc(utils.StartAlarm)))
	http.Handle("/api/stop", corsMiddleware(http.HandlerFunc(utils.StopAlarm)))
	http.Handle("/api/reset", corsMiddleware(http.HandlerFunc(utils.ResetAlarm)))
	http.Handle("/api/status", corsMiddleware(http.HandlerFunc(utils.GetStatus)))
	http.Handle("/api/interval", corsMiddleware(http.HandlerFunc(utils.UpdateInterval)))
	http.Handle("/api/events", corsMiddleware(http.HandlerFunc(utils.AlarmEvents)))
	http.Handle("/api/pause", corsMiddleware(http.HandlerFunc(utils.PauseAlarm)))
	http.Handle("/api/resume", corsMiddleware(http.HandlerFunc(utils.ResumeAlarm)))
}
