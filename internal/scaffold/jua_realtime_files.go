package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaRealtimeFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		// Phase 1: Event Bus
		filepath.Join(apiRoot, "jua", "events", "bus.go"):        eventsBusGo(),
		filepath.Join(apiRoot, "jua", "events", "memory_bus.go"): eventsMemoryBusGo(),
		filepath.Join(apiRoot, "jua", "events", "redis_bus.go"):  eventsRedisBusGo(),
		filepath.Join(apiRoot, "jua", "events", "publisher.go"):  eventsPublisherGo(),
		filepath.Join(apiRoot, "jua", "events", "init.go"):       eventsInitGo(),

		// Phase 2: Real-time Hub
		filepath.Join(apiRoot, "jua", "realtime", "hub.go"):     realtimeHubGo(),
		filepath.Join(apiRoot, "jua", "realtime", "helpers.go"): realtimeHelpersGo(),
		filepath.Join(apiRoot, "jua", "realtime", "init.go"):    realtimeInitGo(),

		// Phase 3: SSE Handler
		filepath.Join(apiRoot, "jua", "realtime", "sse.go"): realtimeSSEGo(),

		// Phase 4: WebSocket Handler
		filepath.Join(apiRoot, "jua", "realtime", "ws.go"): realtimeWSGo(),

		// Phase 5: Middleware + Routes
		filepath.Join(apiRoot, "jua", "realtime", "middleware.go"): realtimeMiddlewareGo(),
		filepath.Join(apiRoot, "jua", "realtime", "routes.go"):     realtimeRoutesGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	// Phase 6: Frontend SDK (skip for API-only architectures)
	if opts.ShouldIncludeWeb() {
		webRoot := filepath.Join(root, "apps", "web")
		webFiles := map[string]string{
			filepath.Join(webRoot, "jua", "realtime", "use-sse.ts"):            realtimeUseSSETS(),
			filepath.Join(webRoot, "jua", "realtime", "use-websocket.ts"):      realtimeUseWebSocketTS(),
			filepath.Join(webRoot, "jua", "realtime", "realtime-provider.tsx"): realtimeProviderTSX(),
			filepath.Join(webRoot, "jua", "realtime", "connection-status.tsx"): realtimeConnectionStatusTSX(),
		}
		for path, content := range webFiles {
			if err := writeFile(path, content); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
		}
	}

	return nil
}

// ─── Phase 1: Event Bus ────────────────────────────────────────────────────

func eventsBusGo() string {
	return `package events

import "time"

// EventType is a typed event identifier flowing through the bus.
type EventType string

// Built-in Jua event types. Add your own domain constants below.
const (
	// Payment events
	PaymentCompleted EventType = "payment.completed"
	PaymentFailed    EventType = "payment.failed"
	PaymentRefunded  EventType = "payment.refunded"

	// User events
	UserCreated     EventType = "user.created"
	UserDeleted     EventType = "user.deleted"
	UserLoggedIn    EventType = "user.logged_in"
	UserLoginFailed EventType = "user.login_failed"

	// Subscription events
	SubscriptionActivated EventType = "subscription.activated"
	SubscriptionCancelled EventType = "subscription.cancelled"
	SubscriptionExpired   EventType = "subscription.expired"
	SubscriptionPastDue   EventType = "subscription.past_due"

	// Tenant events
	TenantCreated   EventType = "tenant.created"
	TenantSuspended EventType = "tenant.suspended"

	// USSD events
	USSDSessionCompleted EventType = "ussd.session.completed"

	// SMS events
	SMSDelivered EventType = "sms.delivered"
	SMSFailed    EventType = "sms.failed"

	// Order / custom events
	OrderCreated   EventType = "order.created"
	OrderUpdated   EventType = "order.updated"
	OrderCompleted EventType = "order.completed"
)

// Event is the universal message flowing through the bus.
// TenantID is always required — events must never leak across tenants.
type Event struct {
	ID        string                 // UUID v4
	Type      EventType
	TenantID  string                 // Required for multi-tenant isolation
	UserID    string                 // Optional — target a specific user
	Channel   string                 // Optional — named channel (e.g. "order:123")
	Payload   map[string]interface{}
	CreatedAt time.Time
}

// Handler processes a single event.
type Handler func(event Event)

// Bus is the shared event bus interface.
// Implementations: MemoryBus (single instance), RedisBus (multi-instance).
type Bus interface {
	// Publish fires an event to all matching subscribers (non-blocking).
	Publish(event Event)

	// Subscribe registers a handler for a specific event type.
	// Returns an unsubscribe function — call it to stop receiving.
	Subscribe(eventType EventType, handler Handler) func()

	// SubscribeAll registers a handler for every event regardless of type.
	// Returns an unsubscribe function.
	SubscribeAll(handler Handler) func()
}
`
}

