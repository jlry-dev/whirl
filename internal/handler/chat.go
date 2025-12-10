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
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/service"
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
}

type ChatHandlr struct {
	hub        *Hub
	rspHandler *ResponseHandler
	logger     *slog.Logger
}

func NewChatHandler(logger *slog.Logger, rspHandler *ResponseHandler, hub *Hub) ChatHandler {
	return &ChatHandlr{
		logger:     logger,
		rspHandler: rspHandler,
		hub:        hub,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: create a proper origin check
	},
}

func (h *ChatHandlr) SocketConnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if errors.Is(err, websocket.ErrBadHandshake) {
			h.rspHandler.Error(w, http.StatusBadRequest, "bad request trying to upgrade connection", nil)
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	sendCh := make(chan *Message, 20)
	cl := &Client{
		logger:      h.logger,
		userID:      uid(userID),
		hub:         h.hub,
		ws:          ws,
		send:        sendCh,
		isConnected: true,
	}

	go cl.ReadMessage()
	go cl.WriteMessage()

	h.hub.connect <- cl
	h.logger.Info("socket connect: user has established websocket connection", slog.Int("userID", userID))
}

type Client struct {
	logger      *slog.Logger
	mu          sync.RWMutex
	inQueue     bool // Indicator for when the client is queueing for random chat
	userID      uid
	hub         *Hub
	ws          *websocket.Conn
	send        chan *Message
	randomPair  *Client // This is for the omegle like feature where we pair the user with another user
	isConnected bool
}

type Hub struct {
	frSrv  service.FriendshipService
	msgSrv service.MessageService
	logger *slog.Logger

	clientMU       sync.RWMutex
	clients        map[string]*Client
	friendMU       sync.RWMutex
	friendRequests map[string]*Client
	queueMU        sync.RWMutex
	queue          []*Client

	connect    chan *Client
	disconnect chan *Client

	messages    chan *Message
	randomJoin  chan *Client
	randomLeave chan *Client
}

func NewHub(frSrv service.FriendshipService, logger *slog.Logger) *Hub {
	return &Hub{
		frSrv: frSrv,

		logger:         logger,
		clients:        make(map[string]*Client, 32),
		friendRequests: make(map[string]*Client, 4),
		connect:        make(chan *Client, 12),
		disconnect:     make(chan *Client, 12),
		messages:       make(chan *Message, 32),
		queue:          []*Client{},
		randomJoin:     make(chan *Client, 12),
		randomLeave:    make(chan *Client, 12),
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

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.connect:
			go h.Connect(c)

		case c := <-h.disconnect:
			go h.Disconnect(c)

		case c := <-h.randomJoin:
			go h.JoinRandom(c)

		case c := <-h.randomLeave:
			go h.LeaveRandom(c)

		case m := <-h.messages:
			go h.HandleMessage(m)
		}
	}
}

func (h *Hub) Connect(c *Client) {
	h.clientMU.Lock()
	h.clients[c.userID.String()] = c
	h.clientMU.Unlock()
}

func (h *Hub) Disconnect(c *Client) {
	h.clientMU.Lock()
	clientID := c.userID.String()

	h.randomLeave <- c
	if _, ok := h.clients[clientID]; ok {

		delete(h.clients, clientID)
		c.ws.Close()
		close(c.send)
	}
	h.clientMU.Unlock()
}

