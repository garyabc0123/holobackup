package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otiai10/copy"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)
//https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=UCXTpFs_3PqI41qX2d9tL2Rw&key=AIzaSyD57ADXNxQNHahPu6mRQPL0Wwl3m0cgqtU
//https://www.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails,status&playlistId=UUXTpFs_3PqI41qX2d9tL2Rw&key=AIzaSyD57ADXNxQNHahPu6mRQPL0Wwl3m0cgqtU&maxResults=50
//youtube api token : AIzaSyD57ADXNxQNHahPu6mRQPL0Wwl3m0cgqtU
//https://www.googleapis.com/youtube/v3/liveBroadcasts?part=contentDetails&broadcastStatus=all&id=UCqm3BQLlJfvkTsX_hvm0UmA&key=AIzaSyD57ADXNxQNHahPu6mRQPL0Wwl3m0cgqtU
//https://www.googleapis.com/youtube/v3/liveChat/messages?liveChatId=sObG4XdTtEc
//https://www.googleapis.com/youtube/v3/activities?part=snippet%2CcontentDetails&part=id&channelId=UCqm3BQLlJfvkTsX_hvm0UmA&maxResults=25&key=AIzaSyD57ADXNxQNHahPu6mRQPL0Wwl3m0cgqtU
//https://www.googleapis.com/youtube/v3/search?part=snippet&eventType=live&type=video&channelId=UCqm3BQLlJfvkTsX_hvm0UmA&key=AIzaSyD57ADXNxQNHahPu6mRQPL0Wwl3m0cgqtU
var(
	youtubeGetChannelAPI string = `https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=`
	youtubeGetPlaylistAPI string = `https://www.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails,status&playlistId=`
	youtubeKey			 string = `&key=`
)
type SettingFile struct{
	Dbpath 			string 		`json:"dbpath"`
	YoutubeToken 	string 		`json:"youtubetoken"`
	Channel 		[]string 	`json:"channel"`
	Downloadpath	string		`json:"downloadpath"`
	Youtubedlpath 	string		`json:"youtubedlpath"`
	Path			string		`json:"path"`
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


	var ifCreateTable bool
	if fileExists(settingFile.Dbpath){
		ifCreateTable = false
	}else{
		ifCreateTable = true
	}
	db , err := sql.Open("sqlite3",settingFile.Dbpath)
	if err != nil{
		panic(err)
	}
	defer db.Close()

	if ifCreateTable{
		sqlstmt := `CREATE TABLE videoTable(
		ID              TEXT,
		PUBLISHEDATTIME TIME,
		CHANNELID		TEXT,
		CHANNELTITLE    TEXT,
		TITLE			TEXT,
		DESCRIPTION     TEXT,
		VIDEOID			TEXT PRIMARY KEY,
		PRIVACYSTATUS	TEXT,
		DOWNLOADED		TEXT
		);
		`
		_,err := db.Exec(sqlstmt)
		if err != nil{
			fmt.Println(err)
		}
	}
	color.Green("success connect DB\n")

	go scannerChannel(settingFile,db)

	for ;;{
		downloadVideo(settingFile, db)
		time.Sleep(1*time.Second)
	}

}

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

