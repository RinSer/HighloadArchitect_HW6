package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/rinser/hw6/feed"
	"github.com/stretchr/testify/assert"
)

var testServer *echo.Echo
var testService *feed.Service

func TestMain(m *testing.M) {
	testServer = echo.New()
	var err error
	testService, err = feed.NewService(
		"test:test@tcp(127.0.0.1:3301)/social_network",
		"localhost:7000",
		"amqp://test:test@localhost:5672/")
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	} else {
		defer testService.Cancel()
		go testService.UpdateFeeds()
		scriptBytes, err := os.ReadFile("db.sql")
		if err != nil {
			log.Fatal(err)
			os.Exit(-1)
		}
		scripts := strings.Split(string(scriptBytes), "--")
		// run db schema creation script
		for _, script := range scripts {
			_, err = testService.Db().Exec(string(script))
			if err != nil {
				log.Fatal(err)
				os.Exit(-1)
			}
		}
		exitVal := m.Run()
		os.Exit(exitVal)
	}
}

func TestAddUser(t *testing.T) {
	// Setup
	userJSON := `{"login":"user0"}`
	req := httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := testServer.NewContext(req, rec)
	// Assertions
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
}

func TestAddFollower(t *testing.T) {
	// Setup
	userJSON := `{"login":"user0"}`
	req := httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId1, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	userJSON = `{"login":"user1"}`
	req = httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId2, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	followerJSON := fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId1, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	// Assertions
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
}

func TestRemoveFollower(t *testing.T) {
	// Setup
	userJSON := `{"login":"user0"}`
	req := httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId1, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	userJSON = `{"login":"user1"}`
	req = httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	// add follower
	userId2, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	followerJSON := fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId1, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
	// remove follower
	req = httptest.NewRequest(http.MethodPost, "/follower?remove=true",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	// Assertions
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
}

func TestAddPublication(t *testing.T) {
	// Setup
	userJSON := `{"login":"user0"}`
	req := httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId1, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	userJSON = `{"login":"user1"}`
	req = httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId2, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	followerJSON := fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId1, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
	pubText := uuid.New().String()
	publicationJSON := fmt.Sprintf(`{"author":%d,"text":"%s"}`,
		userId1, pubText)
	req = httptest.NewRequest(http.MethodPost, "/publication",
		strings.NewReader(publicationJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	// Assertions
	if assert.NoError(t, testService.AddPublication(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		testPub := new(feed.Publication)
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), testPub))
		assert.Equal(t, userId1, testPub.Author)
		assert.Equal(t, pubText, testPub.Text)
		assert.Greater(t, testPub.Id, int64(0))
		assert.NotEmpty(t, testPub.At)
	}
}

func TestGetFeed(t *testing.T) {
	// Setup
	userJSON := `{"login":"user0"}`
	req := httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId1, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	userJSON = `{"login":"user1"}`
	req = httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId2, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	followerJSON := fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId1, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
	pubText := uuid.New().String()
	publicationJSON := fmt.Sprintf(`{"author":%d,"text":"%s"}`,
		userId1, pubText)
	req = httptest.NewRequest(http.MethodPost, "/publication",
		strings.NewReader(publicationJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddPublication(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		testPub := new(feed.Publication)
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), testPub))
		assert.Equal(t, userId1, testPub.Author)
		assert.Equal(t, pubText, testPub.Text)
		assert.Greater(t, testPub.Id, int64(0))
		assert.NotEmpty(t, testPub.At)
	}
	time.Sleep(2 * time.Second)
	req = httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/feed/%d", userId2), nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	c.SetPath("/feed/:userId")
	c.SetParamNames("userId")
	c.SetParamValues(strconv.FormatInt(userId2, 10))
	// Assertions
	if assert.NoError(t, testService.GetFeed(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		pubs := make([]feed.Publication, 0)
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pubs))
		assert.Len(t, pubs, 1)
		if len(pubs) > 1 {
			assert.Equal(t, userId1, pubs[0].Author)
		}
	}
}

func TestInvalidateFeed(t *testing.T) {
	// Setup
	userJSON := `{"login":"user0"}`
	req := httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId1, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	userJSON = `{"login":"user1"}`
	req = httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId2, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	userJSON = `{"login":"user2"}`
	req = httptest.NewRequest(http.MethodPost, "/user",
		strings.NewReader(userJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddUser(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
	}
	userId3, _ := strconv.ParseInt(strings.Trim(rec.Body.String(), "\n"), 10, 64)
	followerJSON := fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId1, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
	followerJSON = fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId3, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
	for i := 0; i < 3; i++ {
		userId := userId1
		if i > 1 {
			userId = userId3
		}
		pubText := uuid.New().String()
		publicationJSON := fmt.Sprintf(`{"author":%d,"text":"%s"}`,
			userId, pubText)
		req = httptest.NewRequest(http.MethodPost, "/publication",
			strings.NewReader(publicationJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec = httptest.NewRecorder()
		c = testServer.NewContext(req, rec)
		if assert.NoError(t, testService.AddPublication(c)) {
			assert.Equal(t, http.StatusCreated, rec.Code)
			testPub := new(feed.Publication)
			assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), testPub))
			assert.Equal(t, userId, testPub.Author)
			assert.Equal(t, pubText, testPub.Text)
			assert.Greater(t, testPub.Id, int64(0))
			assert.NotEmpty(t, testPub.At)
		}
		time.Sleep(2 * time.Second)
		req = httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/feed/%d", userId2), nil)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec = httptest.NewRecorder()
		c = testServer.NewContext(req, rec)
		c.SetPath("/feed/:userId")
		c.SetParamNames("userId")
		c.SetParamValues(strconv.FormatInt(userId2, 10))
		if assert.NoError(t, testService.GetFeed(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			pubs := make([]feed.Publication, 0)
			assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pubs))
			assert.Len(t, pubs, i+1)
			if len(pubs) > i {
				assert.Equal(t, userId, pubs[0].Author)
			}
		}
	}
	followerJSON = fmt.Sprintf(`{"userId":%d,"followerId":%d}`,
		userId1, userId2)
	req = httptest.NewRequest(http.MethodPost, "/follower?remove=true",
		strings.NewReader(followerJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	if assert.NoError(t, testService.AddFollower(c)) {
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "true", strings.Trim(rec.Body.String(), "\n"))
	}
	time.Sleep(2 * time.Second)
	req = httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/feed/%d", userId2), nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = testServer.NewContext(req, rec)
	c.SetPath("/feed/:userId")
	c.SetParamNames("userId")
	c.SetParamValues(strconv.FormatInt(userId2, 10))
	// Assertions
	if assert.NoError(t, testService.GetFeed(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		pubs := make([]feed.Publication, 0)
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pubs))
		assert.Len(t, pubs, 1)
		if len(pubs) > 1 {
			assert.Equal(t, userId3, pubs[0].Author)
		}
	}
}
