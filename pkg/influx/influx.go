package influx

import "time"

type Point struct {
	Measurement string
	Tags        Tags
	Fields      Fields
	Timestamp   *time.Time
}

type Points []*Point

type Tags map[string]string

type Fields map[string]interface{}

func NewPoint(measurement string, tags Tags, fields Fields) *Point {
	return &Point{
		Measurement: measurement,
		Tags:        tags,
		Fields:      fields,
	}
}

func NewPointWithTimestamp(measurement string, tags Tags, fields Fields, t time.Time) *Point {
	return &Point{
		Measurement: measurement,
		Tags:        tags,
		Fields:      fields,
		Timestamp:   &t,
	}
}
