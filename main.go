package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/bitly/go-simplejson"
	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/otiai10/copy"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"log"
)

const (
	youtubeGetChannelAPI string = `https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=`
	youtubeGetPlaylistAPI string = `https://www.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails,status&playlistId=`
	youtubeKey			 string = `&key=`
	MaxThread = 2
	youtubeGetVideo		string = `https://www.googleapis.com/youtube/v3/videos?part=snippet&id=`
)
var (
	downloadingQueue DownloadingQueue
	dbmux	sync.Mutex
)
type SettingFile struct{
	Dbpath 			string 		`json:"dbpath"`
	YoutubeToken 	string 		`json:"youtubetoken"`
	Channel 		[]string 	`json:"channel"`
	Downloadpath	string		`json:"downloadpath"`
	Youtubedlpath 	string		`json:"youtubedlpath"`
	Path			string		`json:"path"`
	LogPath         string      `json:"log"`
}
type VideoInfo struct{
	Id				string
	PublishedAtDate time.Time
	ChannelId		string
	ChannelTitle	string
	Title			string
	Description		string
	VideoId			string
	PrivacyStatus	string
	Downloaded		string

}

type downlaodFrame struct{
	videoId		string
	path		string
}

type DownloadingQueue struct{
	data list.List
	mux sync.Mutex
}
type downloadingQueueframe struct {
	id	string
	startT time.Time
	comefrom string
}
func (d *DownloadingQueue)Add(id string,comefrom string)(*list.Element){
	d.mux.Lock()
	it := d.data.PushFront(downloadingQueueframe{id:id,startT: time.Now(),comefrom: comefrom})
	d.mux.Unlock()
	color.HiGreen("add ",it.Value.(downloadingQueueframe).id)
	return it
}
func (d *DownloadingQueue)Remove(it *list.Element){
	color.HiGreen("complete ",it.Value.(downloadingQueueframe).id)
	d.data.Remove(it)
}
func(d *DownloadingQueue)Print(){
	if d.data.Len() == 0{
		return
	}
	color.HiGreen("Print downloading Queue")
	for i := d.data.Front() ; i != nil ; i = i.Next(){
		color.HiGreen(i.Value.(downloadingQueueframe).id+" from "+i.Value.(downloadingQueueframe).comefrom+" Keep "+time.Now().Sub(i.Value.(downloadingQueueframe).startT).String())
	}
}
func(d *DownloadingQueue)Thread()(int){
	return d.data.Len()
}

func main() {  //GREEN
	
	jsonFile , err := os.Open("setting.json")
	if err != nil{
		panic(err)
	}
	defer jsonFile.Close()
	jsonByteValue,_ := ioutil.ReadAll(jsonFile)
	var settingFile SettingFile
	json.Unmarshal(jsonByteValue,&settingFile)
	color.Green("success read json file\n")
	//fmt.Println(settingFile)

	f,err := os.OpenFile(settingFile.LogPath,os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
    	log.Fatalf("file open error : %v", err)
    }
    defer f.Close()
    log.SetOutput(f)








	settingFile.Youtubedlpath = settingFile.Youtubedlpath + " "

	
	db , err := sql.Open("mysql",settingFile.Dbpath)
	if err != nil{
		panic(err)

	}
	defer db.Close()

	
	color.Green("success connect DB\n")
	go func(){
		for{
			downloadingQueue.Print()
			time.Sleep(60*time.Second)
		}

	}()

	go func(){
		for{
			downloadVideo(settingFile, db)
			time.Sleep(1*time.Second)
		}
	}()
	go scannerChannel(settingFile,db)
	time.Sleep(10*time.Second)
	go downloadStream(settingFile,db)

	for ;;{

		time.Sleep(1*time.Hour)
	}



}


//---------tools func----------------//
//https://golangcode.com/check-if-a-file-exists/
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
//https://golangcode.com/download-a-file-from-a-url/
func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}


