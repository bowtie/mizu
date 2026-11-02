package metrics

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounterWithLabelsRenders(t *testing.T) {
	RegisterCounter("test_requests_total", "Test counter.", "method", "status")
	Add("test_requests_total", "GET", "200")
	Add("test_requests_total", "GET", "200")
	Add("test_requests_total", "POST", "500")

	out := Render()
	assert.Contains(t, out, "# TYPE test_requests_total counter")
	assert.Contains(t, out, `test_requests_total{method="GET",status="200"} 2`)
	assert.Contains(t, out, `test_requests_total{method="POST",status="500"} 1`)
}

func TestLabeledGaugeRendersSortedLabels(t *testing.T) {
	SetLabeledGauge("test_build_info", "Build info.", 1, map[string]string{
		"version": `1.0 "beta"`,
		"commit":  "abc",
	})
	out := Render()
	assert.Contains(t, out, `test_build_info{commit="abc",version="1.0 \"beta\""} 1`)
}

func TestLabelValuesAreEscaped(t *testing.T) {
	RegisterCounter("test_escape_total", "Escape.", "route")
	Add("test_escape_total", `/shows/{id}"evil`)
	out := Render()
	assert.Contains(t, out, `route="/shows/{id}\"evil"`)
}

func TestRuntimeGaugesAlwaysPresent(t *testing.T) {
	out := Render()
	for _, want := range []string{"go_goroutines ", "go_mem_alloc_bytes ", "# TYPE go_goroutines gauge"} {
		assert.True(t, strings.Contains(out, want), "missing %q in output", want)
	}
}
