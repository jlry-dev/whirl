package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jlry-dev/whirl/internal/config"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/service"
)

const (
	// hubShardCount is the number of serial executors used for per-user events.
	// Events for the same user always land on the same shard, so they are
	// processed in submission order. Different users can run in parallel.
	hubShardCount = 8

	// hubShardQueueSize bounds how many events can wait for a shard. Producers
	// block when the queue is full, which gives natural backpressure instead of
	// spawning unbounded goroutines.
	hubShardQueueSize = 256

	// hubRandomQueueSize bounds the matchmaker mailbox.
	hubRandomQueueSize = 256

	// hubDBTimeout bounds database work done by Hub workers so a slow query
	// cannot stall a shard or the matchmaker forever.
	hubDBTimeout = 10 * time.Second
)

type uid int

func (u uid) String() string {
	return strconv.Itoa(int(u))
}

func (u uid) Int() int {
	return int(u)
}

type ChatHandler interface {
	SocketConnect(w http.ResponseWriter, r *http.Request)
	IssueTicket(w http.ResponseWriter, r *http.Request)
}

type ChatHandlr struct {
	hub        *Hub
	rspHandler *ResponseHandler
	logger     *slog.Logger
	upgrader   websocket.Upgrader
	tickets    *TicketStore
}

func NewChatHandler(logger *slog.Logger, rspHandler *ResponseHandler, hub *Hub, policy *config.OriginPolicy, tickets *TicketStore) ChatHandler {
	return &ChatHandlr{
		logger:     logger,
		rspHandler: rspHandler,
		hub:        hub,
		tickets:    tickets,
		upgrader: websocket.Upgrader{
			// Pages lang from a configured frontend origin ang pwede mo open
			// ug WebSocket. Browser always sends Origin; walay Origin = dili
			// browser, so wala ta ga-check sa same-origin policy.
			CheckOrigin: func(r *http.Request) bool {
				return policy.Allows(r.Header.Get("Origin"))
			},
		},
	}
}

// IssueTicket gives the browser a short-lived single-use ticket for the
// handshake. Dili man kabutang ug Authorization header ang browser sa
// WebSocket, so kani ang gamiton instead of the JWT sa URL.
func (h *ChatHandlr) IssueTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Error("websocket ticket: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		h.logger.Error("websocket ticket: failed to get the userID value out of ctx", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	ticket, err := h.tickets.Issue(userID)
	if err != nil {
		h.logger.Error("websocket ticket: failed to issue ticket", slog.Any("error", err))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	h.rspHandler.JSON(w, http.StatusOK, dto.TicketResponse{
		Ticket:    ticket,
		ExpiresIn: int(TicketTTL / time.Second),
	})
}

func (h *ChatHandlr) SocketConnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		if errors.Is(err, websocket.ErrBadHandshake) {
			h.rspHandler.Error(w, http.StatusBadRequest, "bad request trying to upgrade connection", nil)
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	cl := &Client{
		logger: h.logger,
		userID: uid(userID),
		hub:    h.hub,
		ws:     ws,
		send:   make(chan *Message, 20),
		done:   make(chan struct{}),
	}

	// Register the client before starting its reader. Because the registration
	// and every later event for this user go to the same shard, Connect is
	// guaranteed to run before the first join/message/disconnect.
	registered := make(chan struct{})
	h.hub.submitUser(cl.userID, func() {
		defer close(registered)
		h.hub.Connect(cl)
	}, nil)
	<-registered

	go cl.WriteMessage()
	go cl.ReadMessage()

	h.logger.Info("socket connect: user has established websocket connection", slog.Int("userID", userID))
}

type Client struct {
	logger *slog.Logger

	mu         sync.RWMutex
	inQueue    bool
	userID     uid
	hub        *Hub
	ws         *websocket.Conn
	send       chan *Message
	done       chan struct{}
	closeOnce  sync.Once
	randomPair *Client
}

// pairKey identifies the two participants of a friendship in a canonical
// order, so (a, b) and (b, a) address the same entry.
type pairKey struct {
	low  int
	high int
}

func newPairKey(a, b int) pairKey {
	if a > b {
		a, b = b, a
	}

	return pairKey{low: a, high: b}
}

// pendingRequest is an unanswered friend request for a pair. The pair itself is
// the map key, so only the requester has to be stored; no *Client is kept and
// a reconnect cannot leave a stale pointer behind.
type pendingRequest struct {
	from int
}

type Hub struct {
	frSrv  service.FriendshipService
	msgSrv service.MessageService
	logger *slog.Logger

	clientMU sync.RWMutex
	clients  map[string]*Client

	// friendMU guards friendRequests. It is a leaf lock: no other Hub or
	// database call is made while it is held.
	friendMU       sync.RWMutex
	friendRequests map[pairKey]pendingRequest

	// queue is owned by the matchmaker, but queueMU is also taken by the
	// disconnect path so a client can always be removed from the queue.
	queueMU sync.Mutex
	queue   []*Client

	shards       *shardPool
	randomEvents chan randomEvent
}

func NewHub(frSrv service.FriendshipService, msgSrv service.MessageService, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}

	return &Hub{
		frSrv:  frSrv,
		msgSrv: msgSrv,

		logger:         logger,
		clients:        make(map[string]*Client, 32),
		friendRequests: make(map[pairKey]pendingRequest, 4),
		queue:          []*Client{},

		shards:       newShardPool(hubShardCount, hubShardQueueSize, logger),
		randomEvents: make(chan randomEvent, hubRandomQueueSize),
	}
}

