package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	gogpt "github.com/sashabaranov/go-gpt3"
)

type Api struct {
	Config Config
	Logger
}

type ApiResponse struct {
	Status   string      `yaml:"status" json:"status" bson:"status" validate:""`
	Msg      string      `yaml:"msg" json:"msg" bson:"msg" validate:""`
	Duration string      `yaml:"duration" json:"duration" bson:"duration" validate:""`
	Data     interface{} `yaml:"data" json:"data" bson:"data" validate:""`
}

type Message struct {
	Msg        string `yaml:"msg" json:"msg" bson:"msg" validate:""`
	MsgId      string `yaml:"msgId" json:"msgId" bson:"msgId" validate:""`
	Kind       string `yaml:"kind" json:"kind" bson:"kind" validate:""`
	CreateTime string `yaml:"createTime" json:"createTime" bson:"createTime" validate:""`
}

func (api *Api) responseFunc(c *gin.Context, startTime time.Time, status, msg string, httpStatus int, data map[string]interface{}) {
	duration := time.Since(startTime)
	ar := ApiResponse{
		Status:   status,
		Msg:      msg,
		Duration: duration.String(),
		Data:     data,
	}
	c.JSON(httpStatus, ar)
}

func (api *Api) wsPingMsg(conn *websocket.Conn, chClose, chIsCloseSet chan int) {
	var err error
	ticker := time.NewTicker(pingPeriod)

	var mutex = &sync.Mutex{}

	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(pingWait))
			mutex.Lock()
			err = conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
			mutex.Unlock()
		case <-chClose:
			api.LogInfo(fmt.Sprintf("# websocket connection closed"))
			chIsCloseSet <- 0
			return
		}
	}
}

func (api *Api) GetChatMessage(conn *websocket.Conn, cli *gogpt.Client, mutex *sync.Mutex, requestMsg string) {
	var err error
	var strResp string
	req := gogpt.CompletionRequest{
		// Model:            gogpt.GPT3TextDavinci003,
		Model:            "gpt-3.5-turbo",
		MaxTokens:        api.Config.MaxLength,
		Temperature:      0.6,
		Prompt:           requestMsg,
		Stream:           true,
		Stop:             []string{"\n\n\n"},
		TopP:             1,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.1,
	}

	ctx := context.Background()

	stream, err := cli.CreateCompletionStream(ctx, req)
	if err != nil {
		err = fmt.Errorf("[ERROR] create chatGPT stream error: %s", err.Error())
		chatMsg := Message{
			Kind:       "error",
			Msg:        err.Error(),
			MsgId:      uuid.New().String(),
			CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		}
		mutex.Lock()
		_ = conn.WriteJSON(chatMsg)
		mutex.Unlock()
		api.Logger.LogError(err.Error())
		return
	}
	defer stream.Close()

	id := uuid.New().String()
	var i int
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			var s string
			var kind string
			if i == 0 {
				s = "[ERROR] NO RESPONSE, PLEASE RETRY"
				kind = "retry"
			} else {
				s = "\n\n###### [END] ######"
				kind = "chat"
			}
			chatMsg := Message{
				Kind:       kind,
				Msg:        s,
				MsgId:      id,
				CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			}
			mutex.Lock()
			_ = conn.WriteJSON(chatMsg)
			mutex.Unlock()
			if kind == "retry" {
				api.Logger.LogError(s)
			}
			break
		} else if err != nil {
			err = fmt.Errorf("[ERROR] receive chatGPT stream error: %s", err.Error())
			chatMsg := Message{
				Kind:       "error",
				Msg:        err.Error(),
				MsgId:      id,
				CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			}
			mutex.Lock()
			_ = conn.WriteJSON(chatMsg)
			mutex.Unlock()
			api.Logger.LogError(err.Error())
			break
		}

		if len(response.Choices) > 0 {
			var s string
			if i == 0 {
				s = fmt.Sprintf(`%s# %s`, s, requestMsg)
			}
			for _, choice := range response.Choices {
				s = s + choice.Text
			}
			strResp = strResp + s
			chatMsg := Message{
				Kind:       "chat",
				Msg:        s,
				MsgId:      id,
				CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			}
			mutex.Lock()
			_ = conn.WriteJSON(chatMsg)
			mutex.Unlock()
		}
		i = i + 1
	}
	if strResp != "" {
		api.Logger.LogInfo(fmt.Sprintf("[RESPONSE] %s", strResp))
	}
}

