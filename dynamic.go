package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

//var Url = "https://api.vc.bilibili.com/dynamic_svr/v1/dynamic_svr/get_dynamic_detail?dynamic_id=561291168340395906"
//var dynamicUrl = "https://api.vc.bilibili.com/dynamic_svr/v1/dynamic_svr/space_history?host_uid=226729&offset_dynamic_id=0"

var dynamicUrl = "https://api.vc.bilibili.com/dynamic_svr/v1/dynamic_svr/space_history?host_uid=672328094&offset_dynamic_id=0"
var lastContent *DynamicDetail = nil
var lastVideo *DynamicDetail = nil
var lastDynamicId int = 0

var globalSendFlag = false

type DynamicDetail struct {
	Desc       CardDesc
	Msg        string
	OriginName string
	RepostMsg  string
}

type RepostCard struct {
	Item       ContentItem `json:"item"`
	Origin     string      `json:"origin"`
	OriginUser OriginUser  `json:"origin_user"`
}

type OriginUser struct {
	Info OriginUserInfo `json:"info"`
}

type OriginUserInfo struct {
	Name string `json:"uname"`
}

type ImageCard struct {
	Item ImageItem `json:"item"`
}

type ImageItem struct {
	Description string  `json:"description"`
	Pic         []Image `json:"pictures"`
}

type Image struct {
	ImgUrl string `json:"img_src"`
}

type VideoCard struct {
	ShortLink string `json:"short_link"`
	Title     string `json:"title"`
	Pic       string `json:"pic"`
}

type ContentCard struct {
	Item ContentItem `json:"item"`
}

type ContentItem struct {
	Content string `json:"content"`
}

type CardDesc struct {
	Timestamp  int `json:"timestamp"`
	DynamicId  int `json:"dynamic_id"`
	Type       int `json:"type"`
	Like       int `json:"like"`
	Comment    int `json:"comment"`
	OriginType int `json:"orig_type"`
}

type Card struct {
	Desc CardDesc `json:"desc"`
	Data string   `json:"card"`
}

type DynamicData struct {
	Cards []Card `json:"cards"`
}

type BiliRsp struct {
	Code int         `json:"code"`
	Data DynamicData `json:"data"`
}

func dynamic_loop() {
	globalSendFlag = false
	dynamic()
	globalSendFlag = true
	ticker := time.NewTicker(time.Second * 75)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			dynamic()
		case <-interrupt:
			log.Println("interrupt")
			return
		}
	}
}

