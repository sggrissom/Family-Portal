package backend

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type logLevel string

const (
	logLevelInfo  logLevel = "INFO"
	logLevelWarn  logLevel = "WARN"
	logLevelError logLevel = "ERROR"
	logLevelDebug logLevel = "DEBUG"
)

type logCategory string

const (
	logCategoryAuth   logCategory = "AUTH"
	logCategoryPhoto  logCategory = "PHOTO"
	logCategoryAdmin  logCategory = "ADMIN"
	logCategoryAPI    logCategory = "API"
	logCategoryWorker logCategory = "WORKER"
	logCategorySystem logCategory = "SYSTEM"
	logCategoryImport logCategory = "IMPORT"
)

type logEntry struct {
	Timestamp       time.Time   `json:"timestamp"`
	Level           logLevel    `json:"level"`
	Category        logCategory `json:"category"`
	Message         string      `json:"message"`
	Data            interface{} `json:"data,omitempty"`
	UserID          *int        `json:"userId,omitempty"`
	IP              string      `json:"ip,omitempty"`
	UserAgent       string      `json:"userAgent,omitempty"`
	Duration        *int        `json:"duration,omitempty"`
	HandlerDuration *int        `json:"handlerDuration,omitempty"`
	HTTPMethod      string      `json:"httpMethod,omitempty"`
	HTTPPath        string      `json:"httpPath,omitempty"`
	HTTPStatus      *int        `json:"httpStatus,omitempty"`
	StackTrace      string      `json:"-"`
}

func logStructured(level logLevel, category logCategory, message string, data interface{}, r *http.Request) {
	entry := logEntry{
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   message,
		Data:      data,
	}

	if r != nil {
		entry.IP = getClientIP(r)
		entry.UserAgent = r.Header.Get("User-Agent")

		if user, ok := GetUserFromContext(r); ok {
			entry.UserID = &user.Id
		}
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[%s] [%s] %s - JSON marshal error: %v", level, category, message, err)
		return
	}

	log.Println(string(jsonBytes))
}

const (
	LogCategoryAuth   = "AUTH"
	LogCategoryPhoto  = "PHOTO"
	LogCategoryAdmin  = "ADMIN"
	LogCategoryAPI    = "API"
	LogCategoryWorker = "WORKER"
	LogCategorySystem = "SYSTEM"
)

func LogInfo(category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelInfo, logCategory(category), message, d, nil)
}

func LogInfoWithRequest(r *http.Request, category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelInfo, logCategory(category), message, d, r)
}

func LogWarn(category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelWarn, logCategory(category), message, d, nil)
}

func LogWarnWithRequest(r *http.Request, category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelWarn, logCategory(category), message, d, r)
}

func LogErrorSimple(category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelError, logCategory(category), message, d, nil)
}

func LogErrorWithRequest(r *http.Request, category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelError, logCategory(category), message, d, r)
}

func LogDebug(category string, message string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	logStructured(logLevelDebug, logCategory(category), message, d, nil)
}

func redactEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		if email == "" {
			return ""
		}
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		if idx := len(xff); idx > 0 {
			if commaIdx := 0; commaIdx < idx {
				for i, c := range xff {
					if c == ',' {
						commaIdx = i
						break
					}
				}
				if commaIdx > 0 {
					return xff[:commaIdx]
				}
			}
			return xff
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	return r.RemoteAddr
}
