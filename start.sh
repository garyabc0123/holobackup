

while true
do
    ps -ef | grep "holobackup" | grep -v "grep"
    if [ "$?" -eq 1 ]
        then
        ./holobackup #啟動應用，修改成自己的啟動應用腳本或命令
        echo "process has been restarted!"
    else
        echo "process already started!"
    fi
    sleep 10
done


