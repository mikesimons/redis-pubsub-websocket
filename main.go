package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type App struct {
	RedisAddr          string
	ListenAddr         string
	DisableOriginCheck bool
	Upgrader           websocket.Upgrader
	redis              *redis.Client
}

func main() {
	app := &App{}

	var rootCmd = &cobra.Command{
		Use:   "redis-pubsub",
		Short: "Small bridge between redis pub sub & websockets",
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.DebugLevel)
			log.SetFormatter(&log.JSONFormatter{})

			app.Upgrader = websocket.Upgrader{}

			if app.DisableOriginCheck {
				app.Upgrader.CheckOrigin = func(r *http.Request) bool {
					return true
				}
			}

			app.redis = redis.NewClient(&redis.Options{
				Addr: app.RedisAddr,
			})

			http.HandleFunc("/ws", app.wsHandler)
			log.WithFields(log.Fields{"addr": app.ListenAddr}).Info("listening")

			http.ListenAndServe(app.ListenAddr, nil)
		},
	}

	rootCmd.Flags().StringVar(&app.RedisAddr, "redis-addr", "localhost:6379", "Redis server address")
	rootCmd.Flags().StringVar(&app.ListenAddr, "listen", ":8080", "Listen address")
	rootCmd.Flags().BoolVar(&app.DisableOriginCheck, "disable-origin-check", false, "Disable websocket upgrade origin check (for dev)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func (app *App) onWebsocketClose(conn *websocket.Conn, fn func()) {
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				fn()
				break
			}
		}
	}()
}

func (app *App) wsHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	topics := params["t"]

	ws, err := app.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("could not upgrade websocket")
		return
	}

	ch := make(chan *redis.Message, 10)
	app.onWebsocketClose(ws, func() {
		log.Debug("detected websocket close")
		ws.Close()
		close(ch)
	})

	for _, topic := range topics {
		pubsub := app.redis.Subscribe(topic)
		defer pubsub.Close()

		_, err := pubsub.Receive()
		if err != nil {
			log.WithFields(log.Fields{"error": err, "topic": topic}).Error("could not subscribe to topic")
			return
		}

		rch := pubsub.Channel()
		log.WithFields(log.Fields{"topic": topic}).Debug("subscription created")

		go func() {
			for msg := range rch {
				log.WithFields(log.Fields{
					"topic":   msg.Channel,
					"payload": msg.Payload,
				}).Debug("recieved message for topic")
				ch <- msg
			}
		}()
	}

	for msg := range ch {
		writer, err := ws.NextWriter(websocket.TextMessage)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Debug("could not get websocket writer")
			break
		}

		_, err = writer.Write([]byte(msg.Payload))

		if err != nil {
			log.WithFields(log.Fields{"error": err}).Debug("could not write message to websocket")
			break
		}

		writer.Close()
	}

	log.Debug("websocket closed")
}