func eventsMemoryBusGo() string {
	return `package events

import "sync"

// memoryBus is a thread-safe in-process event bus.
// Use for single-instance deployments or local development.
type memoryBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]subEntry
	allSubs     []subEntry
	nextID      uint64
}

type subEntry struct {
	id      uint64
	handler Handler
}

// NewMemoryBus returns a thread-safe in-memory Bus.
func NewMemoryBus() Bus {
	return &memoryBus{
		subscribers: make(map[EventType][]subEntry),
	}
}

func (b *memoryBus) Publish(event Event) {
	b.mu.RLock()
	typeSubs := make([]subEntry, len(b.subscribers[event.Type]))
	copy(typeSubs, b.subscribers[event.Type])
	allSubs := make([]subEntry, len(b.allSubs))
	copy(allSubs, b.allSubs)
	b.mu.RUnlock()

	for _, s := range typeSubs {
		go s.handler(event)
	}
	for _, s := range allSubs {
		go s.handler(event)
	}
}

func (b *memoryBus) Subscribe(eventType EventType, handler Handler) func() {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subscribers[eventType] = append(b.subscribers[eventType], subEntry{id: id, handler: handler})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subscribers[eventType]
		for i, s := range subs {
			if s.id == id {
				b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

func (b *memoryBus) SubscribeAll(handler Handler) func() {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.allSubs = append(b.allSubs, subEntry{id: id, handler: handler})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, s := range b.allSubs {
			if s.id == id {
				b.allSubs = append(b.allSubs[:i], b.allSubs[i+1:]...)
				return
			}
		}
	}
}
`
}

func eventsRedisBusGo() string {
	return `package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

// redisBus uses Redis Pub/Sub for cross-instance event delivery.
// All published events go to Redis; the listener delivers them locally.
// Falls back to MemoryBus if Redis is unavailable.
type redisBus struct {
	client *redis.Client
	local  Bus
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRedisBus returns a Redis-backed Bus.
// Falls back to in-memory if the connection fails.
func NewRedisBus(redisURL string) Bus {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[jua/events] Redis URL error: %v — using memory bus", err)
		return NewMemoryBus()
	}

	client := redis.NewClient(opt)
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[jua/events] Redis unavailable: %v — using memory bus", err)
		return NewMemoryBus()
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &redisBus{
		client: client,
		local:  NewMemoryBus(),
		ctx:    ctx,
		cancel: cancel,
	}
	go b.listen()
	log.Println("[jua/events] Redis event bus ready")
	return b
}

// Publish sends the event to Redis; the listener delivers it to local handlers.
func (b *redisBus) Publish(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[jua/events] marshal error: %v", err)
		b.local.Publish(event)
		return
	}
	ch := "jua:events:" + event.TenantID
	if err := b.client.Publish(b.ctx, ch, data).Err(); err != nil {
		log.Printf("[jua/events] Redis publish error: %v — delivering locally", err)
		b.local.Publish(event)
	}
}

func (b *redisBus) Subscribe(eventType EventType, handler Handler) func() {
	return b.local.Subscribe(eventType, handler)
}

func (b *redisBus) SubscribeAll(handler Handler) func() {
	return b.local.SubscribeAll(handler)
}

func (b *redisBus) listen() {
	pubsub := b.client.PSubscribe(b.ctx, "jua:events:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-b.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("[jua/events] unmarshal error: %v", err)
				continue
			}
			b.local.Publish(event)
		}
	}
}
`
}

