// Sherpa-prometheus-collector provides a collector of statistics for incoming Sherpa requests that are exported over to Prometheus.
//
// Dependencies are not included, you'll want to import "github.com/prometheus/client_golang/prometheus".
//
// Use with the Sherpa library at https://bitbucket.org/mjl/sherpa and the Prometheus library at https://github.com/prometheus/client_golang.
package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	requests, errors, serverErrors *prometheus.CounterVec
	protocolErrors, badFunction, javascript, json	prometheus.Counter
	requestDuration *prometheus.HistogramVec
}

/*
Create new collector for the named api. Metrics will be labeled with "api". The following prometheus metrics are automatically registered on reg, or the default prometheus registerer if reg is nil:

	sherpa_requests_total
		calls, per function
	sherpa_errors_total
		error responses (including server errors), per function
	sherpa_servererrors_total
		server error responses, per function
	sherpa_protocol_errors_total
		incorrect requests
	sherpa_bad_function_total
		unknown functions called
	sherpa_javascript_request_total
		requests to sherpa.js
	sherpa_json_request_total
		requests to sherpa.json
	sherpa_requests_duration_seconds
		histogram for .01, .05, .1, .2, .5, 1, 2, 4, 8, 16, per function
*/
func NewCollector(api string, reg prometheus.Registerer) (*Collector, error) {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	apiLabel := prometheus.Labels{"api": api}
	c := &Collector{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sherpa_requests_total",
			Help: "Total sherpa requests.",
			ConstLabels: apiLabel,
		}, []string{"function"}),
		errors:  prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sherpa_errors_total",
			Help: "Total sherpa error responses.",
			ConstLabels: apiLabel,
		}, []string{"function"}),
		serverErrors:  prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sherpa_servererrors_total",
			Help: "Total sherpa server error responses.",
			ConstLabels: apiLabel,
		}, []string{"function"}),
		protocolErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sherpa_protocol_errors_total",
			Help: "Total sherpa protocol errors.",
			ConstLabels: apiLabel,
		}),
		badFunction: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sherpa_bad_function_total",
			Help: "Total sherpa bad function calls.",
			ConstLabels: apiLabel,
		}),
		javascript: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sherpa_javascript_request_total",
			Help: "Total sherpa.js requests.",
			ConstLabels: apiLabel,
		}),
		json: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sherpa_json_requests_total",
			Help: "Total sherpa.json requests.",
			ConstLabels: apiLabel,
		}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "sherpa_requests_duration_seconds",
			Help: "Sherpa request duration in seconds.",
			ConstLabels: apiLabel,
			Buckets: []float64{.01, .05, .1, .2, .5, 1, 2, 4, 8, 16},
		}, []string{"function"}),
	}
	first := func(errors... error) error {
		for _, err := range errors {
			if err != nil {
				return err
			}
		}
		return nil
	}
	err := first(
		reg.Register(c.requests),
		reg.Register(c.errors),
		reg.Register(c.serverErrors),
		reg.Register(c.protocolErrors),
		reg.Register(c.badFunction),
		reg.Register(c.javascript),
		reg.Register(c.json),
		reg.Register(c.requestDuration),
	)
	return c, err
}

// Increase "sherpa_bad_function_total" count by one.
func (c *Collector) BadFunction() {
	c.badFunction.Inc()
}

// Increase "sherpa_protocol_errors_total" by one.
func (c *Collector) ProtocolError() {
	c.protocolErrors.Inc()
}

// Increase "sherpa_json_requests_total" by one.
func (c *Collector) JSON() {
	c.json.Inc()
}

// Increase "sherpa_javascript_requests_total" sherpa.js by one.
func (c *Collector) JavaScript() {
	c.javascript.Inc()
}

// Increase "sherpa_requests_total" and possibly "sherpa_error_total" and "sherpa_servererror_total" by one, and register the function call duration in "sherpa_requests_duration_seconds".
func (c *Collector) FunctionCall(name string, error bool, serverError bool, duration float64) {
	c.requests.WithLabelValues(name).Inc()
	if error {
		c.errors.WithLabelValues(name).Inc()
		if serverError {
			c.serverErrors.WithLabelValues(name).Inc()
		}
	}
	c.requestDuration.WithLabelValues(name).Observe(duration)
}
