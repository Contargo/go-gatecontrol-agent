package metrics

import (
	"net/http"
	"strings"
)

type InfluxClient interface {
	Write(line string) error
}

type InfluxClientImpl struct {
	influxUrl string
}

func (i *InfluxClientImpl) Write(line string) error {
	_, err := http.Post(i.influxUrl, "application/x-www-form-urlencoded", strings.NewReader(line))
	return err
}

func NewInfluxClient(influxUrl string) InfluxClient {
	return &InfluxClientImpl{influxUrl}
}