func (api *Api) WsChat(c *gin.Context) {
	startTime := time.Now()
	status := StatusFail
	msg := ""
	httpStatus := http.StatusForbidden
	data := map[string]interface{}{}

	wsupgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	wsupgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	mutex := &sync.Mutex{}
	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		err = fmt.Errorf("[ERROR] failed to upgrade websocket %s", err.Error())
		msg = err.Error()
		api.responseFunc(c, startTime, status, msg, httpStatus, data)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(pingWait))
	conn.SetPongHandler(func(s string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pingWait))
		return nil
	})

	var isClosed bool
	chClose := make(chan int)
	chIsCloseSet := make(chan int)
	defer func() {
		conn.Close()
	}()
	go api.wsPingMsg(conn, chClose, chIsCloseSet)
	go func() {
		for {
			select {
			case <-chIsCloseSet:
				isClosed = true
				return
			}
		}
	}()

	api.Logger.LogInfo(fmt.Sprintf("websocket connection open"))
	cli := gogpt.NewClient(api.Config.AppKey)

	var latestRequestTime time.Time
	for {
		if isClosed {
			return
		}
		// read in a message
		messageType, bs, err := conn.ReadMessage()
		if err != nil {
			err = fmt.Errorf("[ERROR] read message error: %s", err.Error())
			api.Logger.LogError(err.Error())
			return
		}
		switch messageType {
		case websocket.TextMessage:
			requestMsg := string(bs)
			api.Logger.LogInfo(fmt.Sprintf("[REQUEST] %s", requestMsg))
			var ok bool
			if latestRequestTime.IsZero() {
				latestRequestTime = time.Now()
				ok = true
			} else {
				if time.Since(latestRequestTime) < time.Second*time.Duration(api.Config.IntervalSeconds) {
					err = fmt.Errorf("[ERROR] please wait %d seconds for next query", api.Config.IntervalSeconds)
					chatMsg := Message{
						Kind:       "error",
						Msg:        err.Error(),
						MsgId:      uuid.New().String(),
						CreateTime: time.Now().Format("2006-01-02 15:04:05"),
					}
					mutex.Lock()
					_ = conn.WriteJSON(chatMsg)
					mutex.Unlock()
					api.Logger.LogError(err.Error())
				} else {
					ok = true
					latestRequestTime = time.Now()
				}
			}
			if ok {
				if len(strings.Trim(requestMsg, " ")) < 2 {
					err = fmt.Errorf("[ERROR] message too short")
					chatMsg := Message{
						Kind:       "error",
						Msg:        err.Error(),
						MsgId:      uuid.New().String(),
						CreateTime: time.Now().Format("2006-01-02 15:04:05"),
					}
					mutex.Lock()
					_ = conn.WriteJSON(chatMsg)
					mutex.Unlock()
					api.Logger.LogError(err.Error())
				} else {
					chatMsg := Message{
						Kind:       "receive",
						Msg:        requestMsg,
						MsgId:      uuid.New().String(),
						CreateTime: time.Now().Format("2006-01-02 15:04:05"),
					}
					mutex.Lock()
					_ = conn.WriteJSON(chatMsg)
					mutex.Unlock()
					go api.GetChatMessage(conn, cli, mutex, requestMsg)
				}
			}
		case websocket.CloseMessage:
			isClosed = true
			api.Logger.LogInfo("[CLOSED] websocket receive closed message")
		case websocket.PingMessage:
			_ = conn.SetReadDeadline(time.Now().Add(pingWait))
			api.Logger.LogInfo("[PING] websocket receive ping message")
		case websocket.PongMessage:
			_ = conn.SetReadDeadline(time.Now().Add(pingWait))
			api.Logger.LogInfo("[PONG] websocket receive pong message")
		default:
			err = fmt.Errorf("[ERROR] websocket receive message type not text")
			chatMsg := Message{
				Kind:       "error",
				Msg:        err.Error(),
				MsgId:      uuid.New().String(),
				CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			}
			mutex.Lock()
			_ = conn.WriteJSON(chatMsg)
			mutex.Unlock()
			api.Logger.LogError(err.Error())
			return
		}
	}
}
