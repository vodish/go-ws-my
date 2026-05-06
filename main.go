package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Разрешаем все origin для простоты
		},
	}

	// Глобальное хранилище клиентов
	clientStore = NewClientStore()

	// Глобальное подключение к БД
	my *My
)

func main() {
	// Загружаем переменные окружения из .env файла
	err := godotenv.Load()
	if err != nil {
		log.Println("Не удалось загрузить .env файл, используем переменные окружения системы")
	}

	// Инициализируем подключение к БД
	my, err = NewMyEnv()
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer my.Close()

	// Получаем порт из переменной окружения
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("Переменная окружения PORT не установлена")
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

	// Регистрируем нового клиента в хранилище
	clientUUID := clientStore.Add(conn)

	log.Printf("Новое подключение: %s, UUID: %s", r.RemoteAddr, clientUUID)

	// Отправляем приветственное сообщение
	conn.WriteMessage(websocket.TextMessage, []byte("Добро пожаловать в чат!"))

	// Рассылаем уведомление о новом пользователе всем клиентам
	clientStore.Broadcast([]byte("Пользователь присоединился к чату"), conn)

	// Читаем сообщения от клиента
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			break
		}

		log.Printf("Получено сообщение от %s: %s", r.RemoteAddr, string(p))

		// Рассылаем сообщение всем клиентам, кроме отправителя
		clientStore.Broadcast(p, conn)

		// Эхо-ответ (опционально)
		if messageType == websocket.TextMessage {
			conn.WriteMessage(websocket.TextMessage, []byte("Сообщение доставлено"))
		}
	}

	// Удаляем клиента при отключении
	clientStore.Remove(conn)

	log.Printf("Отключение: %s", r.RemoteAddr)
	// Рассылаем уведомление об отключении
	clientStore.Broadcast([]byte("Пользователь покинул чат"), nil)
}
