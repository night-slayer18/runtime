// Package plugin defines the Runtime plugin API and an in-process registry
// that hosts plugins and exposes the extension points a plugin may use:
// commands, data sources, views, and an event bus.
//
// This file implements the plugin contract and registry only. The security
// sandbox and signature/integrity verification are layered on separately
// (see the sandbox implementation) so that the API surface here stays focused
// on registration and dispatch.
package plugin

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Registry errors. Callers can match these with errors.Is.
var (
	// ErrNilPlugin is returned when a nil plugin is registered.
	ErrNilPlugin = errors.New("plugin: nil plugin")
	// ErrEmptyName is returned when a plugin (or an extension point) is
	// registered without a name.
	ErrEmptyName = errors.New("plugin: empty name")
	// ErrAlreadyRegistered is returned when a plugin with the same name is
	// already present in the registry.
	ErrAlreadyRegistered = errors.New("plugin: already registered")
	// ErrNotFound is returned when a named plugin is not present.
	ErrNotFound = errors.New("plugin: not found")
)

// CommandHandler runs a plugin command with the supplied arguments.
type CommandHandler func(args []string) error

// DataSourceFactory constructs a data source on demand. The concrete return
// type is intentionally opaque here so the plugin module does not take a hard
// dependency on the datasource package; hosts type-assert the result to the
// data source interface they expect.
type DataSourceFactory func() (any, error)

// ViewFactory constructs a view (UI surface) on demand. As with
// DataSourceFactory the return type is opaque to keep the plugin API free of
// host-specific UI dependencies.
type ViewFactory func() (any, error)

// EventHandler receives data published on a named event.
type EventHandler func(data any)

// PluginMetadata carries descriptive information about a plugin.
type PluginMetadata struct {
	Name        string
	Version     string
	Author      string
	Description string
	URL         string
}

// Plugin is the contract every Runtime plugin implements.
type Plugin interface {
	// Name returns the unique plugin name used as its registry key.
	Name() string
	// Version returns the plugin's semantic version string.
	Version() string
	// Init is called once when the plugin is registered. The plugin uses the
	// supplied context to register its extension points.
	Init(ctx PluginContext) error
	// Dispose is called when the plugin is unregistered so it can release any
	// resources it holds.
	Dispose()
}

// PluginContext is handed to a plugin during Init. It is the only surface a
// plugin uses to extend the host application, which keeps the set of available
// hooks identical regardless of which application loaded the plugin.
type PluginContext interface {
	// RegisterCommand registers a named command handler.
	RegisterCommand(name string, handler CommandHandler)
	// RegisterDataSource registers a named data source factory.
	RegisterDataSource(name string, factory DataSourceFactory)
	// RegisterView registers a named view factory.
	RegisterView(name string, factory ViewFactory)
	// PublishEvent dispatches data to every handler subscribed to name.
	PublishEvent(name string, data any)
}

// boundCommand pairs a handler with the plugin that owns it so the registry can
// clean up on Unregister.
type boundCommand struct {
	owner   string
	handler CommandHandler
}

type boundDataSource struct {
	owner   string
	factory DataSourceFactory
}

type boundView struct {
	owner   string
	factory ViewFactory
}

type boundSubscriber struct {
	owner   string
	handler EventHandler
}

// Registry hosts plugins and the extension points they register. It is safe for
// concurrent use. The registry itself satisfies PluginContext (host-owned
// registrations), and hands each plugin a scoped context during Init so that
// registrations can be attributed and torn down per plugin.
type Registry struct {
	mu          sync.RWMutex
	plugins     map[string]Plugin
	commands    map[string]boundCommand
	dataSources map[string]boundDataSource
	views       map[string]boundView
	subscribers map[string][]boundSubscriber
}

// NewRegistry returns an empty, ready-to-use registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:     make(map[string]Plugin),
		commands:    make(map[string]boundCommand),
		dataSources: make(map[string]boundDataSource),
		views:       make(map[string]boundView),
		subscribers: make(map[string][]boundSubscriber),
	}
}

// Register adds a plugin and runs its Init with a scoped PluginContext. If Init
// fails, every extension point the plugin registered before failing is rolled
// back and the plugin is not retained.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return ErrNilPlugin
	}
	name := p.Name()
	if name == "" {
		return ErrEmptyName
	}

	r.mu.Lock()
	if _, exists := r.plugins[name]; exists {
		r.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrAlreadyRegistered, name)
	}
	r.plugins[name] = p
	r.mu.Unlock()

	ctx := &scopedContext{registry: r, owner: name}
	if err := p.Init(ctx); err != nil {
		// Roll back the partially-registered plugin.
		r.removeOwner(name)
		return fmt.Errorf("plugin %q init: %w", name, err)
	}
	return nil
}

// Unregister disposes a plugin and removes every extension point it owns.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	p, ok := r.plugins[name]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	// Dispose outside the lock so plugin teardown cannot deadlock against the
	// registry (e.g. by publishing an event during Dispose).
	p.Dispose()
	r.removeOwner(name)
	return nil
}

