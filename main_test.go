// main_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Setup test database
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/newsdb_test?sslmode=disable")
	
	// Initialize DB and create tables
	initDB()
	createTables()
	
	// Clean up tables before tests
	db.Exec("DELETE FROM news")
	db.Exec("DELETE FROM topics")
	
	// Run tests
	exitCode := m.Run()
	
	// Clean up after tests
	db.Exec("DELETE FROM news")
	db.Exec("DELETE FROM topics")
	db.Close()
	
	os.Exit(exitCode)
}

func setupEcho() *echo.Echo {
	e := echo.New()
	return e
}

// Test health check endpoint
func TestHealthCheck(t *testing.T) {
	e := setupEcho()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	if assert.NoError(t, healthCheck(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "ok", response["status"])
		assert.NotEmpty(t, response["time"])
	}
}

// Test topic creation and retrieval
func TestTopicLifecycle(t *testing.T) {
	e := setupEcho()
	
	// 1. Create a topic
	topicPayload := `{"name":"Technology","description":"News about technology"}`
	req := httptest.NewRequest(http.MethodPost, "/api/topics", bytes.NewBufferString(topicPayload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	assert.NoError(t, createTopic(c))
	assert.Equal(t, http.StatusCreated, rec.Code)
	
	var createdTopic Topic
	err := json.Unmarshal(rec.Body.Bytes(), &createdTopic)
	assert.NoError(t, err)
	assert.Equal(t, "Technology", createdTopic.Name)
	assert.Equal(t, "News about technology", createdTopic.Description)
	assert.NotZero(t, createdTopic.ID)
	
	// 2. Get topic by ID
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/topics/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(createdTopic.ID)))
	
	assert.NoError(t, getTopicById(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	var retrievedTopic Topic
	err = json.Unmarshal(rec.Body.Bytes(), &retrievedTopic)
	assert.NoError(t, err)
	assert.Equal(t, createdTopic.ID, retrievedTopic.ID)
	assert.Equal(t, "Technology", retrievedTopic.Name)
	
	// 3. Update topic
	updatePayload := `{"name":"Updated Technology","description":"Updated description"}`
	req = httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString(updatePayload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/topics/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(createdTopic.ID)))
	
	assert.NoError(t, updateTopic(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	var updatedTopic Topic
	err = json.Unmarshal(rec.Body.Bytes(), &updatedTopic)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Technology", updatedTopic.Name)
	
	// 4. Get all topics
	req = httptest.NewRequest(http.MethodGet, "/api/topics", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	
	assert.NoError(t, getAllTopics(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	var topics []Topic
	err = json.Unmarshal(rec.Body.Bytes(), &topics)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(topics), 1)
	
	// 5. Delete topic
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/topics/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(createdTopic.ID)))
	
	assert.NoError(t, deleteTopic(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	// 6. Verify topic is deleted
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/topics/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(createdTopic.ID)))
	
	err = getTopicById(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Test news lifecycle with topic dependency
func TestNewsLifecycle(t *testing.T) {
	e := setupEcho()
	
	// 1. Create a topic first
	topicPayload := `{"name":"Science","description":"Scientific news"}`
	req := httptest.NewRequest(http.MethodPost, "/api/topics", bytes.NewBufferString(topicPayload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	assert.NoError(t, createTopic(c))
	
	var topic Topic
	err := json.Unmarshal(rec.Body.Bytes(), &topic)
	assert.NoError(t, err)
	
	// 2. Create a news article
	newsPayload := `{
		"title": "New Scientific Discovery",
		"content": "Scientists have made a breakthrough discovery.",
		"topic_id": ` + string(rune(topic.ID)) + `
	}`
	
	req = httptest.NewRequest(http.MethodPost, "/api/news", bytes.NewBufferString(newsPayload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	
	assert.NoError(t, createNews(c))
	assert.Equal(t, http.StatusCreated, rec.Code)
	
	var news News
	err = json.Unmarshal(rec.Body.Bytes(), &news)
	assert.NoError(t, err)
	assert.Equal(t, "New Scientific Discovery", news.Title)
	assert.Equal(t, topic.ID, news.TopicID)
	
	// 3. Get news by ID
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/news/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(news.ID)))
	
	assert.NoError(t, getNewsById(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	// 4. Get news by topic
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/news/topic/:topic_id")
	c.SetParamNames("topic_id")
	c.SetParamValues(string(rune(topic.ID)))
	
	assert.NoError(t, getNewsByTopic(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	var newsList []News
	err = json.Unmarshal(rec.Body.Bytes(), &newsList)
	assert.NoError(t, err)
	assert.Len(t, newsList, 1)
	
	// 5. Attempt to delete topic with associated news (should fail)
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/topics/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(topic.ID)))
	
	err = deleteTopic(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	
	// 6. Delete news first
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/news/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(news.ID)))
	
	assert.NoError(t, deleteNews(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	
	// 7. Now delete the topic (should succeed)
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/topics/:id")
	c.SetParamNames("id")
	c.SetParamValues(string(rune(topic.ID)))
	
	assert.NoError(t, deleteTopic(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}