package collector_test

import (
	"net/http"
	"log"

	"bitbucket.org/mjl/sherpa"
	"github.com/irias/sherpa-prometheus-collector"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// You'll need to import "github.com/irias/sherpa-prometheus-collector" and "github.com/prometheus/client_golang/prometheus/promhttp".
func Example_main() {
	funcs := map[string]interface{}{
		"sum": func(a, b int) int {
			return a + b
		},
	}
	collector, err := collector.NewCollector("test", nil)
	if err != nil {
		log.Fatalln("making prometheus sherpa stats collector:", err)
	}
	handler, err := sherpa.NewHandler("/test/", "Test API", "0.0.1", funcs, collector)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/test/", handler)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
