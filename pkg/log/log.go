package log

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bilibili/Kratos/pkg/conf/env"
)

// Config log config.
type Config struct {
	Family string
	Host   string

	// stdout
	Stdout bool

	// file
	Dir string
	// buffer size
	FileBufferSize int64
	// MaxLogFile
	MaxLogFile int
	// RotateSize
	RotateSize int64

	// V Enable V-leveled logging at the specified level.
	V int32
	// Module=""
	// The syntax of the argument is a map of pattern=N,
	// where pattern is a literal file name (minus the ".go" suffix) or
	// "glob" pattern and N is a V level. For instance:
	// [module]
	//   "service" = 1
	//   "dao*" = 2
	// sets the V level to 2 in all Go files whose names begin "dao".
	Module map[string]int32
	// Filter tell log handler which field are sensitive message, use * instead.
	Filter []string
}

var (
	h Handler
	c *Config
)

func init() {
	addFlag(flag.CommandLine)
}

var (
	_v      int
	_stdout bool
	_dir    string
	_filter logFilter
	_module = verboseModule{}
)

// addFlag init log from dsn.
func addFlag(fs *flag.FlagSet) {
	if lv, err := strconv.ParseInt(os.Getenv("LOG_V"), 10, 64); err == nil {
		_v = int(lv)
	}
	_stdout, _ = strconv.ParseBool(os.Getenv("LOG_STDOUT"))
	_dir = os.Getenv("LOG_DIR")
	if tm := os.Getenv("LOG_MODULE"); len(tm) > 0 {
		_module.Set(tm)
	}
	if tf := os.Getenv("LOG_FILTER"); len(tf) > 0 {
		_filter.Set(tf)
	}
	// get val from flag
	fs.IntVar(&_v, "log.v", _v, "log verbose level, or use LOG_V env variable.")
	fs.BoolVar(&_stdout, "log.stdout", _stdout, "log enable stdout or not, or use LOG_STDOUT env variable.")
	fs.StringVar(&_dir, "log.dir", _dir, "log file `path, or use LOG_DIR env variable.")
	fs.Var(&_module, "log.module", "log verbose for specified module, or use LOG_MODULE env variable, format: file=1,file2=2.")
	fs.Var(&_filter, "log.filter", "log field for sensitive message, or use LOG_FILTER env variable, format: field1,field2.")
}

// Init create logger with context.
func Init(conf *Config) {
	var isNil bool
	if conf == nil {
		isNil = true
		conf = &Config{
			Stdout: _stdout,
			Dir:    _dir,
			V:      int32(_v),
			Module: _module,
			Filter: _filter,
		}
	}
	if len(env.AppID) != 0 {
		conf.Family = env.AppID // for caster
	}
	conf.Host = env.Hostname
	if len(conf.Host) == 0 {
		host, _ := os.Hostname()
		conf.Host = host
	}
	var hs []Handler
	// when env is dev
	if isNil || conf.Stdout {
		hs = append(hs, NewStdout())
	}
	if conf.Dir != "" {
		hs = append(hs, NewFile(conf.Dir, conf.FileBufferSize, conf.RotateSize, conf.MaxLogFile))
	}
	h = newHandlers(conf.Filter, hs...)
	c = conf
}

// D represents a map of entry level data used for structured logging.
// type D map[string]interface{}
type D struct {
	Key   string
	Value interface{}
}

// AddTo exports a field through the ObjectEncoder interface. It's primarily
// useful to library authors, and shouldn't be necessary in most applications.
func (d D) AddTo(enc core.ObjectEncoder) {
	var err error
	switch val := d.Value.(type) {
	case bool:
		enc.AddBool(d.Key, val)
	case complex128:
		enc.AddComplex128(d.Key, val)
	case complex64:
		enc.AddComplex64(d.Key, val)
	case float64:
		enc.AddFloat64(d.Key, val)
	case float32:
		enc.AddFloat32(d.Key, val)
	case int:
		enc.AddInt(d.Key, val)
	case int64:
		enc.AddInt64(d.Key, val)
	case int32:
		enc.AddInt32(d.Key, val)
	case int16:
		enc.AddInt16(d.Key, val)
	case int8:
		enc.AddInt8(d.Key, val)
	case string:
		enc.AddString(d.Key, val)
	case uint:
		enc.AddUint(d.Key, val)
	case uint64:
		enc.AddUint64(d.Key, val)
	case uint32:
		enc.AddUint32(d.Key, val)
	case uint16:
		enc.AddUint16(d.Key, val)
	case uint8:
		enc.AddUint8(d.Key, val)
	case []byte:
		enc.AddByteString(d.Key, val)
	case uintptr:
		enc.AddUintptr(d.Key, val)
	case time.Time:
		enc.AddTime(d.Key, val)
	case xtime.Time:
		enc.AddTime(d.Key, val.Time())
	case time.Duration:
		enc.AddDuration(d.Key, val)
	case xtime.Duration:
		enc.AddDuration(d.Key, time.Duration(val))
	case error:
		enc.AddString(d.Key, val.Error())
	case fmt.Stringer:
		enc.AddString(d.Key, val.String())
	default:
		err = enc.AddReflected(d.Key, val)
	}

	if err != nil {
		enc.AddString(fmt.Sprintf("%sError", d.Key), err.Error())
	}
}

