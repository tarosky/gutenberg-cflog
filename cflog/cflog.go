package cflog

import (
	"bufio"
	"compress/gzip"
	"io"
	"net/url"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Provided by govvv at compile time
var Version string

const headerKeyLen = len("#Fields: ")

var voidv = struct{}{}

var intFields = map[string]struct{}{
	"sc-status":      voidv,
	"sc-bytes":       voidv,
	"cs-bytes":       voidv,
	"c-port":         voidv,
	"sc-content-len": voidv,
	"sc-range-start": voidv,
	"sc-range-end":   voidv,
}

var floatFields = map[string]struct{}{
	"time-taken":         voidv,
	"time-to-first-byte": voidv,
}

var ZapErrorLevel = zap.String("level", "ERROR")

// CreateLogger creates and returns a new logger.
func CreateLogger(logPaths []string) *zap.Logger {
	config := &zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "json",
		OutputPaths:      logPaths,
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        zapcore.OmitKey,
			LevelKey:       zapcore.OmitKey,
			NameKey:        zapcore.OmitKey,
			CallerKey:      zapcore.OmitKey,
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "m",
			StacktraceKey:  zapcore.OmitKey,
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
		},
	}
	log, err := config.Build(zap.WithCaller(false))
	if err != nil {
		panic("failed to initialize logger")
	}

	return log.With(zap.String("v", Version))
}

type Config struct {
	Log          *zap.Logger
	OutputFields map[string]string
	CommonPrefix string // CommonPrefix must end with "/"
}

func Scan(gzreader io.Reader, c *Config) error {
	r, err := gzip.NewReader(gzreader)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(r)
	sc.Scan() // #Version:
	sc.Scan() // #Fields:
	keys := strings.Split(sc.Text()[headerKeyLen:], " ")
	for sc.Scan() {
		m := make(map[string]interface{}, len(keys))
		var date, time string
		for i, val := range strings.Split(sc.Text(), "\t") {
			if keys[i] == "date" {
				date = val
				continue
			}

			if keys[i] == "time" {
				time = val
				continue
			}

			k, ok := c.OutputFields[keys[i]]
			if !ok {
				continue
			}
			// k := keys[i]

			if val == "-" {
				continue
			}

			if _, ok := intFields[keys[i]]; ok {
				i, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				m[k] = i
				continue
			}

			if _, ok := floatFields[keys[i]]; ok {
				f, err := strconv.ParseFloat(val, 64)
				if err != nil {
					return err
				}
				m[k] = f
				continue
			}

			v, err := url.QueryUnescape(val)
			if err != nil {
				c.Log.Error("url decode failure",
					ZapErrorLevel,
					zap.String("line", sc.Text()))
				continue
			}

			if keys[i] == "cs(Referer)" && strings.HasPrefix(v, c.CommonPrefix) {
				v = v[len(c.CommonPrefix)-1:]
			}

			m[k] = v
		}
		m["_t"] = date + "T" + time + "Z"
		c.Log.Info("l", zap.Any("l", m))
	}
	return nil
}

func ParseOutputFields(val string) map[string]string {
	m := map[string]string{}
	for _, v := range strings.Split(val, ",") {
		ss := strings.SplitN(v, "=", 2)
		m[ss[0]] = ss[1]
	}
	return m
}
