package brackets

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	Room     string
	IsClosed bool
	Mu       sync.Mutex
}

type WebSocketMessage struct {
	Type    string      `json:"type"`              // Тип сообщения, например, "BRACKET_UPDATED", "MATCH_UPDATED"
	Payload interface{} `json:"payload"`           // Полезная нагрузка (данные сообщения)
	RoomID  string      `json:"room_id,omitempty"` // ID комнаты (турнира), к которой относится сообщение
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Hub struct {
	clients    map[*Client]bool
	Broadcast  chan []byte  // <--- Изменено на Broadcast
	Register   chan *Client // <--- Изменено на Register
	Unregister chan *Client // <--- Изменено на Unregister
	rooms      map[string]map[*Client]bool
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),  // <--- Используем Broadcast
		Register:   make(chan *Client), // <--- Используем Register
		Unregister: make(chan *Client), // <--- Используем Unregister
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register: // <--- Используем h.Register
			h.mu.Lock()
			if _, ok := h.rooms[client.Room]; !ok {
				h.rooms[client.Room] = make(map[*Client]bool)
			}
			h.rooms[client.Room][client] = true
			log.Printf("Client registered to room %s. Total clients in room: %d", client.Room, len(h.rooms[client.Room]))
			h.mu.Unlock()

		case client := <-h.Unregister: // <--- Используем h.Unregister
			h.mu.Lock()
			if _, ok := h.rooms[client.Room]; ok {
				if _, okClient := h.rooms[client.Room][client]; okClient {
					client.Mu.Lock()
					if !client.IsClosed {
						close(client.Send)
						client.IsClosed = true
					}
					client.Mu.Unlock()
					delete(h.rooms[client.Room], client)
					if len(h.rooms[client.Room]) == 0 {
						delete(h.rooms, client.Room)
						log.Printf("Room %s closed as it's empty.", client.Room)
					} else {
						log.Printf("Client unregistered from room %s. Total clients in room: %d", client.Room, len(h.rooms[client.Room]))
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.Broadcast: // <--- Используем h.Broadcast
			h.mu.RLock()
			// Этот цикл по h.clients может быть не нужен, если все клиенты всегда в комнатах.
			// Если нужен, то нужно обеспечить, чтобы client.Send был доступен и h.clients обновлялся корректно.
			// Либо удалить этот блок, если все сообщения идут через BroadcastToRoom.
			for client := range h.clients { // Осторожно с этим блоком, если clients не управляется так же, как rooms
				client.Mu.Lock()
				if client.IsClosed {
					client.Mu.Unlock()
					continue
				}
				select {
				case client.Send <- message:
				default: // Канал клиента полон или закрыт
					// Не закрываем канал здесь повторно, если он уже закрыт в Unregister
					// log.Printf("Client send channel full or closed in broadcast. Removing client.")
					// delete(h.clients, client) // Удаление из общей карты клиентов
				}
				client.Mu.Unlock()
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToRoom отправляет сообщение всем клиентам в указанной комнате.
func (h *Hub) BroadcastToRoom(roomID string, message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	roomClients, ok := h.rooms[roomID]
	if !ok {
		log.Printf("No clients in room %s to broadcast to.", roomID)
		return
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshalling message for room %s: %v", roomID, err)
		return
	}

	log.Printf("Broadcasting to room %s: %s", roomID, string(messageBytes))
	for client := range roomClients {
		client.Mu.Lock()
		if client.IsClosed {
			client.Mu.Unlock()
			continue
		}
		select {
		case client.Send <- messageBytes:
		default:
			log.Printf("Client's send channel full or closed for room %s. Skipping.", roomID)
		}
		client.Mu.Unlock()
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c // <--- Используем c.Hub.Unregister (поле Hub экспортируемое, Unregister тоже)
		c.Conn.Close()
		c.Mu.Lock()
		c.IsClosed = true
		c.Mu.Unlock()
		log.Printf("Client readPump closed for room %s", c.Room)
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			log.Printf("Client in room %s disconnected: %v", c.Room, err)
			break
		}
		log.Printf("Received message from client in room %s: %s (will be ignored by default)", c.Room, message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		c.Mu.Lock()
		c.IsClosed = true
		c.Mu.Unlock()
		log.Printf("Client writePump closed for room %s", c.Room)
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				log.Printf("Hub closed client channel for room %s.", c.Room)
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Error getting next writer for client in room %s: %v", c.Room, err)
				return
			}
			w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				log.Printf("Error closing writer for client in room %s: %v", c.Room, err)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Error sending ping to client in room %s: %v", c.Room, err)
				return
			}
		}
	}
}
