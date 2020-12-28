package main

import (
	"database/sql"
	"github.com/bitly/go-simplejson"
	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"log"
	"os"
	"time"
)

//------------scanner arrived func ------------------
func scannerChannel(settingfile SettingFile, db *sql.DB) { //YELLOW
	var before time.Time
	UploadPlaylist := getUploadPlaylistLink(settingfile)
	for { //infinite loop
		if time.Now().Sub(before).Hours() < 2 { //如果兩次距離小於1天 睡10秒再檢查一次
			time.Sleep(10 * time.Second)
			continue
		} else {
			before = time.Now()
			for i := 0; i < len(UploadPlaylist); i++ {
				scanUploadPlaylist(settingfile, UploadPlaylist[i], "", db)
			}

			color.Green("complete scanner channel")
		}
	}
}
func getUploadPlaylistLink(settingfile SettingFile) []string {
	var res []string
	for i := 0; i < len(settingfile.Channel); i++ {

		url := youtubeGetChannelAPI + settingfile.Channel[i] + youtubeKey + settingfile.YoutubeToken
		//color.Yellow("Now scanning "+ url)
		err := DownloadFile(settingfile.Channel[i], url)
		if err != nil {
			color.Red(err.Error())
			log.Println(err.Error())
			break
		}

		jsonFile, err := os.Open(settingfile.Channel[i])
		if err != nil {
			color.Red(err.Error())
			log.Println(err.Error())
			break
		}
		time.Sleep(1 * time.Millisecond)
		defer jsonFile.Close()
		jsonByteValue, _ := ioutil.ReadAll(jsonFile)
		json, err := simplejson.NewJson(jsonByteValue)
		if err != nil {
			color.Red("Get playlist error: " + err.Error())
			log.Println("Get playlist error: " + err.Error())
			continue
		}
		upload := json.Get("items").GetIndex(0).Get("contentDetails").Get("relatedPlaylists").Get("uploads").MustString()
		color.Yellow(settingfile.Channel[i] + " -> " + upload)
		res = append(res, upload)
		//println(upload)
		os.Remove(settingfile.Channel[i])
	}
	return res
}
func scanUploadPlaylist(settingfile SettingFile, playlistLink string, nextpage string, db *sql.DB) {

	color.Yellow("checking :" + playlistLink + " - " + nextpage)
	url := youtubeGetPlaylistAPI + playlistLink + youtubeKey + settingfile.YoutubeToken + "&maxResults=50&pageToken=" + nextpage
	err := DownloadFile(playlistLink+nextpage+"a", url)
	if err != nil {
		color.Red("Scanner Playlist error: " + err.Error())
		log.Println("Scanner Playlist error: " + err.Error())
		return
	}
	jsonFile, err := os.Open(playlistLink + nextpage + "a")
	if err != nil {
		color.Red("Scanner Playlist error: " + err.Error())
		log.Println("Scanner Playlist error: " + err.Error())
		return
	}
	defer jsonFile.Close()
	jsonByteValue, _ := ioutil.ReadAll(jsonFile)
	json, err := simplejson.NewJson(jsonByteValue)
	if err != nil {
		color.Red("Scanner Playlist error: " + err.Error())
		log.Println("Scanner Playlist error: " + err.Error())
		return
	}
	//fmt.Println(json.Get("items"))

	for i, _ := range json.Get("items").MustArray() {
		videoId := json.Get("items").GetIndex(i).Get("snippet").Get("resourceId").Get("videoId").MustString()
		id := json.Get("items").GetIndex(i).Get("id").MustString()
		publishedDateString := json.Get("items").GetIndex(i).Get("snippet").Get("publishedAt").MustString()
		//publishedDate,err := time.Parse("2006-01-02T15:04:05Z",publishedDateString)
		channelId := json.Get("items").GetIndex(i).Get("snippet").Get("channelId").MustString()
		channelTitle := json.Get("items").GetIndex(i).Get("snippet").Get("channelTitle").MustString()
		title := json.Get("items").GetIndex(i).Get("snippet").Get("title").MustString()
		description := json.Get("items").GetIndex(i).Get("snippet").Get("description").MustString()
		privacyStatus := json.Get("items").GetIndex(i).Get("status").Get("privacyStatus").MustString()
		dbmux.Lock()
		sqlcom := `select * from videoTable where VIDEOID  =  "` + videoId + `";`
		rows, err := db.Query(sqlcom)
		if err != nil {
			color.Red("Scanner Playlist check db is exist: " + err.Error())
			log.Println("Scanner Playlist check db is exist: " + err.Error())

		}
		defer rows.Close()
		dbmux.Unlock()
		if !rows.Next() {
			stmt, err := db.Prepare(`insert into videoTable(ID,PUBLISHEDATTIME,CHANNELID,CHANNELTITLE,TITLE,DESCRIPTION,VIDEOID,PRIVACYSTATUS,DOWNLOADED) values(?,?,?,?,?,?,?,?,?)`)

			if err != nil {
				color.Red("Scanner Playlist prepare db: " + err.Error())
				log.Println("Scanner Playlist prepare db: " + err.Error())

				return
			}
			defer stmt.Close()
			dbmux.Lock()
			_, err = stmt.Exec(id, publishedDateString, channelId, channelTitle, title, description, videoId, privacyStatus, "false")
			if err != nil {
				color.Red("Scanner Playlist write db error: " + err.Error())
				log.Println("Scanner Playlist write db error: " + err.Error())
			}
			dbmux.Unlock()
			//fmt.Println(res)
		}

	}

	if data, ok := json.CheckGet("nextPageToken"); ok {
		scanUploadPlaylist(settingfile, playlistLink, data.MustString(), db)
	}
	os.Remove(playlistLink + nextpage + "a")
}
