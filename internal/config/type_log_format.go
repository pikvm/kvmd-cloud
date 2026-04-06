package config

import (
	"fmt"
	"strings"
)

type LogFormat int

const (
	LogFormatJSON LogFormat = iota
	LogFormatText
)

var (
	logFormatEnumToString = map[LogFormat]string{
		LogFormatJSON: "json",
		LogFormatText: "text",
	}

	logFormatStringToEnum = map[string]LogFormat{
		"json": LogFormatJSON,
		"text": LogFormatText,
	}
)

func (e *LogFormat) String() string {
	return logFormatEnumToString[*e]
}

// func (e *LogFormat) parse(s string) (LogFormat, error) {
// 	if val, ok := logFormatStringToEnum[s]; ok {
// 		return val, nil
// 	}
// 	return 0, fmt.Errorf("invalid log format value: %s", s)
// }

func (e *LogFormat) MarshalText() ([]byte, error) {
	str, ok := logFormatEnumToString[*e]
	if !ok {
		return nil, fmt.Errorf("invalid LogFormat value: %d", *e)
	}
	return []byte(str), nil
}

func (e *LogFormat) UnmarshalText(text []byte) error {
	s := strings.ToLower(string(text))
	val, ok := logFormatStringToEnum[s]
	if !ok {
		return fmt.Errorf("invalid log format value: %s", s)
	}
	*e = val
	return nil
}
