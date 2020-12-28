package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/otiai10/copy"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const (
	youtubeGetChannelAPI  string = `https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=`
	youtubeGetPlaylistAPI string = `https://www.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails,status&playlistId=`
	youtubeKey            string = `&key=`
	youtubeGetVideo       string = `https://www.googleapis.com/youtube/v3/videos?part=snippet&id=`
)

var (
	downloadingQueue DownloadingQueue
	dbmux            sync.Mutex
)

type SettingFile struct {
	Dbpath        string   `json:"dbpath"`
	YoutubeToken  string   `json:"youtubetoken"`
	Channel       []string `json:"channel"`
	Downloadpath  string   `json:"downloadpath"`
	Youtubedlpath string   `json:"youtubedlpath"`
	Path          string   `json:"path"`
	LogPath       string   `json:"log"`
	MaxThread     int      `json:"max_thread"`
}
type VideoInfo struct {
	Id              string
	PublishedAtDate time.Time
	ChannelId       string
	ChannelTitle    string
	Title           string
	Description     string
	VideoId         string
	PrivacyStatus   string
	Downloaded      string
}

type downlaodFrame struct {
	videoId string
	path    string
}

type DownloadingQueue struct {
	data list.List
	mux  sync.Mutex
}
type downloadingQueueframe struct {
	id       string
	startT   time.Time
	comefrom string
}

func (d *DownloadingQueue) Add(id string, comefrom string) *list.Element {
	d.mux.Lock()
	it := d.data.PushFront(downloadingQueueframe{id: id, startT: time.Now(), comefrom: comefrom})
	d.mux.Unlock()
	color.HiGreen("add ", it.Value.(downloadingQueueframe).id)
	return it
}
func (d *DownloadingQueue) Remove(it *list.Element) {
	color.HiGreen("complete ", it.Value.(downloadingQueueframe).id)
	d.data.Remove(it)
}
func (d *DownloadingQueue) Print() {
	if d.data.Len() == 0 {
		return
	}
	color.HiGreen("Print downloading Queue")
	for i := d.data.Front(); i != nil; i = i.Next() {
		color.HiGreen(i.Value.(downloadingQueueframe).id + " from " + i.Value.(downloadingQueueframe).comefrom + " Keep " + time.Now().Sub(i.Value.(downloadingQueueframe).startT).String())
	}
}
func (d *DownloadingQueue) Thread() int {
	return d.data.Len()
}