//------------scanner arrived func ------------------
func scannerChannel(settingfile SettingFile,db *sql.DB){   //YELLOW
	var before time.Time
	UploadPlaylist := getUploadPlaylistLink(settingfile)
	for { //infinite loop
		if time.Now().Sub(before).Hours() < 2 {  //如果兩次距離小於1天 睡10秒再檢查一次
			time.Sleep(10*time.Second)
			continue
		}else{
			before = time.Now()
			for i := 0 ; i < len(UploadPlaylist) ; i++{
				scanUploadPlaylist(settingfile,UploadPlaylist[i],"" , db)
			}

			color.Green("complete scanner channel")
		}
	}
}
func getUploadPlaylistLink (settingfile SettingFile)([]string){
	var res  []string
	for i:=0 ; i < len(settingfile.Channel) ; i++{

		url := youtubeGetChannelAPI + settingfile.Channel[i] + youtubeKey + settingfile.YoutubeToken
		//color.Yellow("Now scanning "+ url)
		err := DownloadFile(settingfile.Channel[i] , url)
		if err != nil {
			color.Red(err.Error())
			log.Println(err.Error())
			break
		}

		jsonFile , err := os.Open(settingfile.Channel[i])
		if err != nil{
			color.Red(err.Error())
			log.Println(err.Error())
			break
		}
		time.Sleep(1*time.Millisecond)
		defer jsonFile.Close()
		jsonByteValue,_ := ioutil.ReadAll(jsonFile)
		json,err := simplejson.NewJson(jsonByteValue)
		if err != nil{
			color.Red("Get playlist error: "+ err.Error() )
			log.Println("Get playlist error: "+ err.Error())
			continue
		}
		upload := json.Get("items").GetIndex(0).Get("contentDetails").Get("relatedPlaylists").Get("uploads").MustString()
		color.Yellow(settingfile.Channel[i] + " -> " + upload)
		res = append(res,upload)
		//println(upload)
		os.Remove(settingfile.Channel[i])
	}
	return res
}
func scanUploadPlaylist(settingfile SettingFile,playlistLink string ,nextpage string,db *sql.DB)(){

	color.Yellow("checking :" + playlistLink + " - "+nextpage)
	url := youtubeGetPlaylistAPI + playlistLink + youtubeKey + settingfile.YoutubeToken + "&maxResults=50&pageToken=" + nextpage
	err := DownloadFile(playlistLink+nextpage +"a" , url)
	if err != nil {
		color.Red("Scanner Playlist error: " + err.Error())
		log.Println("Scanner Playlist error: " + err.Error())
		return
	}
	jsonFile , err := os.Open(playlistLink+nextpage+"a")
	if err != nil{
		color.Red("Scanner Playlist error: " + err.Error())
		log.Println("Scanner Playlist error: " + err.Error())
		return
	}
	defer jsonFile.Close()
	jsonByteValue,_ := ioutil.ReadAll(jsonFile)
	json,err := simplejson.NewJson(jsonByteValue)
	if err != nil{
		color.Red("Scanner Playlist error: " + err.Error())
		log.Println("Scanner Playlist error: " + err.Error())
		return
	}
	//fmt.Println(json.Get("items"))




	for i,_ := range(json.Get("items").MustArray()){
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
		rows,err := db.Query(sqlcom)
		if err != nil{
			color.Red("Scanner Playlist check db is exist: "+ err.Error())
			log.Println("Scanner Playlist check db is exist: "+ err.Error())

		}
		defer rows.Close()
		dbmux.Unlock()
		if !rows.Next(){
			stmt ,err := db.Prepare(`insert into videoTable(ID,PUBLISHEDATTIME,CHANNELID,CHANNELTITLE,TITLE,DESCRIPTION,VIDEOID,PRIVACYSTATUS,DOWNLOADED) values(?,?,?,?,?,?,?,?,?)`)

			if err != nil{
				color.Red("Scanner Playlist prepare db: " + err.Error())
				log.Println("Scanner Playlist prepare db: " + err.Error())

				return
			}
			defer stmt.Close()
			dbmux.Lock()
			_ , err =stmt.Exec(id,publishedDateString,channelId,channelTitle,title,description,videoId,privacyStatus,"false")
			if err != nil{
				color.Red("Scanner Playlist write db error: " + err.Error())
				log.Println("Scanner Playlist write db error: " + err.Error())
			}
			dbmux.Unlock()
			//fmt.Println(res)
		}

	}

	if data,ok :=json.CheckGet("nextPageToken"); ok{
		scanUploadPlaylist(settingfile,playlistLink,data.MustString(),db)
	}
	os.Remove(playlistLink+nextpage+"a")
}

