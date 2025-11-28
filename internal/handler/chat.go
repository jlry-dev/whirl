package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

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
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// return a bad request IG?
	}

	uid, ok := r.Context().Value("userID").(int)
	if !ok {
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
	}

	sendCh := make(chan *Message, 20)
	cl := &Client{
		logger: h.logger,
		userID: uid,
		hub:    h.hub,
		ws:     ws,
		send:   sendCh,
	}

	go cl.ReadMessage()
	go cl.WriteMessage()

	h.hub.connect <- cl
}

type Client struct {
	logger     *slog.Logger
	userID     int
	hub        *Hub
	ws         *websocket.Conn
	send       chan *Message
	randomPair *Client // This is for the omegle like feature where we pair the user with another user
}

type Hub struct {
	logger      *slog.Logger
	mu          sync.RWMutex
	clients     map[string]*Client
	connect     chan *Client
	disconnect  chan *Client
	messages    chan *Message
	queue       []*Client
	randomJoin  chan *Client
	randomLeave chan *Client
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		logger:      logger,
		clients:     make(map[string]*Client, 12),
		connect:     make(chan *Client, 12),
		disconnect:  make(chan *Client, 12),
		messages:    make(chan *Message, 32),
		queue:       []*Client{},
		randomJoin:  make(chan *Client, 12),
		randomLeave: make(chan *Client, 12),
	}
}

type Message struct {
	Type    string `json:"type"`
	From    int    `json:"from,omitempty"`
	To      int    `json:"to,omitempty"`
	Code    string `json:"code,omitempty"` // This is used for error codes
	Content string `json:"content,omitempty"`
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.connect:
			h.mu.Lock()

			clientID := strconv.Itoa(c.userID)
			// user already has existing connection
			// so we delete it and replace in with the new connection
			if existing, ok := h.clients[clientID]; ok {
				close(existing.send)
				existing.ws.Close()
			}

			h.clients[clientID] = c
			h.mu.Unlock()

		case c := <-h.disconnect:
			h.mu.Lock()
			clientID := strconv.Itoa(c.userID)
			if _, ok := h.clients[clientID]; ok {

				delete(h.clients, clientID)
				c.ws.Close()
				close(c.send)
			}

			// Leave random chat
			h.randomLeave <- c

			h.mu.Unlock()

		case c := <-h.randomJoin:
			// TODO: what happens if they are still in queue
			h.mu.Lock()
			if c.randomPair != nil {
				c.send <- &Message{
					Type:    "error",
					Code:    "CONNECTED_TO_RANDOM",
					To:      c.userID,
					Content: "The user is already connected in a random chat",
				}
			} else if len(h.queue) > 0 {
				pair := h.queue[0]
				h.queue = h.queue[1:]

				if _, connected := h.clients[strconv.Itoa(pair.userID)]; !connected {
					h.randomJoin <- c
				} else {
					c.randomPair = pair
					pair.randomPair = c

					c.send <- &Message{
						Type:    "random_joined",
						To:      c.userID,
						Content: "You have been whirled",
					}

					pair.send <- &Message{
						Type:    "random_joined",
						To:      pair.userID,
						Content: "You have been whirled",
					}
				}
			} else {
				h.queue = append(h.queue, c)
			}

			h.mu.Unlock()
		case c := <-h.randomLeave:
			h.mu.Lock()
			// If the user has a pair then
			// we clear the pair's random pair field
			if pair := c.randomPair; pair != nil {
				c.randomPair = nil
				pair.randomPair = nil
				pair.send <- &Message{
					Type: "random_partner_left",
				}

			} else {
				// not paired so we leave the queue
				for i, cl := range h.queue {
					if c.userID == cl.userID {
						h.queue = append(h.queue[:i], h.queue[i+1:]...)
					}
				}
			}
			h.mu.Unlock()
		case m := <-h.messages:
			switch m.Type {
			case "message_direct":
				h.mu.RLock()
				if receiver, online := h.clients[strconv.Itoa(m.To)]; !online {
					// receiver is offline
					// save to database
				} else {
					// save to database
					receiver.send <- m
				}
				h.mu.RUnlock()
			case "message_random":
				h.mu.RLock()
				if receiver, online := h.clients[strconv.Itoa(m.To)]; !online {
					// The pair is offline so we leave the chat
					c := h.clients[strconv.Itoa(m.To)]
					// WARN: this could cause an error.
					c.randomPair = nil

					c.logger.Error("message random error : pair is offline")

					// This is an error because it should have been
					c.send <- &Message{
						Type:    "error",
						Code:    "SEND_MESSAGE_FAILED",
						Content: "Random pair has disconnected",
					}
				} else {
					// We clear out the from id because we want it to be anonymous
					m.From = 0
					receiver.send <- m
				}
				h.mu.RUnlock()
			}

		}
	}
}

func (c *Client) WriteMessage() {
	defer func() {
		// INFO: my decision to close the websocket on write is message is because what if there were buffered messages
		c.ws.Close()
	}()

	for {
		m, ok := <-c.send

		if !ok {
			c.logger.Info("socket closed", slog.String("userID", strconv.Itoa(c.userID)))
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.ws.WriteJSON(m); err != nil {
			c.logger.Info("socket error: failed to write JSON", slog.String("error", err.Error()))
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return

			// had an error sending the message, close the websocket
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
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		switch msg.Type {
		case "join_random":
			c.hub.randomJoin <- c
		case "leave_random":
			c.hub.randomLeave <- c
		case "message_random":
			msg.From = c.userID
			if pair := c.randomPair; pair == nil {
				c.send <- &Message{
					Type:    "error",
					Code:    "CONNECTION_NOT_EXIST",
					Content: "You are not connected to a random user",
				}
			} else {
				msg.To = pair.userID
				c.hub.messages <- &msg
			}

		case "message_direct":
			msg.From = c.userID
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
