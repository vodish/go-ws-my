package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	_ "github.com/go-sql-driver/mysql"
)

type My struct {
	db *sql.DB
}

type MV = map[string]any

// Возвращает параметры подключения к MySQL из переменных окружения.
// Используются переменные: DB_USER, DB_PASS, DB_HOST, DB_PORT, DB_NAME.
// Если переменная не установлена, возвращается пустая строка.
func EnvDB() (username, password, host, port, database string) {
	username = os.Getenv("DB_USER")
	password = os.Getenv("DB_PASS")
	host = os.Getenv("DB_HOST")
	port = os.Getenv("DB_PORT")
	database = os.Getenv("DB_NAME")
	return
}

// Создаёт новое подключение к MySQL и возвращает экземпляр My.
// Параметры: username, password, host, port, database.
// Если все пять параметров пусты, используются значения из переменных окружения (EnvDB).
func NewMy(username, password, host, port, database string) (*My, error) {
	// Если все параметры пустые, используем переменные окружения
	if username == "" && password == "" && host == "" && port == "" && database == "" {
		username, password, host, port, database = EnvDB()
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &My{db: db}, nil
}

// Создаёт подключение к MySQL, используя параметры из переменных окружения.
// Вызывает EnvDB для получения параметров, затем NewMy.
func NewMyEnv() (*My, error) {
	username, password, host, port, database := EnvDB()
	return NewMy(username, password, host, port, database)
}

// Создаёт подключение по готовой DSN строке.
func NewMyFromDSN(dsn string) (*My, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &My{db: db}, nil
}

// Закрывает соединение с базой данных.
func (my *My) Close() error {
	return my.db.Close()
}

// Exe выполняет запрос (INSERT, UPDATE, DELETE) и возвращает true при успехе.
func (my *My) Exe(q string, args ...MV) bool {
	// Получаем безопасный запрос, параметры и запрос с подставленными значениями
	sql, params, dbg := my.prepareArgs(q, args)

	// Выполняем безопасный запрос с параметрами
	_, err := my.db.Exec(sql, params...)
	if err != nil {
		logSQL(err, dbg)
		return false
	}
	return true
}

// One выполняет запрос SELECT и возвращает первую строку в виде map[string]string.
// Если строк нет, возвращает пустую map.
func (my *My) One(q string, args ...MV) map[string]string {
	sql, params, dbg := my.prepareArgs(q, args)

	rows, err := my.db.Query(sql, params...)
	if err != nil {
		logSQL(err, dbg)
		return map[string]string{}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		logSQL(err, dbg)
		return map[string]string{}
	}

	if !rows.Next() {
		return map[string]string{}
	}

	// Создаём срезы для значений
	values := make([]any, len(cols))
	valuePtrs := make([]any, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		logSQL(err, dbg)
		return map[string]string{}
	}

	result := make(map[string]string, len(cols))
	for i, col := range cols {
		if values[i] == nil {
			result[col] = ""
		} else {
			result[col] = fmt.Sprintf("%v", values[i])
		}
	}
	return result
}

// Выполняет запрос SELECT и возвращает все строки в виде []map[string]string.
// Если строк нет, возвращает пустой срез.
func (my *My) Sel(q string, args ...MV) []map[string]string {
	// Получаем безопасный запрос, параметры и запрос с подставленными значениями
	sql, params, dbg := my.prepareArgs(q, args)

	rows, err := my.db.Query(sql, params...)
	if err != nil {
		logSQL(err, dbg)
		return []map[string]string{}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		logSQL(err, dbg)
		return []map[string]string{}
	}

	var results []map[string]string
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			logSQL(err, dbg)
			continue
		}

		row := make(map[string]string, len(cols))
		for i, col := range cols {
			if values[i] == nil {
				row[col] = ""
			} else {
				row[col] = fmt.Sprintf("%v", values[i])
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		logSQL(err, dbg)
	}
	return results
}

// Возвращает ID последней вставленной записи.
func (my *My) LastID() int64 {
	var id int64
	query := "SELECT LAST_INSERT_ID()"
	err := my.db.QueryRow(query).Scan(&id)
	if err != nil {
		logSQL(err, query)
		return 0
	}
	return id
}

// Заменяет :name на ?
// и собирает значения в срез
// и версию запроса для отладки
func (my *My) prepareArgs(query string, args []map[string]any) (string, []any, string) {
	if len(args) == 0 || len(args[0]) == 0 {
		return query, []any{}, query
	}

	// безопасный запрос с ? для выполнения
	// срез обработанных значений для передачи в Exec
	// финальный запрос с подставленными значениями для логирования
	sql := query
	var params []any
	dbg := query

	// Сортируем ключи карты для предсказуемого порядка обработки
	keys := make([]string, 0, len(args[0]))
	for key := range args[0] {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Обрабатываем каждый ключ и заменяем плейсхолдеры
	for _, key := range keys {
		placeholder := ":" + key

		// Проверяем, есть ли плейсхолдер в запросе
		if strings.Contains(sql, placeholder) {
			// Получаем исходное значение из карты
			prm_v := args[0][key]

			// Заменяем плейсхолдер на ? в safeQuery
			// Добавляем исходное значение в срез params для безопасного выполнения
			sql = strings.ReplaceAll(sql, placeholder, "?")
			params = append(params, prm_v)

			// Обрабатываем значение через метод V для финального запроса
			// Заменяем плейсхолдер на обработанное значение в finalQuery
			esc_v := my.escapeValue(prm_v)
			dbg = strings.ReplaceAll(dbg, placeholder, esc_v)
		}
	}

	return sql, params, dbg
}

// Экранирует значение для подстановки в SQL запрос.
// Для числовых значений возвращает строку без кавычек, для остальных - с кавычками и экранированием.
func (my *My) escapeValue(x any) string {
	s := fmt.Sprintf("%v", x)
	if my.isDecimal(s) {
		return s
	}
	// Экранирование специальных символов MySQL
	// Вместо самописного экранирования используем параметризованные запросы, но для совместимости с API
	// реализуем простое экранирование через Replace.
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}

// Проверяет, является ли строка десятичным числом (целым или с плавающей точкой).
// Допускается знак + или - в начале и одна точка.
func (my *My) isDecimal(s string) bool {
	if len(s) == 0 {
		return false
	}
	beg := 0
	if s[0] == '-' || s[0] == '+' {
		if len(s) == 1 {
			return false
		}
		beg = 1
	}
	hasDigit := false
	hasDot := false
	for _, ch := range s[beg:] {
		if ch == '.' {
			if hasDot {
				return false
			}
			hasDot = true
		} else if unicode.IsDigit(ch) {
			hasDigit = true
		} else {
			return false
		}
	}
	return hasDigit
}

// Логирует ошибку SQL запроса с цветным выводом (использует ANSI коды).
func logSQL(err error, q string) {
	const red = "\033[31m"
	const reset = "\033[0m"
	log.Printf("%s[sql]%s\n%s", red, reset, err)
	log.Printf("%s[dbg]%s\n%s", red, reset, q)
}