// KV return a log kv for logging field.
func KV(key string, value interface{}) D {
	return D{
		Key:   key,
		Value: value,
	}
}

type logFilter []string

func (f *logFilter) String() string {
	return fmt.Sprint(*f)
}

// Set sets the value of the named command-line flag.
// format: -log.filter key1,key2
func (f *logFilter) Set(value string) error {
	for _, i := range strings.Split(value, ",") {
		*f = append(*f, strings.TrimSpace(i))
	}
	return nil
}

// Info logs a message at the info log level.
func Info(format string, args ...interface{}) {
	h.Log(context.Background(), _infoLevel, KV(_log, fmt.Sprintf(format, args...)))
}

// Warn logs a message at the warning log level.
func Warn(format string, args ...interface{}) {
	h.Log(context.Background(), _warnLevel, KV(_log, fmt.Sprintf(format, args...)))
}

// Error logs a message at the error log level.
func Error(format string, args ...interface{}) {
	h.Log(context.Background(), _errorLevel, KV(_log, fmt.Sprintf(format, args...)))
}

// Infoc logs a message at the info log level.
func Infoc(ctx context.Context, format string, args ...interface{}) {
	h.Log(ctx, _infoLevel, KV(_log, fmt.Sprintf(format, args...)))
}

// Errorc logs a message at the error log level.
func Errorc(ctx context.Context, format string, args ...interface{}) {
	h.Log(ctx, _errorLevel, KV(_log, fmt.Sprintf(format, args...)))
}

// Warnc logs a message at the warning log level.
func Warnc(ctx context.Context, format string, args ...interface{}) {
	h.Log(ctx, _warnLevel, KV(_log, fmt.Sprintf(format, args...)))
}

// Infov logs a message at the info log level.
func Infov(ctx context.Context, args ...D) {
	h.Log(ctx, _infoLevel, args...)
}

// Warnv logs a message at the warning log level.
func Warnv(ctx context.Context, args ...D) {
	h.Log(ctx, _warnLevel, args...)
}

// Errorv logs a message at the error log level.
func Errorv(ctx context.Context, args ...D) {
	h.Log(ctx, _errorLevel, args...)
}

// SetFormat only effective on stdout and file handler
// %T time format at "15:04:05.999" on stdout handler, "15:04:05 MST" on file handler
// %t time format at "15:04:05" on stdout handler, "15:04" on file on file handler
// %D data format at "2006/01/02"
// %d data format at "01/02"
// %L log level e.g. INFO WARN ERROR
// %M log message and additional fields: key=value this is log message
// NOTE below pattern not support on file handler
// %f function name and line number e.g. model.Get:121
// %i instance id
// %e deploy env e.g. dev uat fat prod
// %z zone
// %S full file name and line number: /a/b/c/d.go:23
// %s final file name element and line number: d.go:23
func SetFormat(format string) {
	h.SetFormat(format)
}

// Infow logs a message with some additional context. The variadic key-value pairs are treated as they are in With.
func Infow(ctx context.Context, args ...interface{}) {
	h.Log(ctx, _infoLevel, logw(args)...)
}

// Warnw logs a message with some additional context. The variadic key-value pairs are treated as they are in With.
func Warnw(ctx context.Context, args ...interface{}) {
	h.Log(ctx, _warnLevel, logw(args)...)
}

// Errorw logs a message with some additional context. The variadic key-value pairs are treated as they are in With.
func Errorw(ctx context.Context, args ...interface{}) {
	h.Log(ctx, _errorLevel, logw(args)...)
}

func logw(args []interface{}) []D {
	if len(args)%2 != 0 {
		Warn("log: the variadic must be plural, the last one will ignored")
	}
	ds := make([]D, 0, len(args)/2)
	for i := 0; i < len(args)-1; i = i + 2 {
		if key, ok := args[i].(string); ok {
			ds = append(ds, KV(key, args[i+1]))
		} else {
			Warn("log: key must be string, get %T, ignored", args[i])
		}
	}
	return ds
}

// Close close resource.
func Close() (err error) {
	err = h.Close()
	h = _defaultStdout
	return
}
