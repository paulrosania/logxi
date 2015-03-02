package log

import (
	"os"
	"sync"

	"github.com/mattn/go-isatty"
)

// DefaultLogLog is the default log for this package.
var DefaultLog Logger
var internalLog Logger

// Whether to force disabling of Colors
var disableColors bool

type loggerMap struct {
	sync.Mutex
	loggers map[string]Logger
}

var loggers = &loggerMap{
	loggers: map[string]Logger{},
}

func (lm *loggerMap) set(name string, logger Logger) {
	lm.loggers[name] = logger
}

// logxiEnabledMap maps log name patterns to levels
var logxiNameLevelMap map[string]int

// logxiFormat is the formatter kind to create
var logxiFormat string

var isTTY bool

func init() {
	isTTY = isatty.IsTerminal(os.Stdout.Fd())
	disableColors = !isTTY

	processEnv()
	DefaultLog = New("~")
	internalLog = New("logxi")
}
