package chat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
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
	ticker := time.NewTicker(PingPeriod)

	var mutex = &sync.Mutex{}

	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(PingWait))
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

func (api *Api) GetChatMessage(conn *websocket.Conn, cli *openai.Client, mutex *sync.Mutex, reqMsgs []openai.ChatCompletionMessage) {
	var err error
	var strResp string

	ctx := context.Background()

	switch api.Config.Model {
	case openai.GPT3Dot5Turbo0301, openai.GPT3Dot5Turbo, openai.GPT4, openai.GPT40314, openai.GPT432K0314, openai.GPT432K:
		prompt := reqMsgs[len(reqMsgs)-1].Content
		req := openai.ChatCompletionRequest{
			Model:            api.Config.Model,
			MaxTokens:        api.Config.MaxLength,
			Temperature:      1.0,
			Messages:         reqMsgs,
			Stream:           true,
			TopP:             1,
			FrequencyPenalty: 0.1,
			PresencePenalty:  0.1,
		}

		stream, err := cli.CreateChatCompletionStream(ctx, req)
		if err != nil {
			err = fmt.Errorf("[ERROR] create ChatGPT stream model=%s error: %s", api.Config.Model, err.Error())
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
			if err != nil {
				var s string
				var kind string
				if errors.Is(err, io.EOF) {
					if i == 0 {
						s = "[ERROR] NO RESPONSE, PLEASE RETRY"
						kind = "retry"
					} else {
						s = "\n\n###### [END] ######"
						kind = "chat"
					}
				} else {
					s = fmt.Sprintf("[ERROR] %s", err.Error())
					kind = "error"
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
				break
			}

			if len(response.Choices) > 0 {
				var s string
				if i == 0 {
					s = fmt.Sprintf("%s# %s\n\n", s, prompt)
				}
				for _, choice := range response.Choices {
					s = s + choice.Delta.Content
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
			api.Logger.LogInfo(fmt.Sprintf("[RESPONSE] %s\n", strResp))
		}
	case openai.GPT3TextDavinci003, openai.GPT3TextDavinci002, openai.GPT3TextCurie001, openai.GPT3TextBabbage001, openai.GPT3TextAda001, openai.GPT3TextDavinci001, openai.GPT3DavinciInstructBeta, openai.GPT3Davinci, openai.GPT3CurieInstructBeta, openai.GPT3Curie, openai.GPT3Ada, openai.GPT3Babbage:
		prompt := reqMsgs[len(reqMsgs)-1].Content
		req := openai.CompletionRequest{
			Model:       api.Config.Model,
			MaxTokens:   api.Config.MaxLength,
			Temperature: 0.6,
			Prompt:      prompt,
			Stream:      true,
			//Stop:             []string{"\n\n\n"},
			TopP:             1,
			FrequencyPenalty: 0.1,
			PresencePenalty:  0.1,
		}

		stream, err := cli.CreateCompletionStream(ctx, req)
		if err != nil {
			err = fmt.Errorf("[ERROR] create ChatGPT stream model=%s error: %s", api.Config.Model, err.Error())
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
			if err != nil {
				var s string
				var kind string
				if errors.Is(err, io.EOF) {
					if i == 0 {
						s = "[ERROR] NO RESPONSE, PLEASE RETRY"
						kind = "retry"
					} else {
						s = "\n\n###### [END] ######"
						kind = "chat"
					}
				} else {
					s = fmt.Sprintf("[ERROR] %s", err.Error())
					kind = "error"
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
				break
			}

			if len(response.Choices) > 0 {
				var s string
				if i == 0 {
					s = fmt.Sprintf("%s# %s\n\n", s, prompt)
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
			api.Logger.LogInfo(fmt.Sprintf("[RESPONSE] %s\n", strResp))
		}
	default:
		err = fmt.Errorf("model not exists")
		api.Logger.LogError(err.Error())
		return
	}
}

func (api *Api) GetImageMessage(conn *websocket.Conn, cli *openai.Client, mutex *sync.Mutex, requestMsg string) {
	var err error

	ctx := context.Background()

	prompt := strings.TrimPrefix(requestMsg, "/image ")
	req := openai.ImageRequest{
		Prompt:         prompt,
		Size:           openai.CreateImageSize256x256,
		ResponseFormat: openai.CreateImageResponseFormatB64JSON,
		N:              1,
	}

	sendError := func(err error) {
		err = fmt.Errorf("[ERROR] generate image error: %s", err.Error())
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
	}

	resp, err := cli.CreateImage(ctx, req)
	if err != nil {
		err = fmt.Errorf("[ERROR] generate image error: %s", err.Error())
		sendError(err)
		return
	}
	if len(resp.Data) == 0 {
		err = fmt.Errorf("[ERROR] generate image error: result is empty")
		sendError(err)
		return
	}

	imgBytes, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if err != nil {
		err = fmt.Errorf("[ERROR] image base64 decode error: %s", err.Error())
		sendError(err)
		return
	}

	date := time.Now().Format("2006-01-02")
	imageDir := fmt.Sprintf("assets/images/%s", date)
	err = os.MkdirAll(imageDir, 0700)
	if err != nil {
		err = fmt.Errorf("[ERROR] create image directory error: %s", err.Error())
		sendError(err)
		return
	}

	imageFileName := fmt.Sprintf("%s.png", RandomString(16))
	err = os.WriteFile(fmt.Sprintf("%s/%s", imageDir, imageFileName), imgBytes, 0600)
	if err != nil {
		err = fmt.Errorf("[ERROR] write png image error: %s", err.Error())
		sendError(err)
		return
	}

	msg := fmt.Sprintf("api/%s/%s", imageDir, imageFileName)
	chatMsg := Message{
		Kind:       "image",
		Msg:        msg,
		MsgId:      uuid.New().String(),
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
	}
	mutex.Lock()
	_ = conn.WriteJSON(chatMsg)
	mutex.Unlock()
	api.Logger.LogInfo(fmt.Sprintf("[IMAGE] # %s\n%s", requestMsg, msg))
	return
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

	_ = conn.SetReadDeadline(time.Now().Add(PingWait))
	conn.SetPongHandler(func(s string) error {
		_ = conn.SetReadDeadline(time.Now().Add(PingWait))
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
	cli := openai.NewClient(api.Config.ApiKey)

	reqMsgs := make([]openai.ChatCompletionMessage, 0)

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
					if strings.HasPrefix(requestMsg, "/image ") {
						chatMsg := Message{
							Kind:       "receive",
							Msg:        requestMsg,
							MsgId:      uuid.New().String(),
							CreateTime: time.Now().Format("2006-01-02 15:04:05"),
						}
						mutex.Lock()
						_ = conn.WriteJSON(chatMsg)
						mutex.Unlock()
						go api.GetImageMessage(conn, cli, mutex, requestMsg)
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
						reqMsgs = append(reqMsgs, openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleUser,
							Content: requestMsg,
						})
						go api.GetChatMessage(conn, cli, mutex, reqMsgs)
					}
				}
			}
		case websocket.CloseMessage:
			isClosed = true
			api.Logger.LogInfo("[CLOSED] websocket receive closed message")
		case websocket.PingMessage:
			_ = conn.SetReadDeadline(time.Now().Add(PingWait))
			api.Logger.LogInfo("[PING] websocket receive ping message")
		case websocket.PongMessage:
			_ = conn.SetReadDeadline(time.Now().Add(PingWait))
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
