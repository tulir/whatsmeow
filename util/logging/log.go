package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ichabod-mo/whatsmeow/util/file"
)

type Logger interface {
	Setup()
	Warnf(args ...interface{})
	Errorf(args ...interface{})
	Infof(args ...interface{})
	Debugf(args ...interface{})
	Fatalf(args ...interface{})
}

type stdoutLogger struct{}

type Level int

var (
	F *os.File

	DefaultPrefix      = ""
	DefaultCallerDepth = 2

	logPrefix           = ""
	levelFlags          = []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	StdOutLogger Logger = &stdoutLogger{}
	logger       *log.Logger
)

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

// Setup initialize the log instance
func (s *stdoutLogger) Setup() {
	var err error
	filePath := file.GetLogFilePath()
	fileName := file.GetLogFileName()
	F, err = file.MustOpen(fileName, filePath)
	if err != nil {
		log.Fatalf("logging.Setup err: %v", err)
	}

	logger = log.New(F, DefaultPrefix, log.LstdFlags)
}

// Debug output logs at debug level
func (s *stdoutLogger) Debugf(v ...interface{}) {
	fmt.Printf("excute")
	setPrefix(DEBUG)
	logger.Println(v)
}

// Info output logs at info level
func (s *stdoutLogger) Infof(v ...interface{}) {
	setPrefix(INFO)
	logger.Println(v)
}

// Warn output logs at warn level
func (s *stdoutLogger) Warnf(v ...interface{}) {
	setPrefix(WARNING)
	logger.Println(v)
}

// Error output logs at error level
func (s *stdoutLogger) Errorf(v ...interface{}) {
	setPrefix(ERROR)
	logger.Println(v)
}

// Fatal output logs at fatal level
func (s *stdoutLogger) Fatalf(v ...interface{}) {
	setPrefix(FATAL)
	logger.Fatalln(v)
}

// setPrefix set the prefix of the log output
func setPrefix(level Level) {
	// 检查日志目录
	filePath := file.GetLogFilePath()
	fileName := file.GetLogFileName()
	_, err := os.Stat(filePath + fileName)
	if os.IsNotExist(err) {
		//若日志目录不存在,则创建
		F, err = file.MustOpen(fileName, filePath)
		if err != nil {
			log.Fatalf("logging.Setup err: %v", err)
		}
		logger = log.New(F, DefaultPrefix, log.LstdFlags)
	}

	_, file, line, ok := runtime.Caller(DefaultCallerDepth)
	if ok {
		logPrefix = fmt.Sprintf("[%s][%s:%d]", levelFlags[level], filepath.Base(file), line)
	} else {
		logPrefix = fmt.Sprintf("[%s]", levelFlags[level])
	}

	logger.SetPrefix(logPrefix)
}