// removeOwner deletes the plugin entry and all registrations owned by it.
func (r *Registry) removeOwner(owner string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, owner)
	for k, c := range r.commands {
		if c.owner == owner {
			delete(r.commands, k)
		}
	}
	for k, d := range r.dataSources {
		if d.owner == owner {
			delete(r.dataSources, k)
		}
	}
	for k, v := range r.views {
		if v.owner == owner {
			delete(r.views, k)
		}
	}
	for event, subs := range r.subscribers {
		kept := subs[:0]
		for _, s := range subs {
			if s.owner != owner {
				kept = append(kept, s)
			}
		}
		if len(kept) == 0 {
			delete(r.subscribers, event)
		} else {
			r.subscribers[event] = kept
		}
	}
}

// Plugins returns the metadata of every registered plugin, sorted by name.
func (r *Registry) Plugins() []PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]PluginMetadata, 0, len(r.plugins))
	for name, p := range r.plugins {
		out = append(out, PluginMetadata{Name: name, Version: p.Version()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Command returns the handler registered under name, if any.
func (r *Registry) Command(name string) (CommandHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.commands[name]
	if !ok {
		return nil, false
	}
	return c.handler, true
}

// DataSource returns the factory registered under name, if any.
func (r *Registry) DataSource(name string) (DataSourceFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.dataSources[name]
	if !ok {
		return nil, false
	}
	return d.factory, true
}

// View returns the factory registered under name, if any.
func (r *Registry) View(name string) (ViewFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.views[name]
	if !ok {
		return nil, false
	}
	return v.factory, true
}

// Subscribe registers a host-side handler for a named event. It returns an
// unsubscribe function that removes exactly this handler.
func (r *Registry) Subscribe(event string, handler EventHandler) func() {
	return r.subscribe("", event, handler)
}

func (r *Registry) subscribe(owner, event string, handler EventHandler) func() {
	if handler == nil {
		return func() {}
	}
	sub := boundSubscriber{owner: owner, handler: handler}
	r.mu.Lock()
	r.subscribers[event] = append(r.subscribers[event], sub)
	r.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			subs := r.subscribers[event]
			// Remove the first stored entry matching this subscriber.
			kept := subs[:0]
			removed := false
			for _, s := range subs {
				if !removed && sameHandler(s, sub) {
					removed = true
					continue
				}
				kept = append(kept, s)
			}
			if len(kept) == 0 {
				delete(r.subscribers, event)
			} else {
				r.subscribers[event] = kept
			}
		})
	}
}

// sameHandler compares two subscribers by owner and function pointer identity.
func sameHandler(a, b boundSubscriber) bool {
	return a.owner == b.owner &&
		fmt.Sprintf("%p", a.handler) == fmt.Sprintf("%p", b.handler)
}

// --- Registry as host PluginContext -----------------------------------------

// RegisterCommand registers a host-owned command.
func (r *Registry) RegisterCommand(name string, handler CommandHandler) {
	r.registerCommand("", name, handler)
}

// RegisterDataSource registers a host-owned data source factory.
func (r *Registry) RegisterDataSource(name string, factory DataSourceFactory) {
	r.registerDataSource("", name, factory)
}

// RegisterView registers a host-owned view factory.
func (r *Registry) RegisterView(name string, factory ViewFactory) {
	r.registerView("", name, factory)
}

// PublishEvent dispatches data to every subscriber of name.
func (r *Registry) PublishEvent(name string, data any) {
	r.mu.RLock()
	subs := make([]boundSubscriber, len(r.subscribers[name]))
	copy(subs, r.subscribers[name])
	r.mu.RUnlock()

	// Invoke outside the lock so handlers may publish further events without
	// deadlocking.
	for _, s := range subs {
		s.handler(data)
	}
}

// --- internal owner-aware registration ---------------------------------------

func (r *Registry) registerCommand(owner, name string, handler CommandHandler) {
	if name == "" || handler == nil {
		return
	}
	r.mu.Lock()
	r.commands[name] = boundCommand{owner: owner, handler: handler}
	r.mu.Unlock()
}

func (r *Registry) registerDataSource(owner, name string, factory DataSourceFactory) {
	if name == "" || factory == nil {
		return
	}
	r.mu.Lock()
	r.dataSources[name] = boundDataSource{owner: owner, factory: factory}
	r.mu.Unlock()
}

func (r *Registry) registerView(owner, name string, factory ViewFactory) {
	if name == "" || factory == nil {
		return
	}
	r.mu.Lock()
	r.views[name] = boundView{owner: owner, factory: factory}
	r.mu.Unlock()
}

// scopedContext is the PluginContext handed to a single plugin. It attributes
// every registration to the plugin so Unregister can tear them down.
type scopedContext struct {
	registry *Registry
	owner    string
}

var _ PluginContext = (*scopedContext)(nil)
var _ PluginContext = (*Registry)(nil)

func (c *scopedContext) RegisterCommand(name string, handler CommandHandler) {
	c.registry.registerCommand(c.owner, name, handler)
}

func (c *scopedContext) RegisterDataSource(name string, factory DataSourceFactory) {
	c.registry.registerDataSource(c.owner, name, factory)
}

func (c *scopedContext) RegisterView(name string, factory ViewFactory) {
	c.registry.registerView(c.owner, name, factory)
}

func (c *scopedContext) PublishEvent(name string, data any) {
	c.registry.PublishEvent(name, data)
}
