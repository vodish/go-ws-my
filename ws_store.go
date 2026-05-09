package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WsStore хранит отображения между UUID и WebSocket соединениями
// и обеспечивает потокобезопасный доступ.
type WsStore struct {
	mu        sync.RWMutex
	clients   map[string]*websocket.Conn // UUID -> соединение
	clientIDs map[*websocket.Conn]string // соединение -> UUID
}

// NewWsStore создаёт новый экземпляр WsStore.
func NewWsStore() *WsStore {
	return &WsStore{
		clients:   make(map[string]*websocket.Conn),
		clientIDs: make(map[*websocket.Conn]string),
	}
}

// Add добавляет новое соединение в хранилище и возвращает присвоенный UUID.
func (ws *WsStore) Add(conn *websocket.Conn) string {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	clientUUID := uuid.New().String()
	ws.clients[clientUUID] = conn
	ws.clientIDs[conn] = clientUUID
	return clientUUID
}

// Remove удаляет соединение из хранилища по указателю на соединение.
func (ws *WsStore) Remove(conn *websocket.Conn) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if uuid, ok := ws.clientIDs[conn]; ok {
		delete(ws.clients, uuid)
		delete(ws.clientIDs, conn)
	}
}

// RemoveByUUID удаляет соединение по UUID.
func (ws *WsStore) RemoveByUUID(uuid string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if conn, ok := ws.clients[uuid]; ok {
		delete(ws.clients, uuid)
		delete(ws.clientIDs, conn)
	}
}

// GetConn возвращает соединение по UUID и флаг наличия.
func (ws *WsStore) GetConn(uuid string) (*websocket.Conn, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	conn, ok := ws.clients[uuid]
	return conn, ok
}

// GetUUID возвращает UUID по соединению и флаг наличия.
func (ws *WsStore) GetUUID(conn *websocket.Conn) (string, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	uuid, ok := ws.clientIDs[conn]
	return uuid, ok
}

// Broadcast отправляет сообщение всем подключённым клиентам, кроме исключённого.
// Если exclude == nil, сообщение отправляется всем клиентам.
func (ws *WsStore) Broadcast(message []byte, exclude *websocket.Conn) {
	ws.mu.RLock()
	// Создаём копию карты для итерации, чтобы не держать блокировку при отправке
	clientsCopy := make(map[string]*websocket.Conn)
	for u, c := range ws.clients {
		clientsCopy[u] = c
	}
	ws.mu.RUnlock()

	for _, client := range clientsCopy {
		if client == exclude {
			continue
		}
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
			// Удаляем отключённого клиента
			ws.Remove(client)
		}
	}
}

// SendToUUID отправляет сообщение конкретному клиенту по UUID.
// Возвращает ошибку, если клиент не найден или произошла ошибка отправки.
func (ws *WsStore) SendToUUID(uuid string, message []byte) error {
	ws.mu.RLock()
	client, ok := ws.clients[uuid]
	ws.mu.RUnlock()

	if !ok {
		return fmt.Errorf("клиент с UUID %s не найден", uuid)
	}

	err := client.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		log.Printf("Ошибка отправки сообщения клиенту %s: %v", uuid, err)
		ws.Remove(client)
		return err
	}

	log.Printf("Сообщение отправлено клиенту %s: %s", uuid, string(message))
	return nil
}

// SendToMultiple отправляет сообщения нескольким клиентам, переданным в карте UUID -> сообщение.
func (ws *WsStore) SendToMultiple(clients map[string][]byte) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	for uuid, message := range clients {
		client, ok := ws.clients[uuid]
		if !ok {
			log.Printf("Клиент с UUID %s не найден", uuid)
			continue
		}

		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Ошибка отправки сообщения клиенту %s: %v", uuid, err)
			// Удаляем отключённого клиента
			ws.Remove(client)
		} else {
			log.Printf("Сообщение отправлено клиенту %s: %s", uuid, string(message))
		}
	}
}

// Count возвращает количество активных клиентов.
func (ws *WsStore) Count() int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	return len(ws.clients)
}

// GetAllUUIDs возвращает слайс всех UUID активных клиентов.
func (ws *WsStore) GetAllUUIDs() []string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	uuids := make([]string, 0, len(ws.clients))
	for u := range ws.clients {
		uuids = append(uuids, u)
	}
	return uuids
}

// GetAllConns возвращает слайс всех активных соединений.
func (ws *WsStore) GetAllConns() []*websocket.Conn {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	conns := make([]*websocket.Conn, 0, len(ws.clients))
	for _, c := range ws.clients {
		conns = append(conns, c)
	}
	return conns
}