func (h *Hub) JoinRandom(c *Client) {
	c.mu.RLock()
	alreadyInQueue := c.inQueue
	isPaired := c.randomPair != nil
	c.mu.RUnlock()

	if alreadyInQueue {
		h.logger.Info("user tried to join random but is already in queue", slog.String("user_id", c.userID.String()))
		c.send <- &Message{
			Type:    "error",
			Code:    "ALREADY_IN_QUEUE",
			To:      c.userID.Int(),
			Content: "The user is already queueing for random chat",
		}

		return
	}

	if isPaired {
		h.logger.Info("user tried to join random but is already connected", slog.String("user_id", c.userID.String()))
		c.send <- &Message{
			Type:    "error",
			Code:    "CONNECTED_TO_RANDOM",
			To:      c.userID.Int(),
			Content: "The user is already connected in a random chat",
		}

		return
	}

	h.clientMU.RLock()
	_, online := h.clients[c.userID.String()]
	h.clientMU.RUnlock()

	// Check if the requestor is still online
	// Para ni sa cases where the random join request is buffered
	// Pero ang requestor kay offline na diay
	if !online {
		h.logger.Info("user tried to join random but is already offline", slog.String("user_id", c.userID.String()))
		return
	}

	h.queueMU.Lock()
	defer h.queueMU.Unlock()

	if len(h.queue) > 0 {
		pair := h.queue[0]
		h.queue = h.queue[1:]

		// check nato if online ba ang napili nga pair
		// basin na disconnect na
		h.clientMU.RLock()
		_, pairOnline := h.clients[pair.userID.String()]
		h.clientMU.RUnlock()

		if !pairOnline {
			h.randomJoin <- c

			return
		}

		c.mu.RLock()
		pair.mu.RLock()

		// We check relationship, if they are both in a relationship (friends or blocked) they don't get paired together
		hasRelationship, err := h.frSrv.CheckStatus(context.Background(), &dto.FriendshipDTO{ // WARN: we may need to add proper context deadline
			From: c.userID.Int(),
			To:   pair.userID.Int(),
		})
		if err != nil {
			h.logger.Error("join random: there was an error trying to check friendship status", slog.String("error", err.Error()))
			pair.mu.RUnlock()
			c.mu.RUnlock()

			return
		}

		if hasRelationship {
			// they are in a relationship so we we put both of them back into the queue
			pair.mu.RUnlock()
			c.mu.RUnlock()

			h.randomJoin <- pair
			h.randomJoin <- c
		}

		pair.mu.RUnlock()
		c.mu.RUnlock()

		c.mu.Lock()
		pair.mu.Lock()

		pair.inQueue = false
		c.inQueue = false

		c.randomPair = pair
		pair.randomPair = c

		pair.mu.Unlock()
		c.mu.Unlock()

		c.send <- &Message{
			Type:    "random_joined",
			To:      c.userID.Int(),
			Content: "You have been whirled",
		}

		pair.send <- &Message{
			Type:    "random_joined",
			To:      pair.userID.Int(),
			Content: "You have been whirled",
		}

		return
	}

	c.mu.Lock()
	if c.inQueue || c.randomPair != nil {
		c.mu.Unlock()
		return
	}

	c.inQueue = true
	h.queue = append(h.queue, c)

	c.mu.Unlock()
}

func (h *Hub) LeaveRandom(c *Client) {
	c.mu.RLock()
	pair := c.randomPair
	c.mu.RUnlock()

	// If the user has a pair then
	// we clear the pair's random pair field

	if pair != nil {
		c.mu.Lock()
		pair.mu.Lock()

		c.randomPair = nil
		pair.randomPair = nil

		pair.mu.Unlock()
		c.mu.Unlock()

		select {
		case pair.send <- &Message{
			Type:    "notification",
			Content: "random_pair_left",
		}:
			// sent successfully
		default:
			h.logger.Info("random_leave: notification dropped: pair already disconnected")
		}

		return
	}

	// not paired so we leave the queue
	h.queueMU.Lock()
	for i, cl := range h.queue {
		if c.userID == cl.userID {
			h.queue = append(h.queue[:i], h.queue[i+1:]...)
			break // only remove first occurrence
		}
	}
	h.queueMU.Unlock()
}

