// main.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
)

// Models
type News struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	TopicID   int       `json:"topic_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Topic struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

// Database connection
var db *sql.DB

func main() {
	// Initialize database connection
	initDB()
	defer db.Close()

	// Create tables if they don't exist
	createTables()

	// Initialize Echo
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	// News endpoints
	e.GET("/api/news", getAllNews)
	e.GET("/api/news/:id", getNewsById)
	e.POST("/api/news", createNews)
	e.PUT("/api/news/:id", updateNews)
	e.DELETE("/api/news/:id", deleteNews)
	e.GET("/api/news/topic/:topic_id", getNewsByTopic)

	// Topic endpoints
	e.GET("/api/topics", getAllTopics)
	e.GET("/api/topics/:id", getTopicById)
	e.POST("/api/topics", createTopic)
	e.PUT("/api/topics/:id", updateTopic)
	e.DELETE("/api/topics/:id", deleteTopic)

	// Health check
	e.GET("/health", healthCheck)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}

func initDB() {
	var err error
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/newsdb?sslmode=disable"
	}

	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	log.Println("Database connection established")
}

func createTables() {
	// Create topics table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS topics (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Error creating topics table: %v", err)
	}

	// Create news table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS news (
			id SERIAL PRIMARY KEY,
			title VARCHAR(200) NOT NULL,
			content TEXT NOT NULL,
			topic_id INTEGER REFERENCES topics(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Error creating news table: %v", err)
	}

	log.Println("Database tables created successfully")
}

// Health check handler
func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// News handlers
func getAllNews(c echo.Context) error {
	rows, err := db.Query(`
		SELECT n.id, n.title, n.content, n.topic_id, n.created_at, n.updated_at
		FROM news n
		ORDER BY n.created_at DESC
	`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch news"})
	}
	defer rows.Close()

	var newsList []News
	for rows.Next() {
		var news News
		err := rows.Scan(&news.ID, &news.Title, &news.Content, &news.TopicID, &news.CreatedAt, &news.UpdatedAt)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error scanning news row"})
		}
		newsList = append(newsList, news)
	}

	return c.JSON(http.StatusOK, newsList)
}

func getNewsById(c echo.Context) error {
	id := c.Param("id")
	var news News

	err := db.QueryRow(`
		SELECT id, title, content, topic_id, created_at, updated_at
		FROM news
		WHERE id = $1
	`, id).Scan(&news.ID, &news.Title, &news.Content, &news.TopicID, &news.CreatedAt, &news.UpdatedAt)

	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, ErrorResponse{Message: "News not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch news"})
	}

	return c.JSON(http.StatusOK, news)
}

func createNews(c echo.Context) error {
	news := new(News)
	if err := c.Bind(news); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Invalid request payload"})
	}

	// Validate required fields
	if news.Title == "" || news.Content == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Title and content are required"})
	}

	// Verify topic exists
	var topicExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM topics WHERE id = $1)", news.TopicID).Scan(&topicExists)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error verifying topic"})
	}
	if !topicExists {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Topic does not exist"})
	}

	// Insert news
	err = db.QueryRow(`
		INSERT INTO news (title, content, topic_id, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, news.Title, news.Content, news.TopicID).Scan(&news.ID, &news.CreatedAt, &news.UpdatedAt)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to create news"})
	}

	return c.JSON(http.StatusCreated, news)
}

func updateNews(c echo.Context) error {
	id := c.Param("id")
	news := new(News)
	if err := c.Bind(news); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Invalid request payload"})
	}

	// Validate required fields
	if news.Title == "" || news.Content == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Title and content are required"})
	}

	// Verify topic exists
	var topicExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM topics WHERE id = $1)", news.TopicID).Scan(&topicExists)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error verifying topic"})
	}
	if !topicExists {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Topic does not exist"})
	}

	// Update news
	res, err := db.Exec(`
		UPDATE news
		SET title = $1, content = $2, topic_id = $3, updated_at = NOW()
		WHERE id = $4
	`, news.Title, news.Content, news.TopicID, id)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to update news"})
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error checking update result"})
	}
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, ErrorResponse{Message: "News not found"})
	}

	// Get updated news
	err = db.QueryRow(`
		SELECT id, title, content, topic_id, created_at, updated_at
		FROM news
		WHERE id = $1
	`, id).Scan(&news.ID, &news.Title, &news.Content, &news.TopicID, &news.CreatedAt, &news.UpdatedAt)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch updated news"})
	}

	return c.JSON(http.StatusOK, news)
}

