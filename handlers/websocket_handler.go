package handlers

import (
	"log"
	"net/http"

	"github.com/Dosada05/tournament-system/brackets" // Убедись, что путь к пакету brackets правильный
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// В продакшене здесь должна быть проверка Origin,
		// чтобы разрешать подключения только с доверенных доменов.
		// Например:
		// origin := r.Header.Get("Origin")
		// return origin == "http://yourfrontend.com" || origin == "https://yourfrontend.com"
		return true // Для разработки разрешаем все
	},
}

type WebSocketHandler struct {
	hub *brackets.Hub
	// Здесь могут быть другие зависимости, если нужны, например, TournamentService
	// для проверки существования турнира перед созданием комнаты.
	// tournamentService services.TournamentService
}

func NewWebSocketHandler(hub *brackets.Hub /*, ts services.TournamentService */) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		// tournamentService: ts,
	}
}

// ServeWs обрабатывает WebSocket запросы для конкретного турнира.
// Клиент должен подключаться к /ws/tournaments/{tournamentID}
func (h *WebSocketHandler) ServeWs(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "tournamentID")
	if tournamentIDStr == "" {
		http.Error(w, "Missing tournamentID", http.StatusBadRequest)
		return
	}

	// Опционально: Проверить, существует ли турнир с таким ID,
	// используя tournamentService, прежде чем создавать комнату.
	// _, err := h.tournamentService.GetTournamentByID(r.Context(), tournamentID, 0) // 0 - если не важен currentUserID
	// if err != nil {
	//    log.Printf("Error getting tournament %s for WebSocket: %v", tournamentIDStr, err)
	//	  // services.ErrTournamentNotFound
	//    http.NotFound(w, r) // Или другая ошибка в зависимости от типа err
	//    return
	// }

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection for tournament %s: %v", tournamentIDStr, err)
		// upgrader.Upgrade сам отправляет HTTP ошибку клиенту, так что здесь просто логируем.
		return
	}
	log.Printf("WebSocket connection upgraded for tournament %s", tournamentIDStr)

	// ID комнаты будет соответствовать ID турнира
	roomID := "tournament_" + tournamentIDStr

	client := &brackets.Client{
		Hub:  h.hub,
		Conn: conn,
		Send: make(chan []byte, 256), // Буферизированный канал
		Room: roomID,
	}
	client.Hub.Register <- client

	// Запускаем горутины для чтения и записи в WebSocket соединение для этого клиента.
	// Эти горутины будут работать, пока клиент не отключится.
	go client.WritePump()
	go client.ReadPump()

	log.Printf("Client successfully registered and pumps started for room %s.", roomID)

	// После регистрации можно отправить клиенту текущее состояние сетки, если оно уже есть.
	// Например:
	// currentBracketState := h.getBracketStateForTournament(tournamentIDStr)
	// if currentBracketState != nil {
	//    messageBytes, _ := json.Marshal(currentBracketState) // Предполагается, что currentBracketState можно маршализовать в JSON
	//    client.Send <- messageBytes
	// }
}