func eventsPublisherGo() string {
	return `package events

import (
	"crypto/rand"
	"fmt"
	"time"
)

// DefaultBus is the global event bus used throughout the application.
// Initialized by Init() — available after app startup.
var DefaultBus Bus = NewMemoryBus()

func newEventID() string {
	var b [16]byte
	rand.Read(b[:]) //nolint:errcheck
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Publish fires a tenant-scoped event to all subscribers.
func Publish(eventType EventType, tenantID string, payload map[string]interface{}) {
	DefaultBus.Publish(Event{
		ID:        newEventID(),
		Type:      eventType,
		TenantID:  tenantID,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// PublishForUser fires an event targeted at a specific user.
func PublishForUser(eventType EventType, tenantID, userID string, payload map[string]interface{}) {
	DefaultBus.Publish(Event{
		ID:        newEventID(),
		Type:      eventType,
		TenantID:  tenantID,
		UserID:    userID,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// PublishToChannel fires an event on a named channel (e.g. "order:123").
func PublishToChannel(channel string, tenantID string, payload map[string]interface{}) {
	DefaultBus.Publish(Event{
		ID:        newEventID(),
		Type:      EventType("channel." + channel),
		TenantID:  tenantID,
		Channel:   channel,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// PublishCustom fires a custom (app-defined) event type.
func PublishCustom(eventType string, tenantID string, payload map[string]interface{}) {
	DefaultBus.Publish(Event{
		ID:        newEventID(),
		Type:      EventType(eventType),
		TenantID:  tenantID,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}
`
}

func eventsInitGo() string {
	return `package events

import (
	"log"
	"os"
)

// Init initialises the global DefaultBus.
// Call once on app startup before handling requests.
//
//	events.Init(cfg.RedisURL)
func Init(redisURL string) {
	busType := os.Getenv("REALTIME_BUS")
	if busType == "" {
		if redisURL != "" {
			busType = "redis"
		} else {
			busType = "memory"
		}
	}

	switch busType {
	case "redis":
		if redisURL == "" {
			log.Println("[jua/events] REALTIME_BUS=redis but REDIS_URL is empty — using memory bus")
			DefaultBus = NewMemoryBus()
		} else {
			DefaultBus = NewRedisBus(redisURL)
		}
	default:
		log.Println("[jua/events] Using in-memory event bus (single instance only)")
		DefaultBus = NewMemoryBus()
	}
}
`
}

// ─── Phase 2: Real-time Hub ───────────────────────────────────────────────

func realtimeHelpersGo() string {
	return `package realtime

import (
	"crypto/rand"
	"fmt"
)

// newConnID generates a random 8-byte hex connection ID.
func newConnID() string {
	var b [8]byte
	rand.Read(b[:]) //nolint:errcheck
	return fmt.Sprintf("%x", b)
}
`
}