func (h *Hub) HandleMessage(m *Message) {
	switch m.Type {
	case "direct_message":
		h.clientMU.RLock()
		client, ok := h.clients[strconv.Itoa(m.From)]
		if !ok {
			// the client is offline
			return
		}
		h.clientMU.RUnlock()

		m.Timestamp = time.Now()
		// save to database
		// WARN: again we need to properly create a context with proper deadline
		err := h.msgSrv.StoreMessage(context.Background(), m.From, m.To, m.Content, m.Timestamp)
		if err != nil {
			h.logger.Error("direct message error : failed to store message")

			// This is an error because it should have been
			select {
			case client.send <- &Message{
				Type:    "error",
				Code:    "SEND_MESSAGE_FAILED",
				Content: "Random pair has disconnected",
			}:
			default:
				// ignore on fail
			}

			return
		}

		h.clientMU.RLock()
		receiver, online := h.clients[strconv.Itoa(m.To)]
		h.clientMU.RUnlock()
		if online {
			// save to database
			select {
			case receiver.send <- m:
			default:
			}
		}

	case "message_random":

		h.clientMU.RLock()
		receiver, online := h.clients[strconv.Itoa(m.To)]
		c, cOnline := h.clients[strconv.Itoa(m.From)]
		h.clientMU.RUnlock()

		if !online {
			// The pair is offline so we leave the chat
			if cOnline {

				c.logger.Error("message random error : pair is offline")

				// This is an error because it should have been
				select {
				case c.send <- &Message{
					Type:    "error",
					Code:    "SEND_MESSAGE_FAILED",
					Content: "Random pair has disconnected",
				}:
				default:
					// ignore on fail
				}
			}

			return
		}

		// We clear out the from id because we want it to be anonymous on the frontend
		m.From = 0

		select {
		case receiver.send <- m:
		default:
		}

	case "friend_request":
		senderID := strconv.Itoa(m.From)
		receiverID := strconv.Itoa(m.To)

		h.clientMU.RLock()
		sender, sOnline := h.clients[senderID]
		receiver, rOnline := h.clients[receiverID]
		h.clientMU.RUnlock()

		h.friendMU.Lock()
		defer h.friendMU.Unlock()

		if !sOnline || !rOnline {
			// Clear any request made by either pariticipant
			delete(h.friendRequests, receiverID)
			delete(h.friendRequests, senderID)
			return
		}

		// if

		// Check if ang receiver naay pending nga friend friend request
		// E accept dayn if naa na
		if _, pending := h.friendRequests[receiverID]; pending {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			// Save to database
			err := h.frSrv.AddFriend(ctx, &dto.FriendshipDTO{
				From: m.From,
				To:   m.To,
			})
			if err != nil {
				select {
				case sender.send <- &Message{
					Type: "friend_request_failed",
				}:
				default:
				}

				select {
				case receiver.send <- &Message{
					Type: "friend_request_failed",
				}:
				default:
				}
				return
			}

			// Notify
			select {
			case sender.send <- &Message{
				Type: "friend_request_success",
			}:
			default:
				// Just skip, just tryna avoid a panic :>
			}

			select {
			case receiver.send <- &Message{
				Type: "friend_request_success",
			}:
			default:
			}

			return
		}

		h.friendRequests[senderID] = receiver
		select {
		case receiver.send <- &Message{
			Type: "friend_request",
		}:
		default:
		}
	}
}

func (c *Client) WriteMessage() {
	defer func() {
		// INFO: my decision to close the websocket on write is message is because what if there were buffered messages
		c.ws.Close()
	}()

	if !c.isConnected {
	}

	for {
		m, ok := <-c.send

		if !ok {
			c.logger.Info("write message: closing socket", slog.String("userID", c.userID.String()))
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.ws.WriteJSON(m); err != nil {
			c.logger.Error("socket error: failed to write JSON", slog.String("error", err.Error()))
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

	}
}

func (c *Client) ReadMessage() {
	defer func() {
		// INFO: when the websocket closes it should get disconnected
		c.hub.disconnect <- c
	}()
	// Add maxsize read
	// Add timer
	for {
		// TODO: validate data
		var msg Message
		if err := c.ws.ReadJSON(&msg); err != nil {
			c.logger.Error("read message: closing socket got error", slog.String("error", err.Error()))
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		switch msg.Type {
		case "join_random":
			c.hub.randomJoin <- c
		case "leave_random":
			c.hub.randomLeave <- c
		case "message_random":
			msg.From = c.userID.Int()
			if pair := c.randomPair; pair == nil {
				c.send <- &Message{
					Type:    "error",
					Code:    "CONNECTION_NOT_EXIST",
					Content: "You are not connected to a random user",
				}
			} else {
				msg.To = pair.userID.Int()
				c.hub.messages <- &msg
			}

		case "direct_message":
			msg.From = c.userID.Int()
			c.hub.messages <- &msg

		case "friend_request":
			msg.From = c.userID.Int()
			msg.To = c.randomPair.userID.Int()

			c.hub.messages <- &msg

		default:
			c.send <- &Message{
				Type:    "error",
				Code:    "INVALID_MESSAGE_TYPE",
				Content: "The server does not recognize the message type",
			}
		}

	}
}
