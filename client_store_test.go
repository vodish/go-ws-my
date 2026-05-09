package main

import (
	"testing"

	"github.com/gorilla/websocket"
)

// TestNewClientStore проверяет создание нового хранилища клиентов
func TestNewClientStore(t *testing.T) {
	store := NewWsStore()
	if store == nil {
		t.Error("NewClientStore вернул nil")
	}
	if store.Count() != 0 {
		t.Errorf("Новое хранилище должно быть пустым, но содержит %d клиентов", store.Count())
	}
}

// TestAddAndRemove проверяет добавление и удаление клиентов
func TestAddAndRemove(t *testing.T) {
	store := NewWsStore()

	// Создаем мок соединение
	conn := &websocket.Conn{}

	// Добавляем клиента
	uuid := store.Add(conn)
	if uuid == "" {
		t.Error("Add вернул пустой UUID")
	}
	if store.Count() != 1 {
		t.Errorf("После добавления одного клиента Count() = %d, ожидается 1", store.Count())
	}

	// Проверяем, что можем получить соединение по UUID
	retrievedConn, ok := store.GetConn(uuid)
	if !ok {
		t.Error("GetConn не нашёл добавленного клиента")
	}
	if retrievedConn != conn {
		t.Error("GetConn вернул неверное соединение")
	}

	// Проверяем, что можем получить UUID по соединению
	retrievedUUID, ok := store.GetUUID(conn)
	if !ok {
		t.Error("GetUUID не нашёл UUID для добавленного соединения")
	}
	if retrievedUUID != uuid {
		t.Errorf("GetUUID вернул %s, ожидается %s", retrievedUUID, uuid)
	}

	// Удаляем по соединению
	store.Remove(conn)
	if store.Count() != 0 {
		t.Errorf("После Remove Count() = %d, ожидается 0", store.Count())
	}

	// Добавляем снова
	uuid2 := store.Add(conn)
	if uuid2 == uuid {
		t.Error("Новый UUID должен отличаться от предыдущего")
	}

	// Удаляем по UUID
	store.RemoveByUUID(uuid2)
	if store.Count() != 0 {
		t.Errorf("После RemoveByUUID Count() = %d, ожидается 0", store.Count())
	}
}

// TestBroadcast проверяет рассылку сообщений без паники (пропускаем, если нет реальных соединений)
func TestBroadcast(t *testing.T) {
	// Этот тест требует реальных WebSocket соединений, поэтому пропускаем
	t.Skip("Тест Broadcast требует реальных WebSocket соединений, пропускаем в unit-тестах")
}

// TestSendToUUID проверяет отправку сообщения конкретному клиенту (пропускаем)
func TestSendToUUID(t *testing.T) {
	t.Skip("Тест SendToUUID требует реальных WebSocket соединений, пропускаем в unit-тестах")
}

// TestSendToMultiple проверяет отправку нескольким клиентам (пропускаем)
func TestSendToMultiple(t *testing.T) {
	t.Skip("Тест SendToMultiple требует реальных WebSocket соединений, пропускаем в unit-тестах")
}

// TestCountAndGetAll проверяет методы Count, GetAllUUIDs, GetAllConns
func TestCountAndGetAll(t *testing.T) {
	store := NewWsStore()

	// Проверяем пустое хранилище
	if store.Count() != 0 {
		t.Errorf("Пустое хранилище: Count() = %d, ожидается 0", store.Count())
	}
	if len(store.GetAllBots()) != 0 {
		t.Error("GetAllUUIDs должен возвращать пустой слайс для пустого хранилища")
	}
	if len(store.GetAllConns()) != 0 {
		t.Error("GetAllConns должен возвращать пустой слайс для пустого хранилища")
	}

	// Добавляем клиентов
	conn1 := &websocket.Conn{}
	conn2 := &websocket.Conn{}
	uuid1 := store.Add(conn1)
	uuid2 := store.Add(conn2)

	// Проверяем Count
	if store.Count() != 2 {
		t.Errorf("После добавления двух клиентов Count() = %d, ожидается 2", store.Count())
	}

	// Проверяем GetAllUUIDs
	uuids := store.GetAllBots()
	if len(uuids) != 2 {
		t.Errorf("GetAllUUIDs вернул %d UUID, ожидается 2", len(uuids))
	}
	// Проверяем, что оба UUID присутствуют
	uuidMap := make(map[string]bool)
	for _, u := range uuids {
		uuidMap[u] = true
	}
	if !uuidMap[uuid1] || !uuidMap[uuid2] {
		t.Error("GetAllUUIDs не вернул все добавленные UUID")
	}

	// Проверяем GetAllConns
	conns := store.GetAllConns()
	if len(conns) != 2 {
		t.Errorf("GetAllConns вернул %d соединений, ожидается 2", len(conns))
	}
	connMap := make(map[*websocket.Conn]bool)
	for _, c := range conns {
		connMap[c] = true
	}
	if !connMap[conn1] || !connMap[conn2] {
		t.Error("GetAllConns не вернул все добавленные соединения")
	}
}

// TestConcurrentAccess проверяет потокобезопасность (базовая проверка)
func TestConcurrentAccess(t *testing.T) {
	store := NewWsStore()
	done := make(chan bool)

	// Горутина для добавления клиентов
	go func() {
		for i := 0; i < 100; i++ {
			store.Add(&websocket.Conn{})
		}
		done <- true
	}()

	// Горутина для чтения Count
	go func() {
		for i := 0; i < 100; i++ {
			store.Count()
		}
		done <- true
	}()

	// Ожидаем завершения
	<-done
	<-done

	// Проверяем, что нет паники и Count возвращает разумное значение
	count := store.Count()
	if count < 0 || count > 100 {
		t.Errorf("Некорректное количество клиентов после конкурентного доступа: %d", count)
	}
}