func dynamic() {
	fmt.Println("dynamic loop")
	resp, err := http.Get(dynamicUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	//fmt.Println("body:", string(body))
	biliRsp := BiliRsp{}
	err = json.Unmarshal([]byte(body), &biliRsp)
	if err != nil {
		fmt.Printf("decode dynamic rsp json err:%s\n", err)
		return
	}
	if biliRsp.Code != 0 {
		fmt.Printf("bili rsp code:%d\n", biliRsp.Code)
		return
	}

	if len(biliRsp.Data.Cards) <= 0 {
		return
	}

	newDynamicId := biliRsp.Data.Cards[0].Desc.DynamicId

	for _, card := range biliRsp.Data.Cards {
		if card.Desc.DynamicId == lastDynamicId {
			break
		}
		if card.Desc.Type == 1 {
			parse_repost_dynamic(&card)
		} else if card.Desc.Type == 2 {
			parse_image_dynamic(&card)
		} else if card.Desc.Type == 4 {
			parse_content_dynamic(&card)
		} else if card.Desc.Type == 8 {
			parse_vedio_dynamic(&card)
		}
	}

	lastDynamicId = newDynamicId
}

func parse_content_dynamic(card *Card) {
	if lastContent != nil && lastContent.Desc.Timestamp >= card.Desc.Timestamp {
		return
	}

	msg := content_dynamic_msg(card.Data)
	if msg == "" {
		return
	}

	dynamic := DynamicDetail{}
	dynamic.Msg = msg
	dynamic.Desc = card.Desc
	lastContent = &dynamic

	if globalSendFlag {
		send_dynamic_event(&dynamic)
	}
}

func content_dynamic_msg(data string) string {
	contentCard := ContentCard{}
	err := json.Unmarshal([]byte(data), &contentCard)
	if err != nil {
		fmt.Printf("decode content card json err:%s\n", err)
		return ""
	}

	return contentCard.Item.Content
}

func parse_image_dynamic(card *Card) {
	if lastContent != nil && lastContent.Desc.Timestamp >= card.Desc.Timestamp {
		return
	}

	msg := image_dynamic_msg(card.Data)
	if msg == "" {
		return
	}

	dynamic := DynamicDetail{}
	dynamic.Msg = msg
	dynamic.Desc = card.Desc
	lastContent = &dynamic

	if globalSendFlag {
		send_dynamic_event(&dynamic)
	}
}

func image_dynamic_msg(data string) string {
	imageCard := ImageCard{}
	err := json.Unmarshal([]byte(data), &imageCard)
	if err != nil {
		fmt.Printf("decode image card json err:%s\n", err)
		return ""
	}

	msg := fmt.Sprintf("%s\n", imageCard.Item.Description)
	for _, pic := range imageCard.Item.Pic {
		msg += fmt.Sprintf("[CQ:image,file=%s]", pic.ImgUrl)
	}

	return msg
}

func parse_vedio_dynamic(card *Card) {
	if lastVideo != nil && lastVideo.Desc.Timestamp >= card.Desc.Timestamp {
		return
	}

	msg := video_dynamic_msg(card.Data)
	if msg == "" {
		return
	}

	dynamic := DynamicDetail{}
	dynamic.Msg = msg
	dynamic.Desc = card.Desc
	lastVideo = &dynamic

	if globalSendFlag {
		send_dynamic_event(&dynamic)
	}
}

func video_dynamic_msg(data string) string {
	videoCard := VideoCard{}
	err := json.Unmarshal([]byte(data), &videoCard)
	if err != nil {
		fmt.Printf("decode content card json err:%s\n", err)
		return ""
	}
	msg := fmt.Sprintf("%s\n[CQ:image,file=%s]\n\n%s", videoCard.Title, videoCard.Pic, videoCard.ShortLink)
	return msg
}

func parse_repost_dynamic(card *Card) {
	repostCard := RepostCard{}
	err := json.Unmarshal([]byte(card.Data), &repostCard)
	if err != nil {
		fmt.Printf("decode repost card json err:%s\n", err)
		return
	}
	var msg string
	if card.Desc.OriginType == 2 {
		msg = image_dynamic_msg(repostCard.Origin)
	} else if card.Desc.OriginType == 4 {
		msg = content_dynamic_msg(repostCard.Origin)
	} else if card.Desc.OriginType == 8 {
		msg = video_dynamic_msg(repostCard.Origin)
	}

	dynamic := DynamicDetail{}
	dynamic.Msg = msg
	dynamic.Desc = card.Desc
	dynamic.OriginName = repostCard.OriginUser.Info.Name
	dynamic.RepostMsg = repostCard.Item.Content
	lastVideo = &dynamic

	if globalSendFlag {
		send_dynamic_event(&dynamic)
	}
}

func send_dynamic_event(dynamic *DynamicDetail) {
	date := time.Unix(int64(dynamic.Desc.Timestamp), 0).Format("2006-01-02 15:04:05")

	var msg string
	if dynamic.Desc.Type == 2 || dynamic.Desc.Type == 4 {
		url := fmt.Sprintf("http://t.bilibili.com/%d", dynamic.Desc.DynamicId)
		msg = fmt.Sprintf("嘉然于 %s 发布了新动态:\n-------------------------\n%s\n\n%s", date, dynamic.Msg, url)
	} else if dynamic.Desc.Type == 8 {
		msg = fmt.Sprintf("嘉然于 %s 发布了新视频:\n-------------------------\n%s\n", date, dynamic.Msg)
	} else if dynamic.Desc.Type == 1 {
		if dynamic.Desc.OriginType == 2 || dynamic.Desc.OriginType == 4 {
			url := fmt.Sprintf("http://t.bilibili.com/%d", dynamic.Desc.DynamicId)
			msg = fmt.Sprintf("嘉然于 %s 转发了 %s 的动态:\n%s\n-------------------------\n原动态:\n%s\n\n%s", date, dynamic.OriginName, dynamic.RepostMsg, dynamic.Msg, url)
		} else if dynamic.Desc.OriginType == 8 {
			msg = fmt.Sprintf("嘉然于 %s 转发了 %s 的视频:\n%s\n-------------------------\n原视频:\n%s", date, dynamic.OriginName, dynamic.RepostMsg, dynamic.Msg)
		}
	}

	sendMsg := SendMsg{GroupId: strconv.Itoa(globalGroupId), Message: msg}
	api := Api{Action: "send_group_msg", Params: sendMsg}
	data, _ := json.Marshal(&api)
	//fmt.Println("send msg:", string(data))
	err := apiClient.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Fatal("send fans msg err:", err)
		close(done)
		return
	}
}
