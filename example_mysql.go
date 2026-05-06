//go:build example
// +build example

package main

import (
	"fmt"
	"log"
	"os"
)

func ExampleWithCredentials() {
	// Пример подключения с явными параметрами
	my, err := NewMy("root", "password", "localhost", "3306", "testdb")
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	defer my.Close()

	fmt.Println("Подключение с явными параметрами успешно")
	// ... дальнейшие операции
}

func ExampleWithEnv() {
	// Установим переменные окружения для демонстрации (в реальном приложении они задаются в .env)
	os.Setenv("DB_USER", "root")
	os.Setenv("DB_PASS", "password")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "3306")
	os.Setenv("DB_NAME", "testdb")

	// Способ 1: Использование NewMyEnv (автоматически берёт параметры из окружения)
	my, err := NewMyEnv()
	if err != nil {
		log.Fatal("Ошибка подключения к БД через NewMyEnv:", err)
	}
	defer my.Close()

	fmt.Println("Подключение через NewMyEnv успешно")

	// Способ 2: Использование NewMy с пустыми параметрами (автоматически подставит окружение)
	my2, err := NewMy("", "", "", "", "")
	if err != nil {
		log.Fatal("Ошибка подключения к БД через NewMy с пустыми параметрами:", err)
	}
	defer my2.Close()

	fmt.Println("Подключение через NewMy с пустыми параметрами успешно")

	// Получение параметров через EnvDB
	user, _, host, port, db := EnvDB()
	fmt.Printf("Параметры из окружения: user=%s, host=%s, port=%s, db=%s\n", user, host, port, db)

	// Демонстрация работы с БД
	createTable := `
	CREATE TABLE IF NOT EXISTS example (
		id INT AUTO_INCREMENT PRIMARY KEY,
		message VARCHAR(255)
	)`
	if my.Exe(createTable) {
		fmt.Println("Таблица создана или уже существует")
	}

	// Вставка данных
	insertQuery := fmt.Sprintf("INSERT INTO example (message) VALUES (%s)", my.V("Привет, мир!"))
	if my.Exe(insertQuery) {
		fmt.Println("Данные вставлены, ID:", my.LastID())
	}

	// Выборка
	row := my.One("SELECT * FROM example LIMIT 1")
	fmt.Println("Первая запись:", row)

	// Экранирование
	fmt.Println("Экранирование строки:", my.V("O'Reilly"))
	fmt.Println("Экранирование числа:", my.V(42))
}

// Примеры использования пакета.
// Для запуска примеров соберите с тегом example:
//   go run -tags=example .  (но учтите конфликт с main.go)
// Лучше скопируйте код в отдельную программу.
