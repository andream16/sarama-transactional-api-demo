package metric

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metricsHandler struct{}

const (
	labelAppName   = "app_name"
	labelSuccess   = "success"
	labelAborted   = "aborted"
	labelCommitted = "committed"
	labelSender    = "sender"
	labelOffset    = "offset"
	labelPartition = "partition"
)

var (
	messagesSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_sent",
			Help: "Metrics about messages being sent",
		},
		[]string{labelAppName, labelSuccess, labelCommitted, labelAborted},
	)

	messagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_consumed",
			Help: "Metrics about messages being consumed",
		},
		[]string{labelSender, labelOffset, labelPartition},
	)
)

func NewMetricsHandler() (*metricsHandler, error) {
	if err := register(); err != nil {
		return nil, err
	}

	return &metricsHandler{}, nil
}

func IncMessagesSent(appName string, numSent int, success, committed, aborted bool) {
	messagesSent.With(prometheus.Labels{
		labelAppName:   appName,
		labelSuccess:   fmt.Sprintf("%t", success),
		labelCommitted: fmt.Sprintf("%t", committed),
		labelAborted:   fmt.Sprintf("%t", aborted),
	}).Add(float64(numSent))
}

func IncMessagesConsumed(sender string, offset int64, partition int32) {
	messagesConsumed.With(prometheus.Labels{
		labelSender:    sender,
		labelOffset:    fmt.Sprintf("%d", offset),
		labelPartition: fmt.Sprintf("%d", partition),
	}).Inc()
}

func (h *metricsHandler) Path() string {
	return "/_metrics"
}

func (h *metricsHandler) Register(mux *http.ServeMux) {
	mux.Handle(h.Path(), promhttp.Handler())
}

func register() (err error) {
	for _, c := range []prometheus.Collector{messagesSent, messagesConsumed} {
		prometheus.MustRegister(c)
	}

	return nil
}
