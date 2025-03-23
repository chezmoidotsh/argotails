package zapcoreutils

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/alecthomas/kong"
	"go.uber.org/zap/zapcore"
)

var (
	// LevelEnablerMapper is a type that controls log level enabled for a logger.
	LevelEnablerMapper = kong.TypeMapper(reflect.TypeFor[zapcore.LevelEnabler](), kong.MapperFunc(func(ctx *kong.DecodeContext, target reflect.Value) error {
		t, err := ctx.Scan.PopValue("int")
		if err != nil {
			return err
		}

		var verbosity int
		switch v := t.Value.(type) {
		case string:
			verbosity, err = strconv.Atoi(v)
			if err != nil {
				return err
			}
		case int:
			verbosity = v
		case int8:
			verbosity = int(v)
		case int16:
			verbosity = int(v)
		case int32:
			verbosity = int(v)
		case int64:
			verbosity = int(v)
		case uint:
			verbosity = int(v)
		case uint8:
			verbosity = int(v)
		case uint16:
			verbosity = int(v)
		case uint32:
			verbosity = int(v)
		case uint64:
			verbosity = int(v)
		default:
			return fmt.Errorf("unsupported type %T", t.Value)
		}

		if verbosity < 1 || verbosity > 128 {
			return fmt.Errorf("must be between 1 and 127")
		}

		target.Set(reflect.ValueOf(zapcore.Level(verbosity * -1)))
		return nil
	}))

	// EncoderMapper is a type that controls log encoding format for a logger.
	EncoderMapper = kong.TypeMapper(reflect.TypeFor[zapcore.Encoder](), kong.MapperFunc(func(ctx *kong.DecodeContext, target reflect.Value) error {
		var encoding string
		if err := ctx.Scan.PopValueInto("string", &encoding); err != nil {
			return err
		}

		var encoder zapcore.Encoder
		switch encoding {
		case "json":
			encoder = zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
					encoder.AppendInt(int(level) * -1)
				},
				EncodeTime: zapcore.ISO8601TimeEncoder,
				LevelKey:   "v",
				MessageKey: "msg",
				NameKey:    "component",
				TimeKey:    "time",
			})
		case "console":
			encoder = zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
				EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
					encoder.AppendString(fmt.Sprintf("V(%d)", int(level)*-1))
				},
				EncodeTime: zapcore.ISO8601TimeEncoder,
				LevelKey:   "v",
				MessageKey: "msg",
				TimeKey:    "time",
			})
		default:
			return fmt.Errorf("must be one of 'json' or 'console' but got '%s'", encoding)
		}

		target.Set(reflect.ValueOf(encoder))
		return nil
	}))
)
