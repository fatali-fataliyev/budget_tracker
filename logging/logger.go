package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/subosito/gotenv"
)

var Logger *logrus.Logger

func Init(level string) error {
	Logger = logrus.New()
	err := gotenv.Load()
	if err != nil {
		return fmt.Errorf("failed to load env variables: %w", err)
	}

	appEnv := strings.ToLower(os.Getenv("APP_ENV"))

	//default environment is development
	if appEnv == "" {
		appEnv = "development"
	}
	if appEnv == "production" {
		Logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	level = strings.ToLower(level)

	switch level {
	case "debug":
		Logger.SetLevel(logrus.DebugLevel)
	case "info":
		Logger.SetLevel(logrus.InfoLevel)
	case "warning":
		Logger.SetLevel(logrus.WarnLevel)
	case "error":
		Logger.SetLevel(logrus.ErrorLevel)
	default:
		Logger.SetLevel(logrus.InfoLevel)
	}
	currentDate := time.Now().Format("02_01_2006")
	logDir := "./logging/logs"
	logFileName := currentDate + ".log"
	fullPath := filepath.Join(logDir, logFileName)

	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	Logger.SetOutput(io.MultiWriter(os.Stdout, file))
	return nil
}
