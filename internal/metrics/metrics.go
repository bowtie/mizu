// Package metrics implements a minimal Prometheus-text-format metrics
// registry. It deliberately avoids the prometheus/client_golang
// dependency: Mizu exports a handful of counters and gauges, and the
// text exposition format is trivial to emit.
package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
)

var (
	mu            sync.Mutex
	counters      = map[string]*counter{}
	gauges        = map[string]*gauge{}
	labeledGauges = map[string]*gaugeWithSamples{}
)

type counter struct {
	help   string
	labels []string          // label name set, fixed at registration
	values map[string]uint64 // raw-values join key -> count
}

type gauge struct {
	help  string
	value func() float64 // nil for set-style gauges
	set   float64
}

// gaugeSample is one labeled time series of a static labeled gauge. Each
// sample carries its own value — sharing a single value across label sets
// made the last-registered label set's value render for all of them.
type gaugeSample struct {
	labels []string // ordered values matching the gauge's label names
	value  float64
}

type gaugeWithSamples struct {
	help    string
	labels  []string                // fixed label names, sorted at registration
	samples map[string]*gaugeSample // raw-values join key -> sample
}

// RegisterCounter creates a monotonically increasing counter. labels is
// the ordered list of label names; every Add must pass the same number
// of values. Re-registering an existing name is a no-op (idempotent so
// package init order doesn't matter).
func RegisterCounter(name, help string, labels ...string) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := counters[name]; !ok {
		counters[name] = &counter{
			help:   help,
			labels: labels,
			values: map[string]uint64{},
		}
	}
}

// Add increments a counter by one for the given label values.
func Add(name string, labelValues ...string) {
	AddN(name, 1, labelValues...)
}

// AddN increments a counter by n for the given label values.
func AddN(name string, n uint64, labelValues ...string) {
	mu.Lock()
	defer mu.Unlock()
	c, ok := counters[name]
	if !ok {
		return
	}
	c.values[rawLabelKey(labelValues)] += n
}

// rawLabelKey joins raw label values into a map key. Escaping happens at
// render time only — escaping in the key would double-apply %q quoting.
func rawLabelKey(values []string) string {
	return strings.Join(values, "\x00")
}

// SetGauge stores a static gauge value.
func SetGauge(name, help string, v float64) {
	mu.Lock()
	defer mu.Unlock()
	g, ok := gauges[name]
	if !ok {
		g = &gauge{help: help}
		gauges[name] = g
	}
	if g.value == nil {
		g.set = v
	}
}

// SetLabeledGauge exposes a static gauge with a fixed set of labels,
// e.g. build info: SetLabeledGauge("mizu_build_info", "...", 1, map[string]string{"version": "1.2.3"}).
// Each label set renders its own value; re-registering the same label
// set overwrites that sample.
func SetLabeledGauge(name, help string, v float64, labels map[string]string) {
	mu.Lock()
	defer mu.Unlock()
	g, ok := labeledGauges[name]
	if !ok {
		g = &gaugeWithSamples{help: help, samples: map[string]*gaugeSample{}}
		names := make([]string, 0, len(labels))
		for n := range labels {
			names = append(names, n)
		}
		sort.Strings(names)
		g.labels = names
		labeledGauges[name] = g
	}
	values := make([]string, len(g.labels))
	for i, n := range g.labels {
		values[i] = labels[n]
	}
	g.samples[rawLabelKey(values)] = &gaugeSample{labels: values, value: v}
}

// renderLabels formats ordered name/value pairs for exposition output.
func renderLabels(names, values []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts[i] = fmt.Sprintf(`%s="%s"`, n, escape(v))
	}
	return strings.Join(parts, ",")
}

func escape(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}

// Render produces the Prometheus text exposition format.
func Render() string {
	mu.Lock()

	var b strings.Builder

	names := make([]string, 0, len(counters))
	for n := range counters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := counters[n]
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n", n, c.help, n)
		keys := make([]string, 0, len(c.values))
		for k := range c.values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var vals []string
			if k != "" {
				vals = strings.Split(k, "\x00")
			}
			labelStr := ""
			if len(c.labels) > 0 {
				labelStr = "{" + renderLabels(c.labels, vals) + "}"
			}
			fmt.Fprintf(&b, "%s%s %d\n", n, labelStr, c.values[k])
		}
	}

	gnames := make([]string, 0, len(gauges))
	for n := range gauges {
		gnames = append(gnames, n)
	}
	sort.Strings(gnames)
	for _, n := range gnames {
		g := gauges[n]
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", n, g.help, n)
		if g.value == nil {
			fmt.Fprintf(&b, "%s %v\n", n, g.set)
		}
	}

	lnames := make([]string, 0, len(labeledGauges))
	for n := range labeledGauges {
		lnames = append(lnames, n)
	}
	sort.Strings(lnames)
	for _, n := range lnames {
		g := labeledGauges[n]
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", n, g.help, n)
		sampleKeys := make([]string, 0, len(g.samples))
		for k := range g.samples {
			sampleKeys = append(sampleKeys, k)
		}
		sort.Strings(sampleKeys)
		for _, k := range sampleKeys {
			s := g.samples[k]
			fmt.Fprintf(&b, "%s{%s} %v\n", n, renderLabels(g.labels, s.labels), s.value)
		}
	}

	// Snapshot the gauge funcs while locked; evaluate them after unlock —
	// a func that itself calls metrics.Add (or any registry mutation)
	// would deadlock if invoked under mu.
	type gaugeFn struct {
		name, help string
		fn         func() float64
	}
	var fns []gaugeFn
	for n, g := range gauges {
		if g.value != nil {
			fns = append(fns, gaugeFn{name: n, help: g.help, fn: g.value})
		}
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].name < fns[j].name })

	mu.Unlock()

	for _, f := range fns {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", f.name, f.help, f.name)
		fmt.Fprintf(&b, "%s %v\n", f.name, f.fn())
	}

	// Runtime gauges — always present so scrapes are never empty.
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(&b, "# HELP go_goroutines Number of goroutines.\n# TYPE go_goroutines gauge\ngo_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "# HELP go_mem_alloc_bytes Bytes of allocated heap objects.\n# TYPE go_mem_alloc_bytes gauge\ngo_mem_alloc_bytes %d\n", ms.Alloc)
	fmt.Fprintf(&b, "# HELP go_sys_bytes Bytes of system memory obtained from the OS.\n# TYPE go_sys_bytes gauge\ngo_sys_bytes %d\n", ms.Sys)

	return b.String()
}

// Handler serves the metrics endpoint.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(Render()))
	})
}
