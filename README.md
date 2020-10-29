# holobackup

A tool help you backup youtube channel.

## function
* scan Youtube API
* auto download file
* write metadata to MariaDB
* auto download live stream before video be delete

##### Next

* Web GUI

## Use
install 

[Go Offical Download Link](https://golang.org/dl/)

```shell=
sudo apt install golang python3 git ffmpeg
sudo python3 pip install youtube-dl
go mod download

```
and editor setting.json

## update log

### v0.2

* SQLite -> MariaDB
* limit download archived thread
* record stream video
* add go mod
* output error log file
* stdout simplify