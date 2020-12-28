APP=holobackup

build:
	go clean
	go build -o ${APP} main.go scanYoutubeChannel.go HoloStream.go

clean:
	go clean