func main() { //GREEN

	jsonFile, err := os.Open("setting.json")
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()
	jsonByteValue, _ := ioutil.ReadAll(jsonFile)
	var settingFile SettingFile
	json.Unmarshal(jsonByteValue, &settingFile)
	color.Green("success read json file\n")
	//fmt.Println(settingFile)

	f, err := os.OpenFile(settingFile.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("file open error : %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	settingFile.Youtubedlpath = settingFile.Youtubedlpath + " "

	db, err := sql.Open("mysql", settingFile.Dbpath)
	if err != nil {
		panic(err)

	}
	defer db.Close()

	color.Green("success connect DB\n")
	go func() {
		for {
			downloadingQueue.Print()
			time.Sleep(60 * time.Second)
		}

	}()

	go func() {
		for {
			downloadVideo(settingFile, db)
			time.Sleep(1 * time.Second)
		}
	}()
	go scannerChannel(settingFile, db)
	time.Sleep(10 * time.Second)
	go downloadStream(settingFile, db)

	for {

		time.Sleep(1 * time.Hour)
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

//----------download func-------------
func downloadVideo(setting SettingFile, db *sql.DB) {
	sqlcom := `select * from videoTable where DOWNLOADED != "true";`
	dbmux.Lock()
	row, err := db.Query(sqlcom)
	dbmux.Unlock()
	if err != nil {
		color.Red("Download Video scan db error: " + err.Error())
		log.Println("Download Video scan db error: " + err.Error())
		return
	}

	defer row.Close()

	if row == nil {
		color.Blue("Nothing can be download")
		return
	}
	downloadFrameList := list.New()
	for row.Next() {
		var id, publishedAtTime, channel, channelTitle, title, description, videoid, privacystatus, downloaded string
		err = row.Scan(&id, &publishedAtTime, &channel, &channelTitle, &title, &description, &videoid, &privacystatus, &downloaded)
		if err != nil {
			color.Red("Download Video get row error: " + err.Error())

			log.Println("Download Video get row error: " + err.Error())
		}
		//fmt.Println(id,publishedAtTime,channel,channelTitle,title,description,videoid,privacystatus,downloaded)
		publishedtime, _ := time.Parse("2006-01-02T15:04:05Z", publishedAtTime)

		path := setting.Downloadpath + channel + setting.Path + publishedtime.Format("200601") + setting.Path

		if _, err := os.Stat(path); os.IsNotExist(err) {
			// path/to/whatever does not exist
			os.MkdirAll(path, os.ModePerm)
		}
		downloadFrameList.PushBack(downlaodFrame{videoId: videoid, path: path})

		//errorCode := callYoutubeDL(setting.Youtubedlpath,videoid,path+`'%(title)s.%(ext)s'`)

	}
	var mux sync.Mutex

	var wg sync.WaitGroup
	for i := downloadFrameList.Front(); i != nil; i = i.Next() {
		for downloadingQueue.Thread() > setting.MaxThread {
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(1 * time.Second)

		wg.Add(1)
		go func() {
			mux.Lock()
			var nowGet downlaodFrame = i.Value.(downlaodFrame)

			mux.Unlock()
			bufferdir := "./" + nowGet.videoId + "/"
			os.MkdirAll(bufferdir, os.ModePerm)

			itt := downloadingQueue.Add(nowGet.videoId, "arrived")
			errorCode := callYoutubeDL(setting.Youtubedlpath, nowGet.videoId, bufferdir+`'%(title)s.%(ext)s'`)
			downloadingQueue.Remove(itt)
			err := copy.Copy(bufferdir, nowGet.path)
			if err != nil {
				color.Red("download video copy error: " + err.Error())
				log.Println("download video copy error: " + err.Error())
				return
			}
			color.Blue("copy from " + bufferdir + " to " + nowGet.path)
			os.RemoveAll(bufferdir)
			color.Blue("remove " + bufferdir)

			if errorCode == 0 {
				stmt, err := db.Prepare(`update videoTable set DOWNLOADED=? where VIDEOID=?`)
				if err != nil {
					color.Red("download video : " + err.Error())
					log.Println("download video : " + err.Error())
				}
				defer stmt.Close()
				dbmux.Lock()
				stmt.Exec("true", nowGet.videoId)
				dbmux.Unlock()
			} else {
				stmt, err := db.Prepare(`update videoTable set DOWNLOADED=? where VIDEOID=?`)
				if err != nil {
					color.Red("download video : " + err.Error())
					log.Println("download video : " + err.Error())
				}
				defer stmt.Close()
				dbmux.Lock()
				_, err = stmt.Exec(string(errorCode), nowGet.videoId)
				dbmux.Unlock()
				if err != nil {
					color.Red("download video : " + err.Error())
					log.Println("download video : " + err.Error())
				}
			}

			wg.Done()

		}()
	}
	wg.Wait()

}

//https://blog.csdn.net/benben_2015/article/details/99948369
func callYoutubeDL(ytdlpath string, id string, path string) int {

	commandParams := ` --abort-on-unavailable-fragment --write-info-json --all-subs  --output  `

	id = ` https://www.youtube.com/watch?v=` + id
	command := ytdlpath + " " + commandParams + path + id
	fmt.Println(command)
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Start()
	if err != nil {
		color.Red("cmd.Start failed with %s " + err.Error())
		log.Println("cmd.Start failed with %s " + err.Error())
		return -1
	}

	err = cmd.Wait()
	if err != nil {
		color.Red("cmd.Wait failed with %s " + err.Error())
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

func videoIdInList(inList *list.List, str string) *list.Element {
	for it := inList.Front(); it != nil; it = it.Next() {
		if it.Value.(VideoInfo).VideoId == str {
			return it
		}
	}
	return nil
}