//----------download func-------------
func downloadVideo(setting SettingFile,db *sql.DB){
	sqlcom :=`select * from videoTable where DOWNLOADED != "true";`
	dbmux.Lock()
	row,err := db.Query(sqlcom)
	dbmux.Unlock()
	if err != nil {
		color.Red("Download Video scan db error: " + err.Error())
		log.Println("Download Video scan db error: " + err.Error())
		return
	}

	defer row.Close()

	if row == nil{
		color.Blue("Nothing can be download")
		return
	}
	downloadFrameList := list.New()
	for row.Next(){
		var id,publishedAtTime,channel,channelTitle,title,description,videoid,privacystatus,downloaded string
		err = row.Scan(&id,&publishedAtTime,&channel,&channelTitle,&title,&description,&videoid,&privacystatus,&downloaded)
		if err!= nil{
			color.Red("Download Video get row error: " + err.Error())
			
			log.Println("Download Video get row error: " + err.Error())
		}
		//fmt.Println(id,publishedAtTime,channel,channelTitle,title,description,videoid,privacystatus,downloaded)
		publishedtime, _ := time.Parse("2006-01-02T15:04:05Z", publishedAtTime)

		path := setting.Downloadpath+channel+setting.Path+publishedtime.Format("200601")+setting.Path


		if _, err := os.Stat(path); os.IsNotExist(err) {
			// path/to/whatever does not exist
			os.MkdirAll(path, os.ModePerm)
		}
		downloadFrameList.PushBack(downlaodFrame{videoId:videoid , path:path})

		//errorCode := callYoutubeDL(setting.Youtubedlpath,videoid,path+`'%(title)s.%(ext)s'`)



	}
	var mux  sync.Mutex

	var wg 	 sync.WaitGroup
	//bufferdir := []string{`./cache02/`,`./cache03/`,`./cache04/`,`./cache05/`}
	for i:= 0 ; i< MaxThread; i++{
		for ;downloadingQueue.Thread() > MaxThread;{
			time.Sleep(1*time.Millisecond)
		}


		wg.Add(1)
		go func(){
			for downloadFrameList.Front() != downloadFrameList.Back(){
				mux.Lock()
				nowGet := downloadFrameList.Front()
				downloadFrameList.Remove(downloadFrameList.Front())
				mux.Unlock()
				bufferdir := "./" + nowGet.Value.(downlaodFrame).videoId + "/"
				os.MkdirAll(bufferdir,os.ModePerm)
				itt := downloadingQueue.Add(nowGet.Value.(downlaodFrame).videoId,"arrived")
				errorCode := callYoutubeDL(setting.Youtubedlpath,nowGet.Value.(downlaodFrame).videoId,bufferdir+`'%(title)s.%(ext)s'`)
				downloadingQueue.Remove(itt)
				err := copy.Copy(bufferdir , nowGet.Value.(downlaodFrame).path)
				if err != nil {
					color.Red("download video copy error: " + err.Error())
					log.Println("download video copy error: " + err.Error())
					return 
				}
				color.Blue("copy from " + bufferdir +" to "+nowGet.Value.(downlaodFrame).path)
				os.RemoveAll(bufferdir)
				color.Blue("remove " + bufferdir)

				if errorCode == 0 {
					stmt,err := db.Prepare(`update videoTable set DOWNLOADED=? where VIDEOID=?`)
					if err != nil {
						color.Red("download video : " + err.Error())
						log.Println("download video : " + err.Error())
					}
					defer stmt.Close()
					dbmux.Lock()
					stmt.Exec("true",nowGet.Value.(downlaodFrame).videoId)
					dbmux.Unlock()
				}else{
					stmt,err := db.Prepare(`update videoTable set DOWNLOADED=? where VIDEOID=?`)
					if err != nil {
						color.Red("download video : " + err.Error())
						log.Println("download video : " + err.Error())
					}
					defer stmt.Close()
					dbmux.Lock()
					_ , err = stmt.Exec(string(errorCode),nowGet.Value.(downlaodFrame).videoId)
					dbmux.Unlock()
					if err != nil {
						color.Red("download video : " + err.Error())
						log.Println("download video : " + err.Error())
					}
				}
			}
			wg.Done()

		}()
	}
	wg.Wait()

}
//https://blog.csdn.net/benben_2015/article/details/99948369
func  callYoutubeDL(ytdlpath string,id string,path string) int{

	commandParams := ` --abort-on-unavailable-fragment --write-info-json --all-subs  --output  `

	id = ` https://www.youtube.com/watch?v=`+id
	command := ytdlpath + " " + commandParams + path + id
	fmt.Println(command)
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Start()
	if err != nil {
		color.Red("cmd.Start failed with %s "+ err.Error())
		log.Println("cmd.Start failed with %s "+ err.Error())
		return -1
	}


	err = cmd.Wait()
	if  err != nil {
		color.Red("cmd.Wait failed with %s "+ err.Error())
		log.Println("cmd.Wait failed with %s " + err.Error())
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		} else {

			return -1
		}
	}


	return 0

}


//-----------download living video func----------

