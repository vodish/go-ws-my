package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ClientStore хранит отображения между UUID и WebSocket соединениями
// и обеспечивает потокобезопасный доступ.
type ClientStore struct {
	mu        sync.RWMutex
	clients   map[string]*websocket.Conn // UUID -> соединение
	clientIDs map[*websocket.Conn]string // соединение -> UUID
}

// NewClientStore создаёт новый экземпляр ClientStore.
func NewClientStore() *ClientStore {
	return &ClientStore{
		clients:   make(map[string]*websocket.Conn),
		clientIDs: make(map[*websocket.Conn]string),
	}
}

// Add добавляет новое соединение в хранилище и возвращает присвоенный UUID.
func (cs *ClientStore) Add(conn *websocket.Conn) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	clientUUID := uuid.New().String()
	cs.clients[clientUUID] = conn
	cs.clientIDs[conn] = clientUUID
	return clientUUID
}

// Remove удаляет соединение из хранилища по указателю на соединение.
func (cs *ClientStore) Remove(conn *websocket.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if uuid, ok := cs.clientIDs[conn]; ok {
		delete(cs.clients, uuid)
		delete(cs.clientIDs, conn)
	}
}

// RemoveByUUID удаляет соединение по UUID.
func (cs *ClientStore) RemoveByUUID(uuid string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if conn, ok := cs.clients[uuid]; ok {
		delete(cs.clients, uuid)
		delete(cs.clientIDs, conn)
	}
}

// GetConn возвращает соединение по UUID и флаг наличия.
func (cs *ClientStore) GetConn(uuid string) (*websocket.Conn, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	conn, ok := cs.clients[uuid]
	return conn, ok
}

// GetUUID возвращает UUID по соединению и флаг наличия.
func (cs *ClientStore) GetUUID(conn *websocket.Conn) (string, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	uuid, ok := cs.clientIDs[conn]
	return uuid, ok
}

// Broadcast отправляет сообщение всем подключённым клиентам, кроме исключённого.
// Если exclude == nil, сообщение отправляется всем клиентам.
func (cs *ClientStore) Broadcast(message []byte, exclude *websocket.Conn) {
	cs.mu.RLock()
	// Создаём копию карты для итерации, чтобы не держать блокировку при отправке
	clientsCopy := make(map[string]*websocket.Conn)
	for u, c := range cs.clients {
		clientsCopy[u] = c
	}
	cs.mu.RUnlock()

	for _, client := range clientsCopy {
		if client == exclude {
			continue
		}
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
			// Удаляем отключённого клиента
			cs.Remove(client)
		}
	}
}

// SendToUUID отправляет сообщение конкретному клиенту по UUID.
// Возвращает ошибку, если клиент не найден или произошла ошибка отправки.
func (cs *ClientStore) SendToUUID(uuid string, message []byte) error {
	cs.mu.RLock()
	client, ok := cs.clients[uuid]
	cs.mu.RUnlock()

	if !ok {
		return fmt.Errorf("клиент с UUID %s не найден", uuid)
	}

	err := client.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		log.Printf("Ошибка отправки сообщения клиенту %s: %v", uuid, err)
		cs.Remove(client)
		return err
	}

	log.Printf("Сообщение отправлено клиенту %s: %s", uuid, string(message))
	return nil
}

// SendToMultiple отправляет сообщения нескольким клиентам, переданным в карте UUID -> сообщение.
func (cs *ClientStore) SendToMultiple(clients map[string][]byte) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for uuid, message := range clients {
		client, ok := cs.clients[uuid]
		if !ok {
			log.Printf("Клиент с UUID %s не найден", uuid)
			continue
		}

		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Ошибка отправки сообщения клиенту %s: %v", uuid, err)
			// Удаляем отключённого клиента
			cs.Remove(client)
		} else {
			log.Printf("Сообщение отправлено клиенту %s: %s", uuid, string(message))
		}
	}
}

// Count возвращает количество активных клиентов.
func (cs *ClientStore) Count() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return len(cs.clients)
}

// GetAllUUIDs возвращает слайс всех UUID активных клиентов.
func (cs *ClientStore) GetAllUUIDs() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	uuids := make([]string, 0, len(cs.clients))
	for u := range cs.clients {
		uuids = append(uuids, u)
	}
	return uuids
}

// GetAllConns возвращает слайс всех активных соединений.
func (cs *ClientStore) GetAllConns() []*websocket.Conn {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	conns := make([]*websocket.Conn, 0, len(cs.clients))
	for _, c := range cs.clients {
		conns = append(conns, c)
	}
	return conns
}