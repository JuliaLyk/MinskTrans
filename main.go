package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// User представляет данные пользователя
type User struct {
	TabNum       int
	FullName     string
	Organization string
	Schedule     []Schedule
}

// Schedule представляет расписание пользователя
type Schedule struct {
	Date  string
	Shift string
}

// RouteDetails представляет детали маршрута
type RouteDetails struct {
	TabNum    int
	Route     string
	Departure string
	Arrival   string
	Duration  string
	Breaks    []string
}

var (
	db   *sql.DB
	tmpl = template.Must(template.ParseGlob("templates/*.html"))
)

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./users.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTables()
	createTestData() // Создание тестовых данных

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/calendar", calendarHandler)
	http.HandleFunc("/details", detailsHandler)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createTables() {
	createUserTableSQL := `CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        tab_num INTEGER NOT NULL UNIQUE,
        full_name TEXT NOT NULL,
        organization TEXT NOT NULL
    );`

	_, err := db.Exec(createUserTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	createScheduleTableSQL := `CREATE TABLE IF NOT EXISTS schedule (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        tab_num INTEGER NOT NULL,
        date TEXT NOT NULL,
        shift TEXT NOT NULL,
        FOREIGN KEY(tab_num) REFERENCES users(tab_num)
    );`

	_, err = db.Exec(createScheduleTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	createRoutesTableSQL := `CREATE TABLE IF NOT EXISTS routes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        tab_num INTEGER NOT NULL,
        date TEXT NOT NULL,
        route TEXT NOT NULL,
        departure TEXT NOT NULL,
        arrival TEXT NOT NULL,
        duration TEXT NOT NULL,
        breaks TEXT NOT NULL,
        FOREIGN KEY(tab_num) REFERENCES users(tab_num)
    );`

	_, err = db.Exec(createRoutesTableSQL)
	if err != nil {
		log.Fatal(err)
	}
}

func createTestData() {
	testData := []RouteDetails{
		{TabNum: 12345678, Route: "Маршрут 1", Departure: "Место отправления 1", Arrival: "Место прибытия 1", Duration: "2 часа", Breaks: []string{"Перерыв 1", "Перерыв 2"}},
		{TabNum: 12345678, Route: "Маршрут 2", Departure: "Место отправления 2", Arrival: "Место прибытия 2", Duration: "3 часа", Breaks: []string{"Перерыв 3", "Перерыв 4"}},
		// Добавьте больше данных по мере необходимости
	}

	for _, data := range testData {
		breaksStr := strings.Join(data.Breaks, ", ")
		_, err := db.Exec("INSERT INTO routes (tab_num, date, route, departure, arrival, duration, breaks) VALUES (?, ?, ?, ?, ?, ?, ?)",
			data.TabNum, "2024-07-01", data.Route, data.Departure, data.Arrival, data.Duration, breaksStr)
		if err != nil {
			log.Fatalf("Error inserting test data: %v", err)
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		tabNumStr := r.FormValue("tabNum")
		tabNum := parseTabNum(tabNumStr)

		user, err := getUserByTabNum(tabNum)
		if err != nil {
			http.Error(w, "Пользователь не найден", http.StatusNotFound)
			return
		}

		user.Schedule, err = getUserSchedule(tabNum)
		if err != nil {
			http.Error(w, "Ошибка загрузки расписания", http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "calendar.html", user)
	} else {
		tmpl.ExecuteTemplate(w, "login.html", nil)
	}
}

func calendarHandler(w http.ResponseWriter, r *http.Request) {
	tabNumStr := r.URL.Query().Get("tabNum")
	tabNum := parseTabNum(tabNumStr)

	user, err := getUserByTabNum(tabNum)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	user.Schedule, err = getUserSchedule(tabNum)
	if err != nil {
		http.Error(w, "Ошибка загрузки расписания", http.StatusInternalServerError)
		return
	}

	tmpl.ExecuteTemplate(w, "calendar.html", user)
}

func detailsHandler(w http.ResponseWriter, r *http.Request) {
	tabNumStr := r.URL.Query().Get("tabNum")
	tabNum := parseTabNum(tabNumStr)
	date := r.URL.Query().Get("date")

	if tabNum == 0 || date == "" {
		log.Printf("Invalid request parameters: tabNum %d, date %s", tabNum, date)
		http.Error(w, "Invalid request parameters", http.StatusBadRequest)
		return
	}

	routeDetails, err := getRouteDetails(tabNum, date)
	if err != nil {
		log.Printf("Error getting route details: %v", err)
		http.Error(w, "Data not found", http.StatusNotFound)
		return
	}

	tmpl.ExecuteTemplate(w, "details.html", routeDetails)
}

func parseTabNum(tabNumStr string) int {
	tabNum, _ := strconv.Atoi(tabNumStr)
	return tabNum
}

func getUserByTabNum(tabNum int) (User, error) {
	var user User
	row := db.QueryRow("SELECT tab_num, full_name, organization FROM users WHERE tab_num = ?", tabNum)
	err := row.Scan(&user.TabNum, &user.FullName, &user.Organization)
	if err != nil {
		return user, err
	}
	return user, nil
}

func getUserSchedule(tabNum int) ([]Schedule, error) {
	var schedules []Schedule
	rows, err := db.Query("SELECT date, shift FROM schedule WHERE tab_num = ?", tabNum)
	if err != nil {
		return schedules, err
	}
	defer rows.Close()

	for rows.Next() {
		var schedule Schedule
		err := rows.Scan(&schedule.Date, &schedule.Shift)
		if err != nil {
			return schedules, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func getRouteDetails(tabNum int, date string) (RouteDetails, error) {
	var routeDetails RouteDetails
	query := `SELECT route, departure, arrival, duration, breaks 
              FROM routes 
              WHERE tab_num = ? AND date = ?`
	row := db.QueryRow(query, tabNum, date)
	var breaks string
	err := row.Scan(&routeDetails.Route, &routeDetails.Departure, &routeDetails.Arrival, &routeDetails.Duration, &breaks)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No route details found for tabNum %d and date %s", tabNum, date)
			return routeDetails, err
		}
		log.Printf("Error scanning row for tabNum %d and date %s: %v", tabNum, date, err)
		return routeDetails, err
	}
	routeDetails.TabNum = tabNum
	routeDetails.Breaks = strings.Split(breaks, ", ")
	log.Printf("Route details found for tabNum %d and date %s: %+v", tabNum, date, routeDetails)
	return routeDetails, nil
}