func downloadStream(file SettingFile,db *sql.DB){
	NowDownloadingStreamFrameList := list.New()
	var before time.Time
	for ;;{
		if time.Now().Sub(before).Seconds() < 10{
			time.Sleep(60*time.Second)
			continue
		}
		before = time.Now()
		var streamingListByHoloTVWebsite  []string

		queryDoc, err := goquery.NewDocument("https://schedule.hololive.tv/")
		if(err != nil){
			color.Red("download stream scan hololive.tv error: " + err.Error())
			log.Println("download stream scan hololive.tv error: " + err.Error())
		}
		queryDoc.Find(`a[style *= "border: 3px red solid"]`).Each(func(i int, selection *goquery.Selection){
			c,_:=selection.Attr("href")
			index := strings.Index(c,"=")
			c = c[index + 1:]
			streamingListByHoloTVWebsite = append(streamingListByHoloTVWebsite,c)

			color.Magenta("Find "+c+"i s streaming")
		})

		for i := 0 ; i < len(streamingListByHoloTVWebsite) ; i++ {

			if(videoIdInList(NowDownloadingStreamFrameList , streamingListByHoloTVWebsite[i]) != nil){

				continue
			}

			url := youtubeGetVideo + streamingListByHoloTVWebsite[i] + youtubeKey + file.YoutubeToken
			DownloadFile(streamingListByHoloTVWebsite[i] + "c",url )
			jsonFile , err := os.Open(streamingListByHoloTVWebsite[i] + "c")
			if err != nil{
				color.Red(err.Error())
				log.Println( err.Error())
				return
			}
			defer jsonFile.Close()
			jsonByteValue,_ := ioutil.ReadAll(jsonFile)
			json,err := simplejson.NewJson(jsonByteValue)
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
			newVideoInfo.PublishedAtDate,_ = time.Parse("2006-01-02T15:04:05Z", json.Get("items").GetIndex(0).Get("snippet").Get("publishedAt").MustString())
			newVideoInfo.Id =json.Get("items").GetIndex(0).Get("id").MustString()
			newVideoInfo.PrivacyStatus = "live"
			newVideoInfo.Downloaded = "false"
			NowDownloadingStreamFrameList.PushBack(newVideoInfo)
			go func(newVideoInfo VideoInfo , NowDownloadingStreamFrameList *list.List){

				bufferdir := "./" + newVideoInfo.VideoId + "/"
				os.MkdirAll(bufferdir,os.ModePerm)
				itt := downloadingQueue.Add(newVideoInfo.VideoId,"stream")
				errorCode := callYoutubeDL(file.Youtubedlpath,newVideoInfo.VideoId,bufferdir+`'%(title)s.stream.%(ext)s'`)
				downloadingQueue.Remove(itt)
				downloadPath := file.Downloadpath + newVideoInfo.ChannelId +file.Path+newVideoInfo.PublishedAtDate.Format("200601")+file.Path
				err = copy.Copy(bufferdir , downloadPath)
				if err != nil {
					color.Red("downlaod steam copy error: " + err.Error())
					log.Println("downlaod steam copy error: " + err.Error())
				}
				color.Magenta("copy from " + bufferdir +" to "+downloadPath)
				os.RemoveAll(bufferdir)
				color.Magenta("remove " + bufferdir)


				stmt,err := db.Prepare(`insert into videoTable(ID,PUBLISHEDATTIME,CHANNELID,CHANNELTITLE,TITLE,DESCRIPTION,VIDEOID,PRIVACYSTATUS,DOWNLOADED) values(?,?,?,?,?,?,?,?,?)`)
				if err != nil {
					fmt.Println(err)
				}
				defer stmt.Close()
				if errorCode == 0 {
					newVideoInfo.Downloaded = "live"
				}else{
					newVideoInfo.Downloaded = string(errorCode)

				}
				dbmux.Lock()
				stmt.Exec(newVideoInfo.Id,newVideoInfo.PublishedAtDate,newVideoInfo.ChannelId,newVideoInfo.ChannelTitle,newVideoInfo.Title,newVideoInfo.Description,newVideoInfo.VideoId,newVideoInfo.PrivacyStatus,newVideoInfo.Downloaded)
				dbmux.Unlock()
				re := videoIdInList(NowDownloadingStreamFrameList,newVideoInfo.VideoId)
				if re != nil{
					NowDownloadingStreamFrameList.Remove(re)
				}

			}(newVideoInfo,NowDownloadingStreamFrameList)

		}

	}
}
func videoIdInList(inList *list.List, str string)(*list.Element){
	for it := inList.Front() ; it != nil ; it = it.Next(){
		if it.Value.(VideoInfo).VideoId == str{
			return it
		}
	}
	return nil
}