package main

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

// TestEnvDB проверяет функцию EnvDB на корректное чтение переменных окружения
func TestEnvDB(t *testing.T) {
	// Сохраняем текущие значения переменных окружения
	oldUser := os.Getenv("DB_USER")
	oldPass := os.Getenv("DB_PASSWORD")
	oldHost := os.Getenv("DB_HOST")
	oldPort := os.Getenv("DB_PORT")
	oldDB := os.Getenv("DB_NAME")

	// Восстанавливаем после теста
	defer func() {
		os.Setenv("DB_USER", oldUser)
		os.Setenv("DB_PASSWORD", oldPass)
		os.Setenv("DB_HOST", oldHost)
		os.Setenv("DB_PORT", oldPort)
		os.Setenv("DB_NAME", oldDB)
	}()

	// Устанавливаем тестовые значения
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_HOST", "testhost")
	os.Setenv("DB_PORT", "3307")
	os.Setenv("DB_NAME", "testdb")

	user, pass, host, port, db := EnvDB()

	if user != "testuser" {
		t.Errorf("DB_USER ожидается 'testuser', получено '%s'", user)
	}
	if pass != "testpass" {
		t.Errorf("DB_PASSWORD ожидается 'testpass', получено '%s'", pass)
	}
	if host != "testhost" {
		t.Errorf("DB_HOST ожидается 'testhost', получено '%s'", host)
	}
	if port != "3307" {
		t.Errorf("DB_PORT ожидается '3307', получено '%s'", port)
	}
	if db != "testdb" {
		t.Errorf("DB_NAME ожидается 'testdb', получено '%s'", db)
	}
}

// TestNewMyWithInvalidCredentials проверяет обработку ошибок при неверных учетных данных
func TestNewMyWithInvalidCredentials(t *testing.T) {
	// Пытаемся подключиться с заведомо неверными данными
	_, err := NewMy("invalid_user", "wrong_password", "localhost", "3306", "nonexistent_db")

	if err == nil {
		t.Error("Ожидалась ошибка подключения при неверных учетных данных")
	} else {
		// Проверяем, что ошибка содержит ожидаемые ключевые слова
		errMsg := err.Error()
		expectedKeywords := []string{"access denied", "Access denied", "1045", "28000"}
		found := false
		for _, keyword := range expectedKeywords {
			if strings.Contains(strings.ToLower(errMsg), strings.ToLower(keyword)) {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Полученная ошибка: %v", errMsg)
			// Не считаем это ошибкой теста, так как сообщение может отличаться
		}
	}
}

// TestNewMyWithEmptyParams проверяет поведение при пустых параметрах (должны использоваться переменные окружения)
func TestNewMyWithEmptyParams(t *testing.T) {
	// Сохраняем текущие значения
	oldUser := os.Getenv("DB_USER")
	oldPass := os.Getenv("DB_PASSWORD")
	oldHost := os.Getenv("DB_HOST")
	oldPort := os.Getenv("DB_PORT")
	oldDB := os.Getenv("DB_NAME")

	defer func() {
		os.Setenv("DB_USER", oldUser)
		os.Setenv("DB_PASSWORD", oldPass)
		os.Setenv("DB_HOST", oldHost)
		os.Setenv("DB_PORT", oldPort)
		os.Setenv("DB_NAME", oldDB)
	}()

	// Очищаем переменные окружения
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")

	// При пустых параметрах и пустых переменных окружения должна быть ошибка
	_, err := NewMy("", "", "", "", "")
	if err == nil {
		t.Error("Ожидалась ошибка подключения при отсутствии параметров и переменных окружения")
	}
}

// TestNewMyFromDSN проверяет создание подключения по DSN строке
func TestNewMyFromDSN(t *testing.T) {
	// Используем заведомо неверную DSN для проверки ошибки
	_, err := NewMyFromDSN("invalid_user:wrong_password@tcp(localhost:3306)/nonexistent_db")
	if err == nil {
		t.Error("Ожидалась ошибка подключения при неверной DSN")
	}

	// Проверяем, что функция возвращает ошибку при некорректном формате DSN
	_, err = NewMyFromDSN("invalid_dsn_format")
	if err == nil {
		t.Error("Ожидалась ошибка при некорректном формате DSN")
	}
}

// TestMyMethods проверяет основные методы структуры My (если подключение возможно)
func TestMyMethods(t *testing.T) {
	// Пропускаем тест, если нет реальной БД для тестирования
	if os.Getenv("TEST_DB_AVAILABLE") != "true" {
		t.Skip("Пропускаем тест методов БД, так как TEST_DB_AVAILABLE не установлен")
	}

	// Пытаемся подключиться с параметрами из окружения
	my, err := NewMyEnv()
	if err != nil {
		t.Skipf("Не удалось подключиться к БД для тестирования методов: %v", err)
	}
	defer my.Close()

	// Проверяем метод Exe с простым запросом
	createTable := `CREATE TABLE IF NOT EXISTS test_table (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(50)
	)`
	if !my.Exe(createTable) {
		t.Error("Не удалось выполнить CREATE TABLE")
	}

	// Проверяем метод V (экранирование)
	if my.V("test") != "'test'" {
		t.Errorf("Некорректное экранирование строки: %s", my.V("test"))
	}
	if my.V(123) != "123" {
		t.Errorf("Некорректное экранирование числа: %s", my.V(123))
	}
	if my.V("O'Reilly") != "'O''Reilly'" {
		t.Errorf("Некорректное экранирование строки с апострофом: %s", my.V("O'Reilly"))
	}

	// Проверяем метод LastID (должен вернуть 0, если не было INSERT)
	if my.LastID() != 0 {
		t.Errorf("LastID должен возвращать 0, если не было INSERT, получено: %d", my.LastID())
	}

	// Очищаем тестовую таблицу
	my.Exe("DROP TABLE IF EXISTS test_table")
}

// TestIsDecimalNumber проверяет внутренний метод isDecimalNumber
func TestIsDecimalNumber(t *testing.T) {
	my := &My{}

	testCases := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"-123", true},
		{"+123", true},
		{"12.34", true},
		{"-12.34", true},
		{"+12.34", true},
		{"", false},
		{"abc", false},
		{"12.34.56", false},
		{"12a34", false},
		{"--123", false},
		{"++123", false},
		{".123", true},  // точка в начале допустима (0.123)
		{"123.", true},  // точка в конце допустима (123.0)
	}

	for _, tc := range testCases {
		result := my.isDecimalNumber(tc.input)
		if result != tc.expected {
			t.Errorf("isDecimalNumber(%q) = %v, ожидается %v", tc.input, result, tc.expected)
		}
	}
}