func downloadVideo(setting SettingFile,db *sql.DB){
	sqlcom :=`select * from videoTable where DOWNLOADED != "true";`
	row,err := db.Query(sqlcom)
	if err != nil {
		fmt.Println(err)
	}

	defer row.Close()


	downloadFrameList := list.New()
	for row.Next(){
		var id,publishedAtTime,channel,channelTitle,title,description,videoid,privacystatus,downloaded string
		err = row.Scan(&id,&publishedAtTime,&channel,&channelTitle,&title,&description,&videoid,&privacystatus,&downloaded)
		if err!= nil{
			color.Red(err.Error())
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
	bufferdir := []string{`./cache02/`,`./cache03/`,`./cache04/`,`./cache05/`}
	for i:= 0 ; i< 4; i++{
		wg.Add(1)
		go func(GoId int){
			for downloadFrameList.Front() != downloadFrameList.Back(){
				mux.Lock()
				nowGet := downloadFrameList.Front()
				downloadFrameList.Remove(downloadFrameList.Front())
				mux.Unlock()

				errorCode := callYoutubeDL(setting.Youtubedlpath,nowGet.Value.(downlaodFrame).videoId,bufferdir[GoId]+`'%(title)s.%(ext)s'`)

				err := copy.Copy(bufferdir[GoId] , nowGet.Value.(downlaodFrame).path)
				if err != nil {
					color.Red(err.Error())
				}
				color.Blue("copy from " , bufferdir[GoId] ," to ",nowGet.Value.(downlaodFrame).path)
				os.RemoveAll(bufferdir[GoId])
				color.Blue("remove " , bufferdir[GoId] )

				if errorCode == 0 {
					stmt,err := db.Prepare(`update videoTable set DOWNLOADED=? where VIDEOID=?`)
					if err != nil {
						fmt.Println(err)
					}
					stmt.Exec("true",nowGet.Value.(downlaodFrame).videoId)
				}else{
					stmt,err := db.Prepare(`update videoTable set DOWNLOADED=? where VIDEOID=?`)
					if err != nil {
						fmt.Println(err)
					}
					stmt.Exec(string(errorCode),nowGet.Value.(downlaodFrame).videoId)
				}
			}
			wg.Done()

		}(i)
	}
	wg.Wait()

}

func downloadStream(){

}
func scannerChannel(settingfile SettingFile,db *sql.DB){   //YELLOW
	var before time.Time
	UploadPlaylist := getUploadPlaylistLink(settingfile)
	for { //infinite loop
		if time.Now().Sub(before).Hours() < 24 {  //如果兩次距離小於1天 睡10秒再檢查一次
			time.Sleep(10*time.Second)
		}else{
			before = time.Now()
			for i := 0 ; i < len(UploadPlaylist) ; i++{
				scanUploadPlaylist(settingfile,UploadPlaylist[i],"" , db)
			}

			color.Green("complete")
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
			break
		}

		jsonFile , err := os.Open(settingfile.Channel[i])
		if err != nil{
			color.Red(err.Error())
			break
		}
		defer jsonFile.Close()
		jsonByteValue,_ := ioutil.ReadAll(jsonFile)
		json,_ := simplejson.NewJson(jsonByteValue)
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
	err := DownloadFile(playlistLink+nextpage , url)
	if err != nil {
		color.Red(err.Error())
		return
	}
	jsonFile , err := os.Open(playlistLink+nextpage)
	if err != nil{
		color.Red(err.Error())
		return
	}
	defer jsonFile.Close()
	jsonByteValue,_ := ioutil.ReadAll(jsonFile)
	json,_ := simplejson.NewJson(jsonByteValue)
	//fmt.Println(json.Get("items"))

	stmt ,err := db.Prepare(`insert into videoTable(ID,PUBLISHEDATTIME,CHANNELID,CHANNELTITLE,TITLE,DESCRIPTION,VIDEOID,PRIVACYSTATUS,DOWNLOADED) values(?,?,?,?,?,?,?,?,?)`)
	if err != nil{
		fmt.Println(err)
	}
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

		sqlcom := `select * from videoTable where VIDEOID  =  "` + videoId + `";`
		rows,err := db.Query(sqlcom)
		if err != nil{
			color.Red(err.Error())

		}
		if !rows.Next(){
			res , err :=stmt.Exec(id,publishedDateString,channelId,channelTitle,title,description,videoId,privacyStatus,"false")
			fmt.Println(err)
			fmt.Println(res)
		}
		defer rows.Close()
	}

	if data,ok :=json.CheckGet("nextPageToken"); ok{
		scanUploadPlaylist(settingfile,playlistLink,data.MustString(),db)
	}
	os.Remove(playlistLink+nextpage)
}
//https://blog.csdn.net/benben_2015/article/details/99948369

func  callYoutubeDL(ytdlpath string,id string,path string) int{

	commandParams := ` --abort-on-unavailable-fragment --write-info-json --all-subs  --output  `

	id = ` https://www.youtube.com/watch?v=`+id
	command := ytdlpath + " " + commandParams + path + id
	fmt.Println(command)
	cmd := exec.Command("bash", "-c", command)


	var stdout, stderr []byte
	var errStdout, errStderr error
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	err := cmd.Start()
	if err != nil {
		color.Red("cmd.Start failed with %s\n", err)
		return -1
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		stdout, errStdout = copyAndCapture(os.Stdout, stdoutIn)
		wg.Done()
	}()

	stderr, errStderr = copyAndCapture(os.Stderr, stderrIn)
	wg.Wait()
	err = cmd.Wait()
	if  err != nil {
		color.Red("cmd.Wait failed with %s\n", err)
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
	if errStdout != nil || errStderr != nil {
		color.Red("failed to capture stdout or stderr\n")
		return -1
	}
	outStr, errStr := string(stdout), string(stderr)
	color.Red("out: %s\n err: %s\n", outStr, errStr)
	return 0

}



func copyAndCapture(w io.Writer, r io.Reader) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024, 1024)
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			d := buf[:n]
			out = append(out, d...)
			_, err := w.Write(d)
			if err != nil {
				return out, err
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return out, err
		}
	}
}