package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Разрешаем все origin для простоты
		},
	}

	// Хранилище клиентов с UUID ключами
	clients2   = make(map[string]*websocket.Conn) // UUID -> соединение
	clientIDs  = make(map[*websocket.Conn]string) // соединение -> UUID
	mu         sync.Mutex
)

func main() {
	// Загружаем переменные окружения из .env файла
	err := godotenv.Load()
	if err != nil {
		log.Println("Не удалось загрузить .env файл, используем переменные окружения системы")
	}

	// Получаем порт из переменной окружения
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	// Проверяем, что порт является числом
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("Некорректный порт: %s", port)
	}

	// Обслуживаем статические файлы из папки "static"
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	log.Printf("Сервер запущен на http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка при обновлении до WebSocket:", err)
		return
	}
	defer conn.Close()

	// Генерируем уникальный UUID для клиента
	clientUUID := uuid.New().String()

	// Регистрируем нового клиента
	mu.Lock()
	clients2[clientUUID] = conn
	clientIDs[conn] = clientUUID
	mu.Unlock()

	log.Printf("Новое подключение: %s, UUID: %s", r.RemoteAddr, clientUUID)

	// Отправляем приветственное сообщение
	conn.WriteMessage(websocket.TextMessage, []byte("Добро пожаловать в чат!"))

	// Рассылаем уведомление о новом пользователе всем клиентам
	broadcast([]byte("Пользователь присоединился к чату"), conn)

	// Читаем сообщения от клиента
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			break
		}

		log.Printf("Получено сообщение от %s: %s", r.RemoteAddr, string(p))

		// Рассылаем сообщение всем клиентам, кроме отправителя
		broadcast(p, conn)

		// Эхо-ответ (опционально)
		if messageType == websocket.TextMessage {
			conn.WriteMessage(websocket.TextMessage, []byte("Сообщение доставлено"))
		}
	}

	// Удаляем клиента при отключении
	mu.Lock()
	if uuid, ok := clientIDs[conn]; ok {
		delete(clients2, uuid)
		delete(clientIDs, conn)
	}
	mu.Unlock()

	log.Printf("Отключение: %s", r.RemoteAddr)
	// Рассылаем уведомление об отключении
	broadcast([]byte("Пользователь покинул чат"), nil)
}

// broadcast отправляет сообщение всем подключённым клиентам, кроме исключённого
func broadcast(message []byte, exclude *websocket.Conn) {
	mu.Lock()
	defer mu.Unlock()

	for uuid, client := range clients2 {
		if client == exclude {
			continue
		}
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
			client.Close()
			delete(clients2, uuid)
			delete(clientIDs, client)
		}
	}
}