func realtimeHubGo() string {
	return `package realtime

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"` + "{{MODULE}}" + `/jua/events"
)

// Connection represents a single active client connection (SSE or WebSocket).
type Connection struct {
	ID        string
	UserID    string
	TenantID  string
	Channels  []string   // named channel subscriptions
	Send      chan []byte // outbound message buffer (256 messages)
	ConnType  string     // "sse" or "ws"
	CreatedAt time.Time
	LastPing  time.Time
}

// BroadcastMessage routes an event to the correct set of connections.
type BroadcastMessage struct {
	TenantID string       // required — never broadcast across tenants
	UserID   string       // optional — empty = all tenant connections
	Channel  string       // optional — empty = no channel filter
	Event    events.Event
}

// HubStats provides connection metrics for the admin dashboard.
type HubStats struct {
	TotalConnections    int            ` + "`" + `json:"total_connections"` + "`" + `
	ConnectionsByTenant map[string]int ` + "`" + `json:"connections_by_tenant"` + "`" + `
	SSEConnections      int            ` + "`" + `json:"sse_connections"` + "`" + `
	WSConnections       int            ` + "`" + `json:"ws_connections"` + "`" + `
}

// Hub manages all active real-time connections.
// All state mutations go through operation channels — no locks in the main loop.
type Hub struct {
	connections map[string]*Connection
	byTenant    map[string]map[string]*Connection
	byUser      map[string]map[string]*Connection
	byChannel   map[string]map[string]*Connection

	register   chan *Connection
	unregister chan *Connection
	broadcast  chan BroadcastMessage

	mu sync.RWMutex
}

// NewHub creates a Hub. Call go hub.Run(bus) after creation.
func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		byTenant:    make(map[string]map[string]*Connection),
		byUser:      make(map[string]map[string]*Connection),
		byChannel:   make(map[string]map[string]*Connection),
		register:    make(chan *Connection, 64),
		unregister:  make(chan *Connection, 64),
		broadcast:   make(chan BroadcastMessage, 512),
	}
}

// Run starts the hub's main event loop. Call as a goroutine.
func (h *Hub) Run(bus events.Bus) {
	// Forward all bus events to connected clients
	bus.SubscribeAll(func(event events.Event) {
		h.broadcast <- BroadcastMessage{
			TenantID: event.TenantID,
			UserID:   event.UserID,
			Channel:  event.Channel,
			Event:    event,
		}
	})

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.addConn(conn)
		case conn := <-h.unregister:
			h.removeConn(conn)
		case msg := <-h.broadcast:
			h.route(msg)
		case <-ticker.C:
			h.cleanDeadConnections()
		}
	}
}

// Register adds a connection to the hub.
func (h *Hub) Register(conn *Connection) { h.register <- conn }

// Unregister removes a connection and closes its Send channel.
func (h *Hub) Unregister(conn *Connection) { h.unregister <- conn }

// SendToTenant broadcasts an event to all connections in a tenant.
func (h *Hub) SendToTenant(tenantID string, event events.Event) {
	h.broadcast <- BroadcastMessage{TenantID: tenantID, Event: event}
}

// SendToUser sends an event to all connections of a specific user.
func (h *Hub) SendToUser(userID string, event events.Event) {
	h.broadcast <- BroadcastMessage{TenantID: event.TenantID, UserID: userID, Event: event}
}

// SendToChannel sends an event to all connections subscribed to a channel.
func (h *Hub) SendToChannel(channel string, event events.Event) {
	h.broadcast <- BroadcastMessage{TenantID: event.TenantID, Channel: channel, Event: event}
}

// GetStats returns current connection statistics.
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	stats := HubStats{
		TotalConnections:    len(h.connections),
		ConnectionsByTenant: make(map[string]int),
	}
	for tid, conns := range h.byTenant {
		stats.ConnectionsByTenant[tid] = len(conns)
	}
	for _, conn := range h.connections {
		if conn.ConnType == "sse" {
			stats.SSEConnections++
		} else {
			stats.WSConnections++
		}
	}
	return stats
}

// SubscribeToChannel adds a connection to a named channel index.
func (h *Hub) SubscribeToChannel(conn *Connection, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byChannel[channel] == nil {
		h.byChannel[channel] = make(map[string]*Connection)
	}
	h.byChannel[channel][conn.ID] = conn
	for _, ch := range conn.Channels {
		if ch == channel {
			return
		}
	}
	conn.Channels = append(conn.Channels, channel)
}

// UnsubscribeFromChannel removes a connection from a named channel.
func (h *Hub) UnsubscribeFromChannel(conn *Connection, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.byChannel[channel]; ok {
		delete(ch, conn.ID)
	}
	for i, ch := range conn.Channels {
		if ch == channel {
			conn.Channels = append(conn.Channels[:i], conn.Channels[i+1:]...)
			return
		}
	}
}

func (h *Hub) addConn(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[conn.ID] = conn
	if h.byTenant[conn.TenantID] == nil {
		h.byTenant[conn.TenantID] = make(map[string]*Connection)
	}
	h.byTenant[conn.TenantID][conn.ID] = conn
	if conn.UserID != "" {
		if h.byUser[conn.UserID] == nil {
			h.byUser[conn.UserID] = make(map[string]*Connection)
		}
		h.byUser[conn.UserID][conn.ID] = conn
	}
	log.Printf("[jua/realtime] connected: %s (%s) tenant=%s", conn.ID, conn.ConnType, conn.TenantID)
}

func (h *Hub) removeConn(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.connections[conn.ID]; !exists {
		return
	}
	delete(h.connections, conn.ID)
	if tc := h.byTenant[conn.TenantID]; tc != nil {
		delete(tc, conn.ID)
		if len(tc) == 0 {
			delete(h.byTenant, conn.TenantID)
		}
	}
	if conn.UserID != "" {
		if uc := h.byUser[conn.UserID]; uc != nil {
			delete(uc, conn.ID)
			if len(uc) == 0 {
				delete(h.byUser, conn.UserID)
			}
		}
	}
	for _, channel := range conn.Channels {
		if ch := h.byChannel[channel]; ch != nil {
			delete(ch, conn.ID)
		}
	}
	close(conn.Send)
	log.Printf("[jua/realtime] disconnected: %s (%s)", conn.ID, conn.ConnType)
}

func (h *Hub) route(msg BroadcastMessage) {
	data := marshalEvent(msg.Event)

	h.mu.RLock()
	defer h.mu.RUnlock()

	if msg.Channel != "" {
		if ch, ok := h.byChannel[msg.Channel]; ok {
			for _, conn := range ch {
				if conn.TenantID == msg.TenantID {
					nonBlockingSend(conn, data)
				}
			}
		}
		return
	}

	if msg.UserID != "" {
		if uc, ok := h.byUser[msg.UserID]; ok {
			for _, conn := range uc {
				if conn.TenantID == msg.TenantID {
					nonBlockingSend(conn, data)
				}
			}
		}
		return
	}

	if tc, ok := h.byTenant[msg.TenantID]; ok {
		for _, conn := range tc {
			nonBlockingSend(conn, data)
		}
	}
}

func nonBlockingSend(conn *Connection, data []byte) {
	select {
	case conn.Send <- data:
	default:
		// Client buffer full — drop this message
	}
}

func (h *Hub) cleanDeadConnections() {
	h.mu.RLock()
	var dead []*Connection
	cutoff := time.Now().Add(-90 * time.Second)
	for _, conn := range h.connections {
		if conn.ConnType == "ws" && conn.LastPing.Before(cutoff) {
			dead = append(dead, conn)
		}
	}
	h.mu.RUnlock()
	for _, conn := range dead {
		h.unregister <- conn
	}
}

func marshalEvent(event events.Event) []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"id":        event.ID,
		"type":      string(event.Type),
		"tenantId":  event.TenantID,
		"userId":    event.UserID,
		"channel":   event.Channel,
		"payload":   event.Payload,
		"createdAt": event.CreatedAt,
	})
	return data
}
`
}

