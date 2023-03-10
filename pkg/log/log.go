package log

import "time"

type Level string

const (
	LevelInfo  Level = "info"
	LevelError Level = "error"
)

type Message struct {
	Time    *time.Time
	Level   Level
	Message string
	Data    Data

	domain string
}

type Datum interface{}

type Data map[string]Datum

func MergeData(dataList ...Data) Data {
	data := Data{}

	for _, d := range dataList {
		for k, v := range d {
			data[k] = v
		}
	}

	return data
}