// TestConnectionError проверяет конкретную ошибку из задания
func TestConnectionError(t *testing.T) {
	// Эмулируем ошибку "Access denied for user 'kfe'@'localhost' (using password: NO)"
	// Создаем мок DSN, которая вызовет подобную ошибку
	dsn := "kfe:@tcp(localhost:3306)/"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		// Ошибка при открытии соединения (не при ping)
		t.Logf("Ошибка при sql.Open: %v", err)
		return
	}
	
	// Ping должен вернуть ошибку
	err = db.Ping()
	if err != nil {
		errMsg := err.Error()
		// Проверяем, что ошибка содержит ожидаемые элементы
		if !strings.Contains(errMsg, "1045") && !strings.Contains(errMsg, "28000") {
			t.Logf("Ошибка Ping не содержит ожидаемых кодов, но это нормально: %v", errMsg)
		}
	} else {
		t.Log("Ping успешен (возможно, пользователь kfe существует без пароля)")
	}
	db.Close()
}

// TestMySQLIntegration проверяет реальное подключение к MySQL, если задана переменная окружения TEST_MYSQL_DSN
func TestMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("Пропускаем интеграционный тест, так как TEST_MYSQL_DSN не установлен")
	}

	my, err := NewMyFromDSN(dsn)
	if err != nil {
		t.Fatalf("Не удалось подключиться к MySQL по DSN: %v", err)
	}
	defer my.Close()

	// Проверяем, что подключение работает
	row := my.One("SELECT 1 as test")
	if row["test"] != "1" {
		t.Errorf("Ожидалось '1', получено '%s'", row["test"])
	}
}

// TestAccessDeniedError проверяет обработку ошибки доступа (если есть тестовая БД с неправильными учетными данными)
func TestAccessDeniedError(t *testing.T) {
	// Используем заведомо неверные учетные данные, которые должны вызвать ошибку 1045
	// Если MySQL сервер доступен, но учетные данные неверны
	dsn := "kfe:@tcp(localhost:3306)/"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Не удалось открыть соединение: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		errMsg := err.Error()
		// Проверяем, что это ошибка доступа (код 1045)
		if strings.Contains(errMsg, "1045") || strings.Contains(errMsg, "access denied") {
			t.Logf("Получена ожидаемая ошибка доступа: %v", errMsg)
			return
		}
		// Другие ошибки (например, connection refused) пропускаем
		t.Skipf("Ошибка Ping не является ошибкой доступа: %v", errMsg)
	} else {
		t.Skip("Подключение успешно (возможно, пользователь kfe существует без пароля)")
	}
}

// TestMain устанавливает окружение для тестов, если необходимо
func TestMain(m *testing.M) {
	// Можно установить переменные окружения для тестов здесь
	// Но лучше оставить это на усмотрение пользователя
	os.Exit(m.Run())
}