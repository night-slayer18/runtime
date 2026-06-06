package plugin

import (
	"errors"
	"testing"
)

// fakePlugin is a configurable test double implementing Plugin.
type fakePlugin struct {
	name     string
	version  string
	initFn   func(ctx PluginContext) error
	disposed bool
}

func (f *fakePlugin) Name() string    { return f.name }
func (f *fakePlugin) Version() string { return f.version }
func (f *fakePlugin) Init(ctx PluginContext) error {
	if f.initFn != nil {
		return f.initFn(ctx)
	}
	return nil
}
func (f *fakePlugin) Dispose() { f.disposed = true }

func TestRegisterNilPlugin(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); !errors.Is(err, ErrNilPlugin) {
		t.Errorf("Register(nil) = %v, want ErrNilPlugin", err)
	}
}

func TestRegisterEmptyName(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&fakePlugin{name: ""}); !errors.Is(err, ErrEmptyName) {
		t.Errorf("Register empty name = %v, want ErrEmptyName", err)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&fakePlugin{name: "dup", version: "1.0.0"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := r.Register(&fakePlugin{name: "dup", version: "2.0.0"})
	if !errors.Is(err, ErrAlreadyRegistered) {
		t.Errorf("duplicate Register = %v, want ErrAlreadyRegistered", err)
	}
}

func TestRegisterRunsInitAndRegistersExtensionPoints(t *testing.T) {
	r := NewRegistry()
	p := &fakePlugin{
		name:    "demo",
		version: "0.1.0",
		initFn: func(ctx PluginContext) error {
			ctx.RegisterCommand("hello", func(args []string) error { return nil })
			ctx.RegisterDataSource("mem", func() (any, error) { return "ds", nil })
			ctx.RegisterView("panel", func() (any, error) { return "view", nil })
			return nil
		},
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, ok := r.Command("hello"); !ok {
		t.Error("command 'hello' not registered")
	}
	if _, ok := r.DataSource("mem"); !ok {
		t.Error("data source 'mem' not registered")
	}
	if _, ok := r.View("panel"); !ok {
		t.Error("view 'panel' not registered")
	}

	metas := r.Plugins()
	if len(metas) != 1 || metas[0].Name != "demo" || metas[0].Version != "0.1.0" {
		t.Errorf("Plugins() = %+v, want one demo/0.1.0 entry", metas)
	}
}

func TestRegisterInitFailureRollsBack(t *testing.T) {
	r := NewRegistry()
	wantErr := errors.New("boom")
	p := &fakePlugin{
		name:    "bad",
		version: "0.0.1",
		initFn: func(ctx PluginContext) error {
			ctx.RegisterCommand("ghost", func(args []string) error { return nil })
			return wantErr
		},
	}

	err := r.Register(p)
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("Register err = %v, want wrapping %v", err, wantErr)
	}
	// The plugin and its registrations must be rolled back.
	if _, ok := r.Command("ghost"); ok {
		t.Error("command from failed Init should have been rolled back")
	}
	if len(r.Plugins()) != 0 {
		t.Errorf("Plugins() = %+v, want empty after failed Init", r.Plugins())
	}
}

func TestCommandHandlerInvocation(t *testing.T) {
	r := NewRegistry()
	var gotArgs []string
	p := &fakePlugin{
		name:    "cmds",
		version: "1.0.0",
		initFn: func(ctx PluginContext) error {
			ctx.RegisterCommand("echo", func(args []string) error {
				gotArgs = args
				return nil
			})
			return nil
		},
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h, ok := r.Command("echo")
	if !ok {
		t.Fatal("command 'echo' not found")
	}
	if err := h([]string{"a", "b"}); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "a" || gotArgs[1] != "b" {
		t.Errorf("handler args = %v, want [a b]", gotArgs)
	}
}

func TestDataSourceAndViewFactories(t *testing.T) {
	r := NewRegistry()
	p := &fakePlugin{
		name:    "factories",
		version: "1.0.0",
		initFn: func(ctx PluginContext) error {
			ctx.RegisterDataSource("ds", func() (any, error) { return 42, nil })
			ctx.RegisterView("v", func() (any, error) { return "ui", nil })
			return nil
		},
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	dsf, ok := r.DataSource("ds")
	if !ok {
		t.Fatal("data source not found")
	}
	got, err := dsf()
	if err != nil || got != 42 {
		t.Errorf("ds factory = (%v, %v), want (42, nil)", got, err)
	}

	vf, ok := r.View("v")
	if !ok {
		t.Fatal("view not found")
	}
	gotV, err := vf()
	if err != nil || gotV != "ui" {
		t.Errorf("view factory = (%v, %v), want (ui, nil)", gotV, err)
	}
}

func TestPublishEventDeliversToSubscribers(t *testing.T) {
	r := NewRegistry()
	var received []any
	unsub := r.Subscribe("topic", func(data any) {
		received = append(received, data)
	})
	defer unsub()

	r.PublishEvent("topic", "first")
	r.PublishEvent("topic", 2)
	r.PublishEvent("other", "ignored")

	if len(received) != 2 || received[0] != "first" || received[1] != 2 {
		t.Errorf("received = %v, want [first 2]", received)
	}
}

func TestPluginPublishedEventReachesHostSubscriber(t *testing.T) {
	r := NewRegistry()
	var got any
	unsub := r.Subscribe("ping", func(data any) { got = data })
	defer unsub()

	p := &fakePlugin{
		name:    "publisher",
		version: "1.0.0",
		initFn: func(ctx PluginContext) error {
			ctx.PublishEvent("ping", "pong")
			return nil
		},
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got != "pong" {
		t.Errorf("subscriber got %v, want pong", got)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	r := NewRegistry()
	count := 0
	unsub := r.Subscribe("topic", func(any) { count++ })

	r.PublishEvent("topic", nil)
	unsub()
	r.PublishEvent("topic", nil)
	unsub() // idempotent

	if count != 1 {
		t.Errorf("handler called %d times, want 1", count)
	}
}

func TestUnregisterDisposesAndRemovesRegistrations(t *testing.T) {
	r := NewRegistry()
	p := &fakePlugin{
		name:    "teardown",
		version: "1.0.0",
		initFn: func(ctx PluginContext) error {
			ctx.RegisterCommand("c", func([]string) error { return nil })
			ctx.RegisterDataSource("d", func() (any, error) { return nil, nil })
			ctx.RegisterView("v", func() (any, error) { return nil, nil })
			return nil
		},
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := r.Unregister("teardown"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if !p.disposed {
		t.Error("Dispose was not called")
	}
	if _, ok := r.Command("c"); ok {
		t.Error("command not removed after Unregister")
	}
	if _, ok := r.DataSource("d"); ok {
		t.Error("data source not removed after Unregister")
	}
	if _, ok := r.View("v"); ok {
		t.Error("view not removed after Unregister")
	}
	if len(r.Plugins()) != 0 {
		t.Errorf("Plugins() = %+v, want empty", r.Plugins())
	}
}

func TestUnregisterUnknown(t *testing.T) {
	r := NewRegistry()
	if err := r.Unregister("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Unregister unknown = %v, want ErrNotFound", err)
	}
}

func TestPluginsSortedByName(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"charlie", "alpha", "bravo"} {
		if err := r.Register(&fakePlugin{name: n, version: "1.0.0"}); err != nil {
			t.Fatalf("Register %q: %v", n, err)
		}
	}
	metas := r.Plugins()
	want := []string{"alpha", "bravo", "charlie"}
	if len(metas) != len(want) {
		t.Fatalf("got %d plugins, want %d", len(metas), len(want))
	}
	for i, m := range metas {
		if m.Name != want[i] {
			t.Errorf("Plugins()[%d] = %q, want %q", i, m.Name, want[i])
		}
	}
}

// Compile-time assertions that the concrete context types satisfy PluginContext.
var _ PluginContext = (*scopedContext)(nil)
var _ PluginContext = (*Registry)(nil)
