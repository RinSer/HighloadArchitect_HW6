package main

import (
	"github.com/labstack/echo"
	"github.com/rinser/hw6/feed"
)

func main() {
	// init echo server
	e := echo.New()
	// create feeder service
	s, err := feed.NewService(
		"test:test@tcp(127.0.0.1:3301)/social_network",
		"localhost:7000",
		"amqp://test:test@localhost:5672/")
	if err != nil {
		e.Logger.Fatal(err)
	} else {
		defer s.Cancel()
		go s.UpdateFeeds()
		// add api routes
		e.POST("/user", s.AddUser)
		e.POST("/follower", s.AddFollower)
		e.POST("/publication", s.AddPublication)
		e.GET("/feed/:userId", s.GetFeed)
		// run http server
		e.Logger.Fatal(e.Start(":1234"))
	}
}
