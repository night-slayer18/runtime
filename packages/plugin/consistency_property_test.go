package plugin

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"testing/quick"
)

// Feature: runtime-ecosystem, Property 7: Plugin API consistency
//
// For any application host, a loaded plugin receives the same set of hooks
// (UI extensions, data sources, workflows) regardless of which host loaded it.
//
// Validates: Requirements 8.2

// pluginSpec is a randomly generated description of the extension points a
// plugin will register during Init. It is the input space for the property:
// any combination of commands, data sources, views, and published events.
type pluginSpec struct {
	commands    []string
	dataSources []string
	views       []string
	events      []string
}

// Generate implements quick.Generator so testing/quick can produce random
// plugin specs. It constrains the input space to valid (non-empty) hook names
// of bounded count, which is exactly the space of plugins a host can load.
func (pluginSpec) Generate(r *rand.Rand, size int) reflect.Value {
	// Keep counts bounded but allow zero so empty plugins are also exercised.
	names := func(prefix string) []string {
		n := r.Intn(5) // 0..4 hooks of this kind
		out := make([]string, n)
		for i := range out {
			// Non-empty, occasionally colliding names to stress the registry.
			out[i] = fmt.Sprintf("%s-%d", prefix, r.Intn(6))
		}
		return out
	}
	spec := pluginSpec{
		commands:    names("cmd"),
		dataSources: names("ds"),
		views:       names("view"),
		events:      names("evt"),
	}
	return reflect.ValueOf(spec)
}

// specPlugin is a Plugin built from a pluginSpec. The same specPlugin value is
// loaded into multiple independent registries (hosts) so we can assert the
// hooks it receives and registers are identical across hosts.
type specPlugin struct {
	name string
	spec pluginSpec

	// contextHooks records the set of hook methods exposed by the
	// PluginContext that this plugin was handed during Init, captured per host.
	contextHooks []string
}

func (p *specPlugin) Name() string    { return p.name }
func (p *specPlugin) Version() string { return "1.0.0" }
func (p *specPlugin) Dispose()        {}

func (p *specPlugin) Init(ctx PluginContext) error {
	// Capture the hook set the host exposes to the plugin. Property 7 requires
	// this to be identical no matter which host performed the load.
	p.contextHooks = contextHookSet(ctx)

	for _, c := range p.spec.commands {
		ctx.RegisterCommand(c, func(args []string) error { return nil })
	}
	for _, d := range p.spec.dataSources {
		ctx.RegisterDataSource(d, func() (any, error) { return nil, nil })
	}
	for _, v := range p.spec.views {
		ctx.RegisterView(v, func() (any, error) { return nil, nil })
	}
	for _, e := range p.spec.events {
		ctx.PublishEvent(e, "payload")
	}
	return nil
}

// contextHookSet returns the sorted set of exported method names available on
// the PluginContext implementation, i.e. the hooks a plugin can use.
func contextHookSet(ctx PluginContext) []string {
	t := reflect.TypeOf(ctx)
	out := make([]string, 0, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		out = append(out, t.Method(i).Name)
	}
	sort.Strings(out)
	return out
}

// loadedHooks captures the observable hooks a host ended up with after loading
// a plugin: the sorted names of every command, data source, and view the
// plugin registered, plus the events that were delivered to host subscribers.
type loadedHooks struct {
	commands    []string
	dataSources []string
	views       []string
	events      []string
	contextSet  []string
}

func (h loadedHooks) equal(o loadedHooks) bool {
	return reflect.DeepEqual(h.commands, o.commands) &&
		reflect.DeepEqual(h.dataSources, o.dataSources) &&
		reflect.DeepEqual(h.views, o.views) &&
		reflect.DeepEqual(h.events, o.events) &&
		reflect.DeepEqual(h.contextSet, o.contextSet)
}

// loadInHost models an independent application host: a fresh Registry that
// subscribes to the plugin's events, loads the plugin, and reports the hook set
// that resulted.
func loadInHost(spec pluginSpec) (loadedHooks, error) {
	r := NewRegistry()

	// The host subscribes to every event the plugin may publish so we can
	// observe which events were delivered during load.
	var delivered []string
	for _, e := range uniqueSorted(spec.events) {
		ev := e
		r.Subscribe(ev, func(any) { delivered = append(delivered, ev) })
	}

	p := &specPlugin{name: "subject", spec: spec}
	if err := r.Register(p); err != nil {
		return loadedHooks{}, err
	}

	// Collect the hooks the plugin actually registered on this host.
	hooks := loadedHooks{
		commands:    presentNames(r.Command, spec.commands),
		dataSources: presentDataSources(r, spec.dataSources),
		views:       presentViews(r, spec.views),
		contextSet:  p.contextHooks,
	}
	sort.Strings(delivered)
	hooks.events = delivered
	return hooks, nil
}

func presentNames(lookup func(string) (CommandHandler, bool), names []string) []string {
	var out []string
	for _, n := range uniqueSorted(names) {
		if _, ok := lookup(n); ok {
			out = append(out, n)
		}
	}
	return out
}

func presentDataSources(r *Registry, names []string) []string {
	var out []string
	for _, n := range uniqueSorted(names) {
		if _, ok := r.DataSource(n); ok {
			out = append(out, n)
		}
	}
	return out
}

func presentViews(r *Registry, names []string) []string {
	var out []string
	for _, n := range uniqueSorted(names) {
		if _, ok := r.View(n); ok {
			out = append(out, n)
		}
	}
	return out
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestProperty7_PluginAPIConsistency loads the same generated plugin into
// several independent hosts (Registry instances representing different
// applications) and asserts every host exposes the same hook set to the plugin
// and ends up with identical registered hooks.
func TestProperty7_PluginAPIConsistency(t *testing.T) {
	const hostCount = 4

	prop := func(spec pluginSpec) bool {
		var baseline loadedHooks
		for i := 0; i < hostCount; i++ {
			got, err := loadInHost(spec)
			if err != nil {
				t.Errorf("host %d failed to load plugin: %v", i, err)
				return false
			}
			if i == 0 {
				baseline = got
				continue
			}
			if !got.equal(baseline) {
				t.Errorf("host %d hook set differs from host 0:\n got: %+v\nwant: %+v",
					i, got, baseline)
				return false
			}
		}
		return true
	}

	// MaxCount exceeds the design's 100-iteration minimum for property tests.
	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}