func realtimeInitGo() string {
	return `package realtime

import (
	"` + "{{MODULE}}" + `/jua/events"
)

// GlobalHub is the application-wide real-time connection hub.
// Initialized by Init() — available after app startup.
var GlobalHub *Hub

// Init creates the global hub and starts its event loop.
// Call after events.Init().
//
//	realtime.Init(events.DefaultBus)
func Init(bus events.Bus) {
	GlobalHub = NewHub()
	go GlobalHub.Run(bus)
}
`
}

// ─── Phase 3: SSE Handler ─────────────────────────────────────────────────

func realtimeSSEGo() string {
	return `package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"` + "{{MODULE}}" + `/jua/events"
)

// SSEHandler handles Server-Sent Events connections.
// Route: GET /api/realtime/stream
//
// SSE is the primary real-time transport in Jua:
//   - Works over HTTP/1.1, no upgrade required
//   - Browser auto-reconnects with Last-Event-ID replay
//   - Lower overhead than WebSocket for server→client streaming
//   - Ideal for low-bandwidth / African mobile networks
func SSEHandler(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		tenantID := c.GetString("tenantID")
		lastEventID := c.GetHeader("Last-Event-ID")

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no") // disable Nginx response buffering
		c.Header("Access-Control-Allow-Origin", "*")

		conn := &Connection{
			ID:        newConnID(),
			UserID:    userID,
			TenantID:  tenantID,
			Send:      make(chan []byte, 256),
			ConnType:  "sse",
			CreatedAt: time.Now(),
			LastPing:  time.Now(),
		}
		hub.Register(conn)
		defer hub.Unregister(conn)

		// Replay missed events for reconnecting clients
		if lastEventID != "" {
			replayEvents(c, tenantID, userID, lastEventID)
		}

		// Confirm connection
		fmt.Fprintf(c.Writer, "event: connected\ndata: {\"connectionId\":\"%s\"}\n\n", conn.ID)
		c.Writer.Flush()

		clientGone := c.Request.Context().Done()
		keepalive := time.NewTicker(30 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case <-clientGone:
				return
			case message, ok := <-conn.Send:
				if !ok {
					return
				}
				conn.LastPing = time.Now()
				fmt.Fprintf(c.Writer, "%s", string(message))
				c.Writer.Flush()
			case <-keepalive.C:
				fmt.Fprintf(c.Writer, ": keepalive\n\n")
				c.Writer.Flush()
			}
		}
	}
}

// FormatSSEEvent serialises an event into the SSE wire format.
func FormatSSEEvent(event events.Event) []byte {
	data, _ := json.Marshal(event.Payload)
	return []byte(fmt.Sprintf(
		"id: %s\nevent: %s\ndata: %s\n\n",
		event.ID,
		string(event.Type),
		string(data),
	))
}

// replayEvents sends missed events to a reconnecting client.
// Events are stored in Redis sorted set: jua:events:history:{tenantID}
func replayEvents(c *gin.Context, tenantID, userID, afterEventID string) {
	rc, ok := c.Value("redisClient").(*redis.Client)
	if !ok || rc == nil {
		return
	}

	ctx := context.Background()
	key := "jua:events:history:" + tenantID

	results, err := rc.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		log.Printf("[jua/sse] replay error: %v", err)
		return
	}

	var found bool
	for _, r := range results {
		memberStr, ok := r.Member.(string)
		if !ok {
			continue
		}
		var event events.Event
		if err := json.Unmarshal([]byte(memberStr), &event); err != nil {
			continue
		}
		if !found {
			if event.ID == afterEventID {
				found = true
			}
			continue
		}
		// Only replay events for this user or all-tenant events
		if event.UserID != "" && event.UserID != userID {
			continue
		}
		fmt.Fprintf(c.Writer, "%s", string(FormatSSEEvent(event)))
	}
	if found {
		c.Writer.Flush()
	}
}

// StoreEventInHistory persists an event for replay on reconnect.
// Score is the nanosecond timestamp. Keeps last 1000 events per tenant.
func StoreEventInHistory(rc *redis.Client, event events.Event, ttlSeconds int) {
	ctx := context.Background()
	key := "jua:events:history:" + event.TenantID
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	score := float64(event.CreatedAt.UnixNano())
	rc.ZAdd(ctx, key, redis.Z{Score: score, Member: string(data)})
	rc.ZRemRangeByRank(ctx, key, 0, -1001) // keep last 1000
	rc.Expire(ctx, key, time.Duration(ttlSeconds)*time.Second)
}
`
}

