package main

import (
	"container/list"
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/bitly/go-simplejson"
	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/otiai10/copy"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

func downloadStream(file SettingFile, db *sql.DB) {
	NowDownloadingStreamFrameList := list.New()
	var before time.Time
	for {
		if time.Now().Sub(before).Seconds() < 10 {
			time.Sleep(60 * time.Second)
			continue
		}
		before = time.Now()
		var streamingListByHoloTVWebsite []string

		queryDoc, err := goquery.NewDocument("https://schedule.hololive.tv/")
		if err != nil {
			color.Red("download stream scan hololive.tv error: " + err.Error())
			log.Println("download stream scan hololive.tv error: " + err.Error())
			continue
		}
		queryDoc.Find(`a[style *= "border: 3px red solid"]`).Each(func(i int, selection *goquery.Selection) {
			c, _ := selection.Attr("href")
			index := strings.Index(c, "=")
			c = c[index+1:]
			streamingListByHoloTVWebsite = append(streamingListByHoloTVWebsite, c)

			color.Magenta("Find " + c + "i s streaming")
		})

		for i := 0; i < len(streamingListByHoloTVWebsite); i++ {

			if videoIdInList(NowDownloadingStreamFrameList, streamingListByHoloTVWebsite[i]) != nil {

				continue
			}

			url := youtubeGetVideo + streamingListByHoloTVWebsite[i] + youtubeKey + file.YoutubeToken
			DownloadFile(streamingListByHoloTVWebsite[i]+"c", url)
			jsonFile, err := os.Open(streamingListByHoloTVWebsite[i] + "c")
			if err != nil {
				color.Red(err.Error())
				log.Println(err.Error())
				return
			}
			defer jsonFile.Close()
			jsonByteValue, _ := ioutil.ReadAll(jsonFile)
			json, err := simplejson.NewJson(jsonByteValue)
			if err != nil {
				color.Red("download stream err: " + err.Error())
				log.Println("download stream err: " + err.Error())
			}
			os.Remove(streamingListByHoloTVWebsite[i] + "c")
			var newVideoInfo VideoInfo
			newVideoInfo.VideoId = streamingListByHoloTVWebsite[i]
			newVideoInfo.Title = json.Get("items").GetIndex(0).Get("snippet").Get("title").MustString()
			newVideoInfo.Description = json.Get("items").GetIndex(0).Get("snippet").Get("description").MustString()
			newVideoInfo.ChannelTitle = json.Get("items").GetIndex(0).Get("snippet").Get("channelTitle").MustString()
			newVideoInfo.ChannelId = json.Get("items").GetIndex(0).Get("snippet").Get("channelId").MustString()
			newVideoInfo.PublishedAtDate, _ = time.Parse("2006-01-02T15:04:05Z", json.Get("items").GetIndex(0).Get("snippet").Get("publishedAt").MustString())
			newVideoInfo.Id = json.Get("items").GetIndex(0).Get("id").MustString()
			newVideoInfo.PrivacyStatus = "live"
			newVideoInfo.Downloaded = "false"
			var ifNeedToDownload bool = false
			for it := 0; it < len(file.Channel); it++ {
				if newVideoInfo.ChannelId == file.Channel[it] {
					ifNeedToDownload = true
					break
				}
			}
			if !ifNeedToDownload {
				continue
			}
			NowDownloadingStreamFrameList.PushBack(newVideoInfo)
			go func(newVideoInfo VideoInfo, NowDownloadingStreamFrameList *list.List) {

				bufferdir := "./" + newVideoInfo.VideoId + "/"
				os.MkdirAll(bufferdir, os.ModePerm)
				itt := downloadingQueue.Add(newVideoInfo.VideoId, "stream")
				errorCode := callYoutubeDL(file.Youtubedlpath, newVideoInfo.VideoId, bufferdir+`'%(title)s.stream.%(ext)s'`)
				downloadingQueue.Remove(itt)
				downloadPath := file.Downloadpath + newVideoInfo.ChannelId + file.Path + newVideoInfo.PublishedAtDate.Format("200601") + file.Path
				err = copy.Copy(bufferdir, downloadPath)
				if err != nil {
					color.Red("downlaod steam copy error: " + err.Error())
					log.Println("downlaod steam copy error: " + err.Error())
				}
				color.Magenta("copy from " + bufferdir + " to " + downloadPath)
				os.RemoveAll(bufferdir)
				color.Magenta("remove " + bufferdir)

				stmt, err := db.Prepare(`insert into videoTable(ID,PUBLISHEDATTIME,CHANNELID,CHANNELTITLE,TITLE,DESCRIPTION,VIDEOID,PRIVACYSTATUS,DOWNLOADED) values(?,?,?,?,?,?,?,?,?)`)
				if err != nil {
					fmt.Println(err)
				}
				defer stmt.Close()
				if errorCode == 0 {
					newVideoInfo.Downloaded = "live"
				} else {
					newVideoInfo.Downloaded = string(errorCode)

				}
				dbmux.Lock()
				stmt.Exec(newVideoInfo.Id, newVideoInfo.PublishedAtDate, newVideoInfo.ChannelId, newVideoInfo.ChannelTitle, newVideoInfo.Title, newVideoInfo.Description, newVideoInfo.VideoId, newVideoInfo.PrivacyStatus, newVideoInfo.Downloaded)
				dbmux.Unlock()
				re := videoIdInList(NowDownloadingStreamFrameList, newVideoInfo.VideoId)
				if re != nil {
					NowDownloadingStreamFrameList.Remove(re)
				}

			}(newVideoInfo, NowDownloadingStreamFrameList)

		}

	}
}
