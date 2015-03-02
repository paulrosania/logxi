package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/mattn/go-colorable"
	"github.com/mgutz/ansi"
	"gopkg.in/stack.v1"
)

// Theme defines a color theme for HappyDevFormatter
type colorScheme struct {
	Key   string
	Value string

	Debug string
	Info  string
	Warn  string
	Error string
	Reset string
}

var theme *colorScheme

func processThemeEnv() {
	colors := os.Getenv("LOGXI_COLORS")
	if colors == "" {
		colors = DarkScheme
	}
	theme = parseTheme(colors)
}

// DarkScheme is a colors scheme for dark backgrounds
var DarkScheme = "key=cyan+h,value,DBG,WRN=yellow+h,INF=green+h,ERR=red+h"

// LightScheme is a color scheme for light backgrounds
var LightScheme = "key=cyan+b,value,DBG,WRN=yellow+b,INF=green+b,ERR=red+b"

func parseKVList(s, separator string) map[string]string {
	pairs := strings.Split(s, separator)
	if len(pairs) == 0 {
		return nil
	}
	m := map[string]string{}
	for _, pair := range pairs {
		if pair == "" {
			continue
		}
		parts := strings.Split(pair, "=")
		lenParts := len(parts)
		if lenParts == 1 {
			m[parts[0]] = ""
		} else if lenParts == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func parseTheme(theme string) *colorScheme {
	m := parseKVList(theme, ",")
	return &colorScheme{
		Key:   ansi.ColorCode(m["key"]),
		Value: ansi.ColorCode(m["value"]),
		Debug: ansi.ColorCode(m["DBG"]),
		Warn:  ansi.ColorCode(m["WRN"]),
		Info:  ansi.ColorCode(m["INF"]),
		Error: ansi.ColorCode(m["ERR"]),
		Reset: ansi.ColorCode("reset"),
	}
}

func keyColor(s string) string {
	return theme.Key + s + theme.Reset
}

// DisableColors disables coloring of log entries.
func DisableColors(val bool) {
	disableColors = val
}

// GetColorableStdout gets a writer that can output colors
// on Windows and non-Widows OS. If colors are disabled,
// os.Stdout is returned.
func GetColorableStdout() io.Writer {
	if isTTY && !disableColors {
		return colorable.NewColorableStdout()
	}
	return os.Stdout
}

// HappyDevFormatter is the default recorder used if one is unspecified when
// creating a new Logger.
type HappyDevFormatter struct {
	name         string
	itoaLevelMap map[int]string
}

// NewHappyDevFormatter returns a new instance of HappyDevFormatter.
// Performance isn't priority. It's more important developers see errors
// and stack.
func NewHappyDevFormatter(name string) *HappyDevFormatter {
	var buildKV = func(level string) string {
		var buf bytes.Buffer

		buf.WriteString(Separator)
		buf.WriteString(theme.Key)
		buf.WriteString("n=")
		buf.WriteString(theme.Reset)
		buf.WriteString(name)

		buf.WriteString(Separator)
		buf.WriteString(theme.Key)
		buf.WriteString("l=")
		buf.WriteString(theme.Reset)
		buf.WriteString(level)

		buf.WriteString(Separator)
		buf.WriteString(theme.Key)
		buf.WriteString("n=")
		buf.WriteString(theme.Reset)
		buf.WriteString(level)

		buf.WriteString(Separator)
		buf.WriteString(theme.Key)
		buf.WriteString("m=")
		buf.WriteString(theme.Reset)

		return buf.String()
	}
	itoaLevelMap := map[int]string{
		LevelDebug: buildKV(LevelMap[LevelDebug]),
		LevelWarn:  buildKV(LevelMap[LevelWarn]),
		LevelInfo:  buildKV(LevelMap[LevelInfo]),
		LevelError: buildKV(LevelMap[LevelError]),
		LevelFatal: buildKV(LevelMap[LevelFatal]),
	}
	return &HappyDevFormatter{itoaLevelMap: itoaLevelMap, name: name}
}

func (tf *HappyDevFormatter) writeKey(buf *bytes.Buffer, key string) {
	// assumes this is not the first key
	buf.WriteString(Separator)
	buf.WriteString(theme.Key)
	buf.WriteString(key)
	buf.WriteRune('=')
	buf.WriteString(theme.Reset)
}

func (tf *HappyDevFormatter) writeError(buf *bytes.Buffer, err *errors.Error) {
	buf.WriteString(theme.Error)
	buf.WriteString(err.Error())
	buf.WriteRune('\n')
	buf.Write(err.Stack())
	buf.WriteString(theme.Reset)
}

func (tf *HappyDevFormatter) set(buf *bytes.Buffer, key string, value interface{}, colorCode string) {
	tf.writeKey(buf, key)
	if colorCode != "" {
		buf.WriteString(colorCode)
	}
	if err, ok := value.(error); ok {
		err2 := errors.Wrap(err, 4)
		tf.writeError(buf, err2)
	} else if err, ok := value.(*errors.Error); ok {
		tf.writeError(buf, err)
	} else {
		fmt.Fprintf(buf, "%v", value)
	}
	if colorCode != "" {
		buf.WriteString(theme.Reset)
	}
}

// Format records a log entry.
func (tf *HappyDevFormatter) Format(buf *bytes.Buffer, level int, msg string, args []interface{}) {
	buf.WriteString(keyColor("t="))
	buf.WriteString(time.Now().Format("2006-01-02T15:04:05.000000"))

	tf.set(buf, "n", tf.name, theme.Value)

	var colorCode string
	var context string

	switch level {
	case LevelDebug:
		colorCode = theme.Debug
	case LevelInfo:
		colorCode = theme.Info
	case LevelWarn:
		c := stack.Caller(2)
		context = fmt.Sprintf("%+v", c)
		colorCode = theme.Warn
	default:
		trace := stack.Trace().TrimRuntime()

		// if one line, keep it on same line, multiple lines group all
		// on next line
		var errbuf bytes.Buffer
		lines := 0
		for i, stack := range trace {
			if i < 3 {
				continue
			}
			if i == 3 && len(trace) > 4 {
				errbuf.WriteString("\n\t")
			} else if i > 3 {
				errbuf.WriteString("\n\t")
			}
			errbuf.WriteString(fmt.Sprintf("%+v", stack))
			lines++
		}
		if lines > 1 {
			errbuf.WriteRune('\n')
		}

		context = errbuf.String()
		colorCode = theme.Error
	}
	tf.set(buf, "l", LevelMap[level], colorCode)
	tf.set(buf, "m", msg, colorCode)
	if context != "" {
		tf.set(buf, "c", context, colorCode)
	}

	var lenArgs = len(args)
	if lenArgs > 0 {
		if lenArgs%2 == 0 {
			for i := 0; i < lenArgs; i += 2 {
				if key, ok := args[i].(string); ok {
					tf.set(buf, key, args[i+1], theme.Value)
				} else {
					tf.set(buf, "BADKEY_NAME_"+strconv.Itoa(i+1), args[i], theme.Error)
					tf.set(buf, "BADKEY_VALUE_"+strconv.Itoa(i+1), args[i+1], theme.Error)
				}
			}
		} else {
			buf.WriteString(theme.Error)
			buf.WriteString(Separator)
			buf.WriteString("IMBALANCED_PAIRS=>")
			buf.WriteString(theme.Warn)
			fmt.Fprint(buf, args...)
			buf.WriteString(theme.Reset)
		}
	}
	buf.WriteRune('\n')
}
