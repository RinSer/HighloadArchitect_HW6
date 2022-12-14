package feed

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v9"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/net/websocket"
)

const (
	FeedMaxSize           = 1000
	WebsocketExchangeName = "FeedExchange"
)

type Service struct {
	ctx    context.Context
	cancel context.CancelFunc
	db     *sql.DB
	rdb    *redis.Client
	conn   *amqp.Connection
	ch     *amqp.Channel
	queue  *amqp.Queue
}

func NewService(
	sqlConnection string,
	redisHost string,
	rabbitConnection string) (*Service, error) {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	// connect to MySQL
	db, err := sql.Open("mysql", sqlConnection)
	if err != nil {
		return nil, err
	}

	// connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// connect to RabbitMQ
	conn, err := amqp.Dial(rabbitConnection)
	if err != nil {
		return nil, err
	}
	ch, queue := addChannel(conn)

	return &Service{
		ctx, cancel, db, rdb, conn, ch, queue,
	}, nil
}

// API handlers

func (s *Service) AddUser(c echo.Context) (err error) {
	u := new(User)
	err = c.Bind(u)
	if err != nil {
		return
	}
	tx, err := s.db.BeginTx(s.ctx, nil)
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}()
	if err != nil {
		return
	}
	_, err = tx.ExecContext(s.ctx,
		`INSERT INTO users (login) values (?);`, u.Login)
	if err != nil {
		return
	}
	row := tx.QueryRowContext(s.ctx, `SELECT LAST_INSERT_ID();`)
	row.Scan(&u.Id)
	return c.JSON(http.StatusCreated, u.Id)
}

func (s *Service) AddFollower(c echo.Context) (err error) {
	var rowsAffected int64
	f := new(Follower)
	err = c.Bind(f)
	if err != nil {
		return
	}
	remove, _ := strconv.ParseBool(c.QueryParam("remove"))
	if remove {
		var tag sql.Result
		tag, err = s.db.ExecContext(s.ctx,
			`DELETE FROM followers WHERE userId = ? && followerId = ?;`,
			f.UserId, f.FollowerId)
		if err != nil {
			return
		}
		rowsAffected, err = tag.RowsAffected()
		if err != nil {
			return
		}
		s.rdb.SRem(s.ctx, followedSetKey(f.UserId), f.FollowerId)
		// invalidate unfollowed publications
		followerId := strconv.FormatInt(f.FollowerId, 10)
		pubsList := s.rdb.LRange(s.ctx, followerId, 0, FeedMaxSize)
		pubs, err := pubsList.Result()
		if err != nil {
			return err
		}
		for _, pub := range pubs {
			p := new(Publication)
			err = json.Unmarshal([]byte(pub), p)
			if err != nil {
				return err
			}
			if p.Author == f.UserId {
				s.rdb.LRem(s.ctx, followerId, 0, pub)
			}
		}
	} else {
		var tag sql.Result
		tag, err = s.db.ExecContext(s.ctx,
			`INSERT INTO followers (userId, followerId) values (?, ?);`,
			f.UserId, f.FollowerId)
		if err != nil {
			return
		}
		rowsAffected, err = tag.RowsAffected()
		if err != nil {
			return
		}
		s.rdb.SAdd(s.ctx, followedSetKey(f.UserId), f.FollowerId)
	}
	return c.JSON(http.StatusCreated, rowsAffected == 1)
}

func (s *Service) AddPublication(c echo.Context) (err error) {
	p := new(Publication)
	err = c.Bind(p)
	if err != nil {
		return
	}
	p.At = time.Now()
	tx, err := s.db.BeginTx(s.ctx, nil)
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}()
	if err != nil {
		return
	}
	_, err = tx.ExecContext(s.ctx,
		`INSERT INTO publications (author, txt, createdAt) values (?, ?, ?);`,
		p.Author, p.Text, p.At)
	if err != nil {
		return
	}
	row := tx.QueryRowContext(s.ctx, `SELECT LAST_INSERT_ID();`)
	row.Scan(&p.Id)
	err = s.SendPublicationToQueue(p)
	if err != nil {
		return
	}
	return c.JSON(http.StatusCreated, p)
}

