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

var agent = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Mobile Safari/537.36"
var tiktokUrl = "https://www.iesdouyin.com/web/api/v2/aweme/post/?sec_uid=MS4wLjABAAAA5ZrIrbgva_HMeHuNn64goOD2XYnk4ItSypgRHlbSh1c&count=6&max_cursor=0&aid=1128&_signature=RdSP6AAAJP5XQDsqrOPQkUXUj.&dytk="
var lastTiktok *AwemeInfo
var tiktokSendFlag = false

type AweCover struct {
	UrlList []string `json:"url_list"`
}

type AweVideo struct {
	Cover AweCover `json:"cover"`
}

type AwemeInfo struct {
	AwemeId string   `json:"aweme_id"`
	Desc    string   `json:"desc"`
	Video   AweVideo `json:"video"`
}

type TikTokRsp struct {
	Code      int         `json:"status_code"`
	AwemeList []AwemeInfo `json:"aweme_list"`
}

func tiktok_loop() {
	tiktokSendFlag = false
	tiktok()
	tiktokSendFlag = true
	ticker := time.NewTicker(time.Second * 90)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			tiktok()
		case <-interrupt:
			log.Println("interrupt")
			return
		}
	}
}

func tiktok() {
	fmt.Println("tiktok loop")
	client := http.Client{}
	request, _ := http.NewRequest("GET", tiktokUrl, nil)

	request.Header.Set("user-agent", agent)
	resp, err := client.Do(request)
	if err != nil {
		fmt.Println("tiktok get error, err:", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("tiktok read err:", err)
		return
	}

	tiktokRsp := TikTokRsp{}
	err = json.Unmarshal([]byte(body), &tiktokRsp)
	if err != nil {
		fmt.Printf("decode tiktok rsp json err:%s\n", err)
		return
	}
	if tiktokRsp.Code != 0 {
		fmt.Printf("tiktok rsp code:%d\n", tiktokRsp.Code)
		return
	}

	if len(tiktokRsp.AwemeList) < 1 {
		fmt.Printf("tiktok rsp len is 0\n")
		return
	}

	aweid := tiktokRsp.AwemeList[0].AwemeId
	if lastTiktok == nil || aweid != lastTiktok.AwemeId {
		lastTiktok = &(tiktokRsp.AwemeList[0])
		if tiktokSendFlag {
			send_tiktok()
		}
	}
}

func send_tiktok() {
	url := fmt.Sprintf("https://www.douyin.com/video/%s", lastTiktok.AwemeId)
	pic := fmt.Sprintf("%s\n[CQ:image,file=%s]", lastTiktok.Desc, lastTiktok.Video.Cover.UrlList[0])
	msg := fmt.Sprintf("嘉然抖音发布了新视频:\n-------------------------\n%s\n\n%s", pic, url)

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