func deleteNews(c echo.Context) error {
	id := c.Param("id")

	res, err := db.Exec("DELETE FROM news WHERE id = $1", id)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to delete news"})
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error checking delete result"})
	}
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, ErrorResponse{Message: "News not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "News deleted successfully"})
}

func getNewsByTopic(c echo.Context) error {
	topicID := c.Param("topic_id")

	rows, err := db.Query(`
		SELECT n.id, n.title, n.content, n.topic_id, n.created_at, n.updated_at
		FROM news n
		WHERE n.topic_id = $1
		ORDER BY n.created_at DESC
	`, topicID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch news by topic"})
	}
	defer rows.Close()

	var newsList []News
	for rows.Next() {
		var news News
		err := rows.Scan(&news.ID, &news.Title, &news.Content, &news.TopicID, &news.CreatedAt, &news.UpdatedAt)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error scanning news row"})
		}
		newsList = append(newsList, news)
	}

	return c.JSON(http.StatusOK, newsList)
}

// Topic handlers
func getAllTopics(c echo.Context) error {
	rows, err := db.Query(`
		SELECT id, name, description, created_at, updated_at
		FROM topics
		ORDER BY name
	`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch topics"})
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var topic Topic
		err := rows.Scan(&topic.ID, &topic.Name, &topic.Description, &topic.CreatedAt, &topic.UpdatedAt)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error scanning topic row"})
		}
		topics = append(topics, topic)
	}

	return c.JSON(http.StatusOK, topics)
}

func getTopicById(c echo.Context) error {
	id := c.Param("id")
	var topic Topic

	err := db.QueryRow(`
		SELECT id, name, description, created_at, updated_at
		FROM topics
		WHERE id = $1
	`, id).Scan(&topic.ID, &topic.Name, &topic.Description, &topic.CreatedAt, &topic.UpdatedAt)

	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, ErrorResponse{Message: "Topic not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch topic"})
	}

	return c.JSON(http.StatusOK, topic)
}

func createTopic(c echo.Context) error {
	topic := new(Topic)
	if err := c.Bind(topic); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Invalid request payload"})
	}

	// Validate required fields
	if topic.Name == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Topic name is required"})
	}

	// Insert topic
	err := db.QueryRow(`
		INSERT INTO topics (name, description, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, topic.Name, topic.Description).Scan(&topic.ID, &topic.CreatedAt, &topic.UpdatedAt)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to create topic"})
	}

	return c.JSON(http.StatusCreated, topic)
}

func updateTopic(c echo.Context) error {
	id := c.Param("id")
	topic := new(Topic)
	if err := c.Bind(topic); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Invalid request payload"})
	}

	// Validate required fields
	if topic.Name == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Topic name is required"})
	}

	// Update topic
	res, err := db.Exec(`
		UPDATE topics
		SET name = $1, description = $2, updated_at = NOW()
		WHERE id = $3
	`, topic.Name, topic.Description, id)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to update topic"})
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error checking update result"})
	}
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, ErrorResponse{Message: "Topic not found"})
	}

	// Get updated topic
	err = db.QueryRow(`
		SELECT id, name, description, created_at, updated_at
		FROM topics
		WHERE id = $1
	`, id).Scan(&topic.ID, &topic.Name, &topic.Description, &topic.CreatedAt, &topic.UpdatedAt)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to fetch updated topic"})
	}

	return c.JSON(http.StatusOK, topic)
}

func deleteTopic(c echo.Context) error {
	id := c.Param("id")

	// Check if there are news articles with this topic first
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM news WHERE topic_id = $1", id).Scan(&count)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to check news references"})
	}
	if count > 0 {
		return c.JSON(http.StatusConflict, ErrorResponse{Message: "Cannot delete topic with associated news articles"})
	}

	res, err := db.Exec("DELETE FROM topics WHERE id = $1", id)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Failed to delete topic"})
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "Error checking delete result"})
	}
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, ErrorResponse{Message: "Topic not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Topic deleted successfully"})
}