// ─── Phase 4: WebSocket Handler ───────────────────────────────────────────

func realtimeWSGo() string {
	return `package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"` + "{{MODULE}}" + `/jua/events"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: restrict origins in production
	},
}

// wsMessage represents an incoming message from a WebSocket client.
type wsMessage struct {
	Type    string                 ` + "`" + `json:"type"` + "`" + `
	Channel string                 ` + "`" + `json:"channel,omitempty"` + "`" + `
	Payload map[string]interface{} ` + "`" + `json:"payload,omitempty"` + "`" + `
}

// WSHandler handles WebSocket connections.
// Route: GET /api/realtime/ws
//
// Use WebSocket when you need bidirectional communication.
// For server→client only, prefer SSE (simpler, lower overhead).
func WSHandler(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		tenantID := c.GetString("tenantID")

		ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[jua/ws] upgrade error: %v", err)
			return
		}
		defer ws.Close()

		conn := &Connection{
			ID:        newConnID(),
			UserID:    userID,
			TenantID:  tenantID,
			Send:      make(chan []byte, 256),
			ConnType:  "ws",
			CreatedAt: time.Now(),
			LastPing:  time.Now(),
		}
		hub.Register(conn)
		defer hub.Unregister(conn)

		ws.WriteJSON(map[string]string{"type": "connected", "connectionId": conn.ID}) //nolint:errcheck

		go wsWritePump(ws, conn)
		wsReadPump(ws, conn, hub, tenantID)
	}
}

func wsWritePump(ws *websocket.Conn, conn *Connection) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case message, ok := <-conn.Send:
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
			if !ok {
				ws.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			if err := ws.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
			conn.LastPing = time.Now()

		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func wsReadPump(ws *websocket.Conn, conn *Connection, hub *Hub, tenantID string) {
	defer hub.Unregister(conn)

	ws.SetReadLimit(4096)
	ws.SetReadDeadline(time.Now().Add(90 * time.Second)) //nolint:errcheck
	ws.SetPongHandler(func(string) error {
		conn.LastPing = time.Now()
		return ws.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	for {
		_, rawMsg, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[jua/ws] read error: %v", err)
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			if msg.Channel != "" {
				hub.SubscribeToChannel(conn, msg.Channel)
				ws.WriteJSON(map[string]string{"type": "subscribed", "channel": msg.Channel}) //nolint:errcheck
			}
		case "unsubscribe":
			if msg.Channel != "" {
				hub.UnsubscribeFromChannel(conn, msg.Channel)
				ws.WriteJSON(map[string]string{"type": "unsubscribed", "channel": msg.Channel}) //nolint:errcheck
			}
		case "ping":
			conn.LastPing = time.Now()
			ws.WriteJSON(map[string]string{"type": "pong"}) //nolint:errcheck
		case "custom_event":
			// Client→server event: publish to bus for server-side handling
			events.PublishCustom("client.message", tenantID, msg.Payload)
		}
	}
}
`
}

// ─── Phase 5: Middleware + Routes ─────────────────────────────────────────

