package logger

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	Logger     *logrus.Logger // Main application logger (to file)
	FileLogger *logrus.Logger // File logger for application logs
)

// Initialize sets up the loggers with proper configuration
func Initialize() {
	// Purge log files on startup
	purgeLogFiles()

	// Set up file logger for application logs
	FileLogger = logrus.New()

	// Set log level based on environment
	logLevel := os.Getenv("LOG_LEVEL")
	var level logrus.Level
	switch logLevel {
	case "DEBUG":
		level = logrus.DebugLevel
	case "INFO":
		level = logrus.InfoLevel
	case "WARN":
		level = logrus.WarnLevel
	case "ERROR":
		level = logrus.ErrorLevel
	default:
		level = logrus.InfoLevel
	}

	// Configure file logger (application logs to file)
	FileLogger.SetLevel(level)
	FileLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		ForceColors:     false, // No colors in file
		DisableColors:   true,
	})

	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
		return
	}

	// Open log file
	logFile, err := os.OpenFile(
		fmt.Sprintf("%s/autolog.log", logsDir),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}

	FileLogger.SetOutput(logFile)
	FileLogger.SetReportCaller(true)

	// Set main logger to use file logger for application logs
	Logger = FileLogger

	Logger.Info("Logging system initialized", map[string]interface{}{
		"api_logs":  "stdout (simple text)",
		"app_logs":  "file",
		"log_level": level.String(),
		"log_file":  fmt.Sprintf("%s/autolog.log", logsDir),
	})
}

// GetLogger returns the configured main logger instance
func GetLogger() *logrus.Logger {
	if Logger == nil {
		Initialize()
	}
	return Logger
}

// WithContext creates a logger with additional context fields
func WithContext(fields map[string]interface{}) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// WithLogFile creates a logger with log file context
func WithLogFile(logFileID uint, filename string) *logrus.Entry {
	return GetLogger().WithFields(logrus.Fields{
		"log_file_id": logFileID,
		"filename":    filename,
		"component":   "log_processor",
	})
}

// WithJob creates a logger with job context
func WithJob(jobID uint, jobType string) *logrus.Entry {
	return GetLogger().WithFields(logrus.Fields{
		"job_id":    jobID,
		"job_type":  jobType,
		"component": "job_service",
	})
}

// WithLLM creates a logger with LLM service context
func WithLLM(logFileID *uint, jobID *uint, callType string) *logrus.Entry {
	fields := logrus.Fields{
		"component": "llm_service",
		"call_type": callType,
	}
	if logFileID != nil {
		fields["log_file_id"] = *logFileID
	}
	if jobID != nil {
		fields["job_id"] = *jobID
	}
	return GetLogger().WithFields(fields)
}

// WithUser creates a logger with user context
func WithUser(userID uint) *logrus.Entry {
	return GetLogger().WithFields(logrus.Fields{
		"user_id":   userID,
		"component": "controller",
	})
}

// WithError creates a logger with error context
func WithError(err error, component string) *logrus.Entry {
	fields := logrus.Fields{
		"error":     err.Error(),
		"component": component,
	}

	// Add stack trace for debug level
	if Logger.GetLevel() >= logrus.DebugLevel {
		fields["stack_trace"] = getStackTrace()
	}

	return GetLogger().WithFields(fields)
}

// getStackTrace returns a formatted stack trace
func getStackTrace() string {
	var stack []string
	for i := 1; i < 10; i++ {
		if pc, file, line, ok := runtime.Caller(i); ok {
			fn := runtime.FuncForPC(pc)
			stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
		}
	}
	return strings.Join(stack, "\n")
}

// Log levels convenience functions (with fields) - Application logs
func Debug(msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	GetLogger().WithFields(fields).Debug(msg)
}

func Info(msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	GetLogger().WithFields(fields).Info(msg)
}

func Warn(msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	GetLogger().WithFields(fields).Warn(msg)
}

func Error(msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	GetLogger().WithFields(fields).Error(msg)
}

func Fatal(msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	GetLogger().WithFields(fields).Fatal(msg)
}

// Simple convenience functions (without fields) - Application logs
func Debugf(msg string) {
	GetLogger().Debug(msg)
}

func Infof(msg string) {
	GetLogger().Info(msg)
}

func Warnf(msg string) {
	GetLogger().Warn(msg)
}

func Errorf(msg string) {
	GetLogger().Error(msg)
}

func Fatalf(msg string) {
	GetLogger().Fatal(msg)
}

// purgeLogFiles removes all log files on startup to keep the system clean
func purgeLogFiles() {
	// Purge application logs
	purgeDirectory("logs", []string{".log", ".tmp"})

	// Purge uploaded log files
	purgeDirectory("uploads/logs", []string{".json", ".txt", ".log", ".csv", ".xml", ".yaml", ".yml"})

	fmt.Println("✅ All log files purged successfully")
}

// purgeDirectory removes files with specified extensions from a directory
func purgeDirectory(dirPath string, extensions []string) {
	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		// Directory doesn't exist, nothing to purge
		return
	}

	// Read all files in the directory
	files, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Printf("Failed to read directory %s: %v\n", dirPath, err)
		return
	}

	// Remove files with matching extensions
	purgedCount := 0
	for _, file := range files {
		if !file.IsDir() {
			fileName := file.Name()
			shouldPurge := false

			for _, ext := range extensions {
				if strings.HasSuffix(fileName, ext) {
					shouldPurge = true
					break
				}
			}

			if shouldPurge {
				filePath := fmt.Sprintf("%s/%s", dirPath, fileName)
				if err := os.Remove(filePath); err != nil {
					fmt.Printf("Failed to remove file %s: %v\n", filePath, err)
				} else {
					fmt.Printf("Purged: %s\n", filePath)
					purgedCount++
				}
			}
		}
	}

	if purgedCount > 0 {
		fmt.Printf("✅ Purged %d files from %s\n", purgedCount, dirPath)
	}
}

// PurgeAllLogs is a public function that can be called manually to purge all log files
func PurgeAllLogs() {
	purgeLogFiles()
}