func (s *Service) GetFeed(c echo.Context) (err error) {
	userId := c.Param("userId")
	pubsList := s.rdb.LRange(s.ctx, userId, 0, FeedMaxSize)
	pubs, err := pubsList.Result()
	if err != nil {
		return
	}
	publications := make([]Publication, len(pubs))
	for idx, pub := range pubs {
		p := new(Publication)
		err = json.Unmarshal([]byte(pub), p)
		if err != nil {
			return
		}
		publications[idx] = *p
	}
	return c.JSON(http.StatusOK, publications)
}

func (s *Service) UpdateFeed(c echo.Context) (err error) {
	userId := c.Param("userId")
	// add user's queue and start consuming messages
	routingKey := fmt.Sprintf("user.%s", userId)
	queue, err := s.ch.QueueDeclare(
		routingKey+"."+uuid.NewString(), // name
		false,                           // durable
		false,                           // delete when unused
		false,                           // exclusive
		false,                           // no-wait
		nil,                             // arguments
	)
	if err != nil {
		return
	}
	err = s.ch.QueueBind(
		queue.Name,
		routingKey,
		WebsocketExchangeName,
		true,
		nil,
	)
	if err != nil {
		return
	}
	msgs, err := s.ch.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return
	}
	// send messages to the websocket
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		defer s.ch.QueueDelete(queue.Name, false, false, true)
		for msg := range msgs {
			err := websocket.Message.Send(ws, msg.Body)
			if err != nil {
				c.Logger().Error(err)
			}
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

// AMQP Methods

func (s *Service) SendPublicationToQueue(pub *Publication) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(pub)
	if err != nil {
		return err
	}

	return s.ch.PublishWithContext(ctx,
		"",           // exchange
		s.queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
}

func (s *Service) SendPublicationToExchange(followerId string, pub *Publication) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(pub)
	if err != nil {
		return err
	}

	return s.ch.PublishWithContext(ctx,
		WebsocketExchangeName,              // exchange
		fmt.Sprintf("user.%s", followerId), // routing key
		true,                               // mandatory
		false,                              // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
}

func (s *Service) UpdateFeeds() {
	msgs, err := s.ch.Consume(
		s.queue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		log.Print(err, "Failed to register a consumer")
	}

	go func() {
		for msg := range msgs {
			p := new(Publication)
			_ = json.Unmarshal(msg.Body, p)
			followersSet := s.rdb.SMembers(s.ctx, followedSetKey(p.Author))
			followers, _ := followersSet.Result()
			for _, follower := range followers {
				// send publication to websocket
				err := s.SendPublicationToExchange(follower, p)
				if err != nil {
					log.Println(err.Error())
				}
				// add publication to the cashed feed
				res := s.rdb.LPush(s.ctx, follower, string(msg.Body))
				err = res.Err()
				if err != nil {
					log.Println(err.Error())
				}
				// feed should contain no more than FeedMaxSize items
				resTrim := s.rdb.LTrim(s.ctx, follower, 0, FeedMaxSize)
				err = resTrim.Err()
				if err != nil {
					log.Println(err.Error())
				}
			}
		}
	}()

	<-s.ctx.Done()
}

// Helpers

func (s *Service) Db() *sql.DB {
	return s.db
}

func (s *Service) Cancel() {
	defer s.ch.Close()
	defer s.conn.Close()
	s.cancel()
}

func (s *Service) ReopenChannel() {
	errReceiver := make(chan *amqp.Error)
	s.ch.NotifyClose(errReceiver)
	for err := range errReceiver {
		log.Print(err)
		close(errReceiver)
		s.ch, s.queue = addChannel(s.conn)
		errReceiver = make(chan *amqp.Error)
		s.ch.NotifyClose(errReceiver)
	}
}

func addChannel(conn *amqp.Connection) (*amqp.Channel, *amqp.Queue) {
	ch, err := conn.Channel()
	if err != nil {
		log.Print(err)
		return addChannel(conn)
	}
	queue, err := ch.QueueDeclare(
		"publications", // name
		false,          // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Print(err)
		return addChannel(conn)
	}
	err = ch.ExchangeDeclare(
		WebsocketExchangeName, // name
		"direct",              // kind
		true,                  // durable
		false,                 // autoDelete
		false,                 // internal
		false,                 // noWait
		nil,                   // args
	)
	if err != nil {
		log.Print(err)
		return addChannel(conn)
	}
	return ch, &queue
}

func followedSetKey(userId int64) string {
	return fmt.Sprintf("%dfollowedBy", userId)
}