func realtimeMiddlewareGo() string {
	return `package realtime

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"` + "{{MODULE}}" + `/internal/services"
)

// RealtimeAuthMiddleware validates JWT for SSE and WebSocket connections.
//
// Browser EventSource and WebSocket APIs cannot set custom request headers,
// so the JWT is passed as a query parameter: ?token=<jwt>
// The Authorization: Bearer header is also accepted (non-browser clients).
func RealtimeAuthMiddleware(db *gorm.DB, authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer query parameter (required for browser EventSource / WebSocket)
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && parts[0] == "Bearer" {
					token = parts[1]
				}
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "token query parameter or Authorization header required",
				},
			})
			c.Abort()
			return
		}

		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		c.Set("userID", fmt.Sprintf("%d", claims.UserID))
		// tenantID is set by multitenancy middleware if active;
		// for single-tenant apps it defaults to the empty string.
		if c.GetString("tenantID") == "" {
			c.Set("tenantID", "default")
		}
		c.Next()
	}
}
`
}

func realtimeRoutesGo() string {
	return `package realtime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"` + "{{MODULE}}" + `/internal/services"
)

// RegisterRoutes mounts all real-time routes onto the Gin engine.
// Call this from your routes setup after initialising the hub.
//
//	// Jua real-time routes
//	realtime.RegisterRoutes(router, db, authService, realtime.GlobalHub)
func RegisterRoutes(router *gin.Engine, db *gorm.DB, authService *services.AuthService, hub *Hub) {
	rt := router.Group("/api/realtime")
	rt.Use(RealtimeAuthMiddleware(db, authService))
	{
		// Server-Sent Events — primary real-time transport for Africa-optimised apps
		rt.GET("/stream", SSEHandler(hub))

		// WebSocket — use when bidirectional communication is needed
		rt.GET("/ws", WSHandler(hub))

		// Connection statistics — admin only (protect this in production)
		rt.GET("/stats", func(c *gin.Context) {
			c.JSON(http.StatusOK, hub.GetStats())
		})
	}
}
`
}

// ─── Phase 6: Frontend SDK ────────────────────────────────────────────────

func realtimeUseSSETS() string {
	return `import { useState, useEffect, useRef, useCallback } from 'react'

interface SSEOptions {
  onConnect?: () => void
  onDisconnect?: () => void
  onError?: (error: Event) => void
  token?: string
  apiBase?: string
}

interface SSEResult<T = unknown> {
  data: T | null
  connected: boolean
  lastEventId: string
}

function getAuthToken(): string {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem('jua_token') || ''
}

/**
 * useSSE subscribes to one or more Server-Sent Event types.
 * The browser will automatically reconnect on disconnection,
 * sending the Last-Event-ID header for missed-event replay.
 *
 * @example
 * const { data, connected } = useSSE<Payment>('payment.completed')
 * const { data } = useSSE(['order.created', 'order.updated'])
 */
export function useSSE<T = unknown>(
  eventTypes: string | string[],
  options?: SSEOptions
): SSEResult<T> {
  const [data, setData] = useState<T | null>(null)
  const [connected, setConnected] = useState(false)
  const [lastEventId, setLastEventId] = useState('')
  const esRef = useRef<EventSource | null>(null)
  const lastIdRef = useRef('')

  const connect = useCallback(() => {
    const token = options?.token || getAuthToken()
    const base = options?.apiBase || ''
    const params = new URLSearchParams({ token })
    if (lastIdRef.current) params.set('lastEventId', lastIdRef.current)
    const url = ` + "`" + `${base}/api/realtime/stream?${params}` + "`" + `

    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => {
      setConnected(true)
      options?.onConnect?.()
    }

    es.onerror = (e) => {
      setConnected(false)
      options?.onError?.(e)
    }

    const types = Array.isArray(eventTypes) ? eventTypes : [eventTypes]
    types.forEach((type) => {
      es.addEventListener(type, (e: MessageEvent) => {
        lastIdRef.current = e.lastEventId
        setLastEventId(e.lastEventId)
        try {
          setData(JSON.parse(e.data))
        } catch {
          setData(e.data as unknown as T)
        }
      })
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    connect()
    return () => {
      esRef.current?.close()
      setConnected(false)
      options?.onDisconnect?.()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return { data, connected, lastEventId }
}
`
}