type Message struct {
	Type      string    `json:"type"`
	From      int       `json:"from,omitempty"`
	To        int       `json:"to,omitempty"`
	Code      string    `json:"code,omitempty"` // This is used for error codes
	Content   string    `json:"content,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type job func()

type shard struct {
	id   int
	jobs chan job
}

func newShard(id, capacity int, logger *slog.Logger) *shard {
	s := &shard{
		id:   id,
		jobs: make(chan job, capacity),
	}
	go s.run(logger)
	return s
}

func (s *shard) run(logger *slog.Logger) {
	for j := range s.jobs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("hub shard job panicked",
						slog.Int("shard", s.id),
						slog.Any("panic", r))
				}
			}()
			j()
		}()
	}
}

type shardPool struct {
	shards []*shard
}

func newShardPool(count, capacity int, logger *slog.Logger) *shardPool {
	p := &shardPool{
		shards: make([]*shard, count),
	}
	for i := range p.shards {
		p.shards[i] = newShard(i, capacity, logger)
	}
	return p
}

func (p *shardPool) shardFor(key uint64) *shard {
	return p.shards[key%uint64(len(p.shards))]
}

// submit enqueues j on the shard for key. It blocks while the shard queue is
// full, so callers apply backpressure instead of creating unbounded work. A
// non-nil done channel lets a closing client abandon the wait.
func (p *shardPool) submit(key uint64, j job, done <-chan struct{}) bool {
	if done != nil {
		select {
		case <-done:
			return false
		default:
		}
	}

	s := p.shardFor(key)
	if done == nil {
		s.jobs <- j
		return true
	}

	select {
	case s.jobs <- j:
		return true
	case <-done:
		return false
	}
}

func (h *Hub) submitUser(userID uid, j job, done <-chan struct{}) bool {
	return h.shards.submit(uint64(userID), j, done)
}

type randomEventKind uint8

const (
	randomJoinEvent randomEventKind = iota
	randomLeaveEvent
)

type randomEvent struct {
	kind   randomEventKind
	client *Client
	done   chan struct{}
}

// Run is the matchmaker. It is the single owner of the random-chat decision
// process, so join/leave requests are serialized globally while per-user work
// continues to run in parallel on the shards.
func (h *Hub) Run() {
	for ev := range h.randomEvents {
		h.handleRandomEvent(ev)
	}
}

func (h *Hub) handleRandomEvent(ev randomEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("hub random event panicked", slog.Any("panic", r))
		}
		if ev.done != nil {
			close(ev.done)
		}
	}()

	switch ev.kind {
	case randomJoinEvent:
		h.JoinRandom(ev.client)
	case randomLeaveEvent:
		h.LeaveRandom(ev.client)
	default:
		h.logger.Warn("hub: unknown random event kind", slog.Int("kind", int(ev.kind)))
	}
}

// requestRandom sends a matchmaking request from a user shard and waits for the
// matchmaker to finish it. Because the shard is serial, a join submitted before
// a message is guaranteed to be applied before that message is handled.
func (h *Hub) requestRandom(kind randomEventKind, c *Client) {
	select {
	case <-c.done:
		return
	default:
	}

	ev := randomEvent{
		kind:   kind,
		client: c,
		done:   make(chan struct{}),
	}

	select {
	case h.randomEvents <- ev:
	case <-c.done:
		return
	}

	select {
	case <-ev.done:
	case <-c.done:
	}
}

func (h *Hub) Connect(c *Client) {
	clientID := c.userID.String()

	h.clientMU.Lock()
	old := h.clients[clientID]
	h.clients[clientID] = c
	h.clientMU.Unlock()

	// A new connection takes over from a stale one. Closing the old client
	// here also unblocks its ReadMessage/WriteMessage goroutines.
	if old != nil && old != c {
		h.detach(old)
	}
}

func (h *Hub) Disconnect(c *Client) {
	h.detach(c)
}

// detach removes c from the active client map and random-chat state.
// It is safe to call more than once and will never remove a newer connection
// that has taken over c's user ID.
func (h *Hub) detach(c *Client) {
	clientID := c.userID.String()

	h.clientMU.Lock()
	if h.clients[clientID] == c {
		delete(h.clients, clientID)
	}
	h.clientMU.Unlock()

	// LeaveRandom is safe here because it takes queueMU, which is also taken by
	// the matchmaker around every queue/pair mutation.
	h.LeaveRandom(c)

	// Unanswered requests die with the session that made them.
	h.clearPendingFor(c.userID.Int())

	c.close()
}

func (h *Hub) isCurrentClient(c *Client) bool {
	h.clientMU.RLock()
	defer h.clientMU.RUnlock()
	return h.clients[c.userID.String()] == c
}

func (h *Hub) clientByID(userID int) (*Client, bool) {
	h.clientMU.RLock()
	defer h.clientMU.RUnlock()
	c, ok := h.clients[strconv.Itoa(userID)]
	return c, ok
}

func (h *Hub) JoinRandom(c *Client) {
	c.mu.RLock()
	alreadyInQueue := c.inQueue
	isPaired := c.randomPair != nil
	c.mu.RUnlock()

	if alreadyInQueue {
		h.logger.Info("user tried to join random but is already in queue", slog.String("user_id", c.userID.String()))
		c.enqueue(&Message{
			Type:    "error",
			Code:    "ALREADY_IN_QUEUE",
			To:      c.userID.Int(),
			Content: "The user is already queueing for random chat",
		})

		return
	}

	if isPaired {
		h.logger.Info("user tried to join random but is already connected", slog.String("user_id", c.userID.String()))
		c.enqueue(&Message{
			Type:    "error",
			Code:    "CONNECTED_TO_RANDOM",
			To:      c.userID.Int(),
			Content: "The user is already connected in a random chat",
		})

		return
	}

	// Check if the requestor is still online. This matters when a join request
	// was buffered behind a reconnect or a disconnect.
	if !h.isCurrentClient(c) {
		h.logger.Info("user tried to join random but is already offline", slog.String("user_id", c.userID.String()))
		return
	}

	pair, ok := h.popCandidate(c)
	if !ok {
		h.requeue(c)
		return
	}

	hasRelationship := false
	if h.frSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), hubDBTimeout)
		var err error
		hasRelationship, err = h.frSrv.CheckStatus(ctx, &dto.FriendshipDTO{
			From: c.userID.Int(),
			To:   pair.userID.Int(),
		})
		cancel()

		if err != nil {
			h.logger.Error("join random: there was an error trying to check friendship status", slog.String("error", err.Error()))
			h.requeue(pair)
			h.requeue(c)
			return
		}
	}

	if hasRelationship {
		// Requeue both, but do not immediately retry them. Retrying here
		// would loop forever when the only two queued users are related.
		h.requeue(pair)
		h.requeue(c)
		return
	}

	if h.tryPair(c, pair) {
		h.logger.Info("a pair have been whirled", slog.String(c.userID.String(), pair.userID.String()))

		c.enqueue(&Message{
			Type:    "random_joined",
			To:      c.userID.Int(),
			Content: "You have been whirled",
		})

		pair.enqueue(&Message{
			Type:    "random_joined",
			To:      pair.userID.Int(),
			Content: "You have been whirled",
		})

		return
	}

	// A concurrent disconnect changed the state while the relationship check
	// was in flight. Requeue and let the next event retry.
	h.requeue(pair)
	h.requeue(c)
}

// popCandidate removes the next online client from the queue. The caller does
// not own queueMU while doing the relationship check.
func (h *Hub) popCandidate(c *Client) (*Client, bool) {
	h.queueMU.Lock()
	defer h.queueMU.Unlock()

	for len(h.queue) > 0 {
		pair := h.queue[0]
		h.queue = h.queue[1:]

		pair.mu.Lock()
		pair.inQueue = false
		pair.mu.Unlock()

		if pair == c {
			continue
		}
		if !h.isCurrentClient(pair) {
			continue
		}

		return pair, true
	}

	return nil, false
}

// tryPair commits the pairing under queueMU so it cannot interleave with a
// disconnect or another matchmaking decision.
func (h *Hub) tryPair(c, pair *Client) bool {
	h.queueMU.Lock()
	defer h.queueMU.Unlock()

	if !h.isCurrentClient(c) || !h.isCurrentClient(pair) {
		return false
	}

	c.mu.Lock()
	cEligible := !c.inQueue && c.randomPair == nil
	c.mu.Unlock()

	pair.mu.Lock()
	pairEligible := !pair.inQueue && pair.randomPair == nil
	pair.mu.Unlock()

	if !cEligible || !pairEligible {
		return false
	}

	c.mu.Lock()
	pair.mu.Lock()

	c.inQueue = false
	pair.inQueue = false
	c.randomPair = pair
	pair.randomPair = c

	pair.mu.Unlock()
	c.mu.Unlock()

	return true
}

// requeue puts c back into the random queue if it is still online and still
// eligible. A client is eligible when it is neither queued nor paired.
func (h *Hub) requeue(c *Client) {
	h.queueMU.Lock()
	defer h.queueMU.Unlock()

	if !h.isCurrentClient(c) {
		return
	}

	c.mu.Lock()
	if c.inQueue || c.randomPair != nil {
		c.mu.Unlock()
		return
	}
	c.inQueue = true
	c.mu.Unlock()

	h.queue = append(h.queue, c)
}

func (h *Hub) LeaveRandom(c *Client) {
	h.queueMU.Lock()
	defer h.queueMU.Unlock()

	pair := c.pair()

	h.logger.Info("user is leaving random queue", slog.String("user", c.userID.String()))

	if pair != nil {
		c.mu.Lock()
		c.inQueue = false
		if c.randomPair == pair {
			c.randomPair = nil
		}
		c.mu.Unlock()

		pair.mu.Lock()
		if pair.randomPair == c {
			pair.randomPair = nil
		}
		pair.mu.Unlock()

		h.removeFromQueueLocked(c)

		if !pair.enqueue(&Message{
			Type:    "notification",
			Content: "random_pair_left",
		}) {
			h.logger.Info("random_leave: notification dropped: pair already disconnected")
		}

		return
	}

	c.mu.Lock()
	c.inQueue = false
	c.mu.Unlock()

	h.removeFromQueueLocked(c)
}

func (h *Hub) removeFromQueueLocked(c *Client) {
	for i, cl := range h.queue {
		if cl == c {
			h.queue = append(h.queue[:i], h.queue[i+1:]...)
			return
		}
	}
}

func (h *Hub) HandleMessage(m *Message) {
	switch m.Type {
	case "direct_message":
		h.handleDirectMessage(m)
	case "message_random":
		h.handleRandomMessage(m)
	case "friend_request":
		h.handleFriendRequest(m)
	}
}

func (h *Hub) handleDirectMessage(m *Message) {
	client, ok := h.clientByID(m.From)
	if !ok {
		// the client is offline
		return
	}

	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}

	ctx, cancel := context.WithTimeout(context.Background(), hubDBTimeout)
	defer cancel()

	// save to database
	if err := h.msgSrv.StoreMessage(ctx, m.From, m.To, m.Content, m.Timestamp); err != nil {
		h.logger.Error("handle message:" + err.Error())
		h.logger.Error("direct message error : failed to store message")

		client.enqueue(&Message{
			Type:    "error",
			Code:    "SEND_MESSAGE_FAILED",
			Content: "Random pair has disconnected",
		})

		return
	}

	receiver, online := h.clientByID(m.To)
	if online {
		receiver.enqueue(m)
	}
}

func (h *Hub) handleRandomMessage(m *Message) {
	sender, ok := h.clientByID(m.From)
	if !ok {
		return
	}

	// Resolve the pair at processing time, not at read time. This is what
	// makes "join_random" followed immediately by "message_random" safe: both
	// events are handled by the same shard in order.
	pair := sender.pair()
	if pair == nil {
		sender.enqueue(&Message{
			Type:    "error",
			Code:    "CONNECTION_NOT_EXIST",
			Content: "You are not connected to a random user",
		})
		return
	}

	receiver, online := h.clientByID(pair.userID.Int())
	if !online {
		sender.logger.Error("message random error : pair is offline")
		sender.enqueue(&Message{
			Type:    "error",
			Code:    "SEND_MESSAGE_FAILED",
			Content: "Random pair has disconnected",
		})
		return
	}

	m.To = pair.userID.Int()

	// We clear out the from id because we want it to be anonymous on the frontend
	m.From = 0

	receiver.enqueue(m)
}

func (h *Hub) handleFriendRequest(m *Message) {
	sender, online := h.clientByID(m.From)
	if !online {
		return
	}

	pair := sender.pair()
	if pair == nil {
		sender.enqueue(&Message{
			Type:    "error",
			Code:    "CONNECTION_NOT_EXIST",
			Content: "You are not connected to a random user",
		})
		return
	}

	// The receiver always comes from server-side pair state, never the client.
	m.To = pair.userID.Int()
	if m.To == m.From {
		return
	}

	receiver, receiverOnline := h.clientByID(m.To)
	key := newPairKey(m.From, m.To)

	h.friendMU.Lock()

	if !receiverOnline {
		// A request cannot be answered while the other user is offline. Only
		// this pair is dropped, never the sender's requests elsewhere.
		delete(h.friendRequests, key)
		h.friendMU.Unlock()
		return
	}

	pending, exists := h.friendRequests[key]

	if !exists {
		h.friendRequests[key] = pendingRequest{from: m.From}
		h.friendMU.Unlock()

		receiver.enqueue(&Message{Type: "friend_request"})
		return
	}

	if pending.from == m.From {
		// The same user asked twice. Do not write anything; re-ask the other
		// side so the request stays visible.
		h.friendMU.Unlock()

		receiver.enqueue(&Message{Type: "friend_request"})
		return
	}

	// pending.from == m.To: both participants asked. Claim the request before
	// touching the database so a concurrent acceptance cannot insert twice.
	delete(h.friendRequests, key)
	h.friendMU.Unlock()

	h.acceptFriendRequest(key, m.From, m.To, sender, receiver)
}

// acceptFriendRequest stores the friendship after the pending request has been
// claimed. It runs without friendMU held, so a slow query cannot stall other
// friend requests.
func (h *Hub) acceptFriendRequest(key pairKey, from, to int, sender, receiver *Client) {
	if h.frSrv == nil {
		h.restorePending(key, pendingRequest{from: to})
		h.notifyFriendRequestFailed(sender, receiver)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), hubDBTimeout)
	defer cancel()

	err := h.frSrv.AddFriend(ctx, &dto.FriendshipDTO{
		From: from,
		To:   to,
	})
	if err != nil && !errors.Is(err, service.ErrFriendshipExists) {
		h.logger.Error("friend request: failed to create friendship", slog.String("error", err.Error()))

		// Let the pair retry: put the original request back unless a newer one
		// already took the slot.
		h.restorePending(key, pendingRequest{from: to})
		h.notifyFriendRequestFailed(sender, receiver)
		return
	}

	sender.enqueue(&Message{Type: "friend_request_success"})
	receiver.enqueue(&Message{Type: "friend_request_success"})
}

// restorePending returns a claimed request to the map. A newer request for the
// same pair is never overwritten.
func (h *Hub) restorePending(key pairKey, req pendingRequest) {
	h.friendMU.Lock()
	defer h.friendMU.Unlock()

	if _, exists := h.friendRequests[key]; !exists {
		h.friendRequests[key] = req
	}
}

// clearPendingFor drops every pending request involving userID. Unanswered
// requests must not outlive the session that made or received them.
func (h *Hub) clearPendingFor(userID int) {
	h.friendMU.Lock()
	defer h.friendMU.Unlock()

	for key := range h.friendRequests {
		if key.low == userID || key.high == userID {
			delete(h.friendRequests, key)
		}
	}
}

func (h *Hub) notifyFriendRequestFailed(sender, receiver *Client) {
	sender.enqueue(&Message{Type: "friend_request_failed"})
	receiver.enqueue(&Message{Type: "friend_request_failed"})
}

func (c *Client) WriteMessage() {
	defer c.close()

	for {
		select {
		case <-c.done:
			return
		case m := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteJSON(m); err != nil {
				c.logger.Error("socket error: failed to write JSON", slog.String("error", err.Error()))
				return
			}
		}
	}
}

func (c *Client) ReadMessage() {
	defer func() {
		c.close()
		// Every queued event for this client runs before the disconnect,
		// because both are submitted to the same shard in order.
		c.hub.submitUser(c.userID, func() {
			c.hub.Disconnect(c)
		}, nil)
	}()

	// Add maxsize read
	// Add timer
	for {
		// TODO: validate data
		var msg Message
		if err := c.ws.ReadJSON(&msg); err != nil {
			c.logger.Error("read message: closing socket got error", slog.String("error", err.Error()))
			return
		}

		msg.From = c.userID.Int()
		msg.Timestamp = time.Now()

		switch msg.Type {
		case "join_random":
			if !c.hub.submitUser(c.userID, func() {
				c.hub.requestRandom(randomJoinEvent, c)
			}, c.done) {
				return
			}

		case "leave_random":
			if !c.hub.submitUser(c.userID, func() {
				c.hub.requestRandom(randomLeaveEvent, c)
			}, c.done) {
				return
			}

		case "message_random", "direct_message", "friend_request":
			m := msg
			if !c.hub.submitUser(c.userID, func() {
				c.hub.HandleMessage(&m)
			}, c.done) {
				return
			}

		default:
			c.enqueue(&Message{
				Type:    "error",
				Code:    "INVALID_MESSAGE_TYPE",
				Content: "The server does not recognize the message type",
			})
		}
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.ws != nil {
			_ = c.ws.Close()
		}
	})
}

func (c *Client) enqueue(m *Message) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.send <- m:
		return true
	default:
		// Slow client: close it so it can reconnect instead of blocking the Hub.
		c.logger.Warn("send buffer full; closing slow client",
			slog.String("userID", c.userID.String()))
		c.close()
		return false
	}
}

func (c *Client) pair() *Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.randomPair
}
