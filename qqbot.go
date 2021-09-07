package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	jsoniter "github.com/json-iterator/go"
)

var globalGroupId = 67279950

//var globalGroupId = 985271542

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var eventUrl = "ws://localhost:6700/event"
var apiUrl = "ws://localhost:6700/api"

var fansUrl = "https://api.bilibili.com/x/relation/stat?vmid=672328094&jsonp=jsonp"

var apiClient *(websocket.Conn)
var eventClient *(websocket.Conn)

var done = make(chan struct{})
var interrupt = make(chan os.Signal, 1)

type SendMsg struct {
	GroupId string `json:"group_id"`
	Message string `json:"message"`
}

type Api struct {
	Action string  `json:"action"`
	Params SendMsg `json:"params"`
}

type Sender struct {
	Card     string `json:"card"`
	NickName string `json:"nickname"`
}

type Event struct {
	PostType    string `json:"post_type"`
	MessageType string `json:"message_type"`
	Message     string `json:"message"`
	GroupId     int    `json:"group_id"`
	SubType     string `json:"sub_type"`
	Sender      Sender `json:"sender"`
}

type FollowerData struct {
	Follower int `json:"follower"`
}

type FollowerRsp struct {
	Code int          `json:"post_type"`
	Data FollowerData `json:"data"`
}

func main() {
	log.SetFlags(0)

	signal.Notify(interrupt, os.Interrupt)
	var err error
	eventClient, _, err = websocket.DefaultDialer.Dial(eventUrl, nil)
	if err != nil {
		log.Fatal("dial event err:", err)
		return
	}
	apiClient, _, err = websocket.DefaultDialer.Dial(apiUrl, nil)
	if err != nil {
		log.Fatal("dial api err:", err)
		return
	}
	defer eventClient.Close()
	defer apiClient.Close()

	go dynamic_loop()

	go tiktok_loop()

	go func() {
		defer close(done)
		for {
			_, message, err := eventClient.ReadMessage()
			if err != nil {
				log.Fatal("read event err:", err)
				return
			}

			//fmt.Println("read msg:", string(message))
			event := Event{}
			err = json.Unmarshal(message, &event)
			if err != nil {
				log.Println("parse event err:%s", err)
				continue
			}

			if event.PostType == "message" && event.GroupId == globalGroupId {
				if event.Message == "嘉然粉丝数" || event.Message == "然然粉丝数" || event.Message == "粉丝数" {
					fans_event(&event)
				} else if event.Message == "roll" || event.Message == "Roll" || event.Message == "ROLL" {
					roll_event(&event)
				} else if event.Message == "然然动态" || event.Message == "嘉然动态" || event.Message == "动态" {
					dynamic_content_event(&event)
				} else if event.Message == "然然投稿" || event.Message == "嘉然投稿" || event.Message == "投稿" {
					dynamic_video_event(&event)
				} else if event.Message == "嘉然抖音视频" || event.Message == "嘉然抖音" {
					send_tiktok()
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")
			apiClient.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			eventClient.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func roll_event(event *Event) {
	randNum := rand.Intn(100)
	var name string
	if event.Sender.Card != "" {
		name = event.Sender.Card
	} else {
		name = event.Sender.NickName
	}

	msg := fmt.Sprintf("%s的roll结果为:%d", name, randNum)
	sendMsg := SendMsg{GroupId: strconv.Itoa(event.GroupId), Message: msg}
	api := Api{Action: "send_group_msg", Params: sendMsg}
	data, _ := json.Marshal(&api)

	fmt.Println("send msg:", string(data))
	err := apiClient.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Fatal("send fans msg err:", err)
		close(done)
		return
	}
}

func fans_event(event *Event) {
	rsp, err := http.Get(fansUrl)
	if err != nil {
		log.Fatal("get fans err:", err)
		return
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		log.Fatal("read fans rsp err:", err)
		return
	}

	fansRsp := FollowerRsp{}
	json.Unmarshal(body, &fansRsp)
	fans := fansRsp.Data.Follower

	msg := fmt.Sprintf("嘉然实时粉丝数为:%d", fans)
	sendMsg := SendMsg{GroupId: strconv.Itoa(event.GroupId), Message: msg}
	api := Api{Action: "send_group_msg", Params: sendMsg}
	data, _ := json.Marshal(&api)

	err = apiClient.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Fatal("send fans msg err:", err)
		close(done)
		return
	}
}

func dynamic_content_event(event *Event) {
	if lastContent == nil {
		return
	}

	date := time.Unix(int64(lastContent.Desc.Timestamp), 0).Format("2006-01-02 15:04:05")
	url := fmt.Sprintf("http://t.bilibili.com/%d", lastContent.Desc.DynamicId)
	msg := fmt.Sprintf("嘉然最近动态发布于%s:\n--------------------------\n%s\n\n%s", date, lastContent.Msg, url)

	sendMsg := SendMsg{GroupId: strconv.Itoa(event.GroupId), Message: msg}
	api := Api{Action: "send_group_msg", Params: sendMsg}
	data, _ := json.Marshal(&api)
	err := apiClient.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Fatal("send fans msg err:", err)
		close(done)
		return
	}
}

func dynamic_video_event(event *Event) {
	if lastVideo == nil {
		return
	}

	date := time.Unix(int64(lastVideo.Desc.Timestamp), 0).Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("嘉然最近视频发布于%s:\n--------------------------\n%s\n", date, lastVideo.Msg)

	sendMsg := SendMsg{GroupId: strconv.Itoa(event.GroupId), Message: msg}
	api := Api{Action: "send_group_msg", Params: sendMsg}
	data, _ := json.Marshal(&api)
	err := apiClient.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Fatal("send fans msg err:", err)
		close(done)
		return
	}
}