func realtimeUseWebSocketTS() string {
	return `import { useState, useEffect, useRef, useCallback } from 'react'

interface WebSocketResult {
  data: unknown
  connected: boolean
  send: (message: object) => void
  subscribe: (channel: string) => void
  unsubscribe: (channel: string) => void
}

function getAuthToken(): string {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem('jua_token') || ''
}

function getWSBaseURL(): string {
  if (typeof window === 'undefined') return 'ws://localhost:8080'
  return window.location.origin.replace(/^https/, 'wss').replace(/^http/, 'ws')
}

/**
 * useWebSocket connects to the Jua real-time WebSocket endpoint.
 * Reconnects automatically after disconnection (3 second delay).
 *
 * @example
 * const { data, connected, subscribe } = useWebSocket()
 * useEffect(() => { subscribe('order:123') }, [])
 */
export function useWebSocket(): WebSocketResult {
  const [data, setData] = useState<unknown>(null)
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout>>()

  const connect = useCallback(() => {
    const token = getAuthToken()
    const url = ` + "`" + `${getWSBaseURL()}/api/realtime/ws?token=${token}` + "`" + `
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      clearTimeout(reconnectRef.current)
    }
    ws.onclose = () => {
      setConnected(false)
      reconnectRef.current = setTimeout(connect, 3000)
    }
    ws.onerror = () => ws.close()
    ws.onmessage = (e) => {
      try { setData(JSON.parse(e.data)) } catch { setData(e.data) }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(reconnectRef.current)
      wsRef.current?.close()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const send = useCallback((message: object) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message))
    }
  }, [])

  const subscribe = useCallback((channel: string) => send({ type: 'subscribe', channel }), [send])
  const unsubscribe = useCallback((channel: string) => send({ type: 'unsubscribe', channel }), [send])

  return { data, connected, send, subscribe, unsubscribe }
}
`
}

func realtimeProviderTSX() string {
	return `'use client'

import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  useCallback,
  type ReactNode,
} from 'react'

interface RealtimeContextType {
  connected: boolean
  /** Register a handler for an event type. Returns an unsubscribe function. */
  on: (eventType: string, handler: (payload: unknown) => void) => () => void
}

const RealtimeContext = createContext<RealtimeContextType>({
  connected: false,
  on: () => () => {},
})

function getAuthToken(): string {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem('jua_token') || ''
}

interface RealtimeProviderProps {
  children: ReactNode
  apiBase?: string
}

/**
 * RealtimeProvider wraps your app with a shared SSE connection.
 * All useRealtime() calls share one connection per page.
 *
 * Add to your root layout:
 *   <RealtimeProvider>{children}</RealtimeProvider>
 */
export function RealtimeProvider({ children, apiBase = '' }: RealtimeProviderProps) {
  const [connected, setConnected] = useState(false)
  const handlersRef = useRef<Map<string, Set<(p: unknown) => void>>>(new Map())
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    const token = getAuthToken()
    if (!token) return

    const url = ` + "`" + `${apiBase}/api/realtime/stream?token=${token}` + "`" + `
    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)

    // Wildcard message handler
    es.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data)
        handlersRef.current.get('*')?.forEach((h) => h(event))
      } catch {}
    }

    return () => { es.close(); setConnected(false) }
  }, [apiBase])

  const on = useCallback((eventType: string, handler: (payload: unknown) => void): (() => void) => {
    if (!handlersRef.current.has(eventType)) {
      handlersRef.current.set(eventType, new Set())
      // Register named EventSource listener
      esRef.current?.addEventListener(eventType, (e: MessageEvent) => {
        try {
          const payload = JSON.parse(e.data)
          handlersRef.current.get(eventType)?.forEach((h) => h(payload))
        } catch {}
      })
    }
    handlersRef.current.get(eventType)!.add(handler)
    return () => { handlersRef.current.get(eventType)?.delete(handler) }
  }, [])

  return (
    <RealtimeContext.Provider value={{ connected, on }}>
      {children}
    </RealtimeContext.Provider>
  )
}

/** useRealtime returns the shared real-time context from RealtimeProvider. */
export function useRealtime() {
  return useContext(RealtimeContext)
}
`
}

func realtimeConnectionStatusTSX() string {
	return `'use client'

import { useRealtime } from './realtime-provider'

/**
 * ConnectionStatus shows a live indicator for the real-time connection.
 * Green dot = connected, pulsing grey = disconnected / reconnecting.
 *
 * Place in your navbar:
 *   <ConnectionStatus />
 */
export function ConnectionStatus() {
  const { connected } = useRealtime()

  return (
    <div className="flex items-center gap-1.5 text-xs text-muted-foreground select-none">
      <span
        className={[
          'h-2 w-2 rounded-full transition-colors',
          connected
            ? 'bg-green-500'
            : 'bg-muted-foreground/40 animate-pulse',
        ].join(' ')}
      />
      <span className="hidden sm:inline">
        {connected ? 'Live' : 'Reconnecting\u2026'}
      </span>
    </div>
  )
}
`
}
