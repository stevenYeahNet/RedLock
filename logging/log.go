package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var logInputChan chan []byte
var logFile *os.File

func init()  {
	fmt.Println("log dir: ")
	logInputChan = make(chan []byte,1024)
	go func() {
		for{
			select {
			case data := <- logInputChan:
				if data == nil{
					return
				}
				writeLog(string(data))
			}
		}
	}()
}

func Log(msg string)  {
	funcAddr,file,line,ok := runtime.Caller(0)
	if ok{
		funcName := runtime.FuncForPC(funcAddr).Name()
		logInputChan <- []byte( fmt.Sprintf("[%s, %s:%d, %s()] %s\n", time.Now().String(), file, line, funcName, msg) )
	}else {
		logInputChan <- []byte( fmt.Sprintf("[%s] %s\n",time.Now().String(),msg) )
	}
	timeNow := time.Now().Format("2006-01-05 15:04:05")
	fmt.Println(timeNow + " " + msg)
}

func getogDir() string{
	if runtime.GOOS == "darwin" {
		return "~/logs"
	}else if runtime.GOOS == "windows"{
		return "D:\\logs"
	}else{
		path := "./"
		var flag = true
		if flag{
			return path
		}else {
			return "~/logs"
		}
	}
}
func getCurrentDirectory() (string,bool) {
	dir,err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil{
		fmt.Println("getdirectory err:",err.Error())
		return "",false
	}
	fmt.Println("dir: ",dir)
	return strings.Replace(dir,"\\","/",-1),true
}

func createLogFile()  {
	var err error
	readDir := getogDir() + "/" + time.Now().Format("2006-01-02")
	err = os.MkdirAll(readDir,0755)
	if err != nil{
		fmt.Println("makedir -p failed, ",readDir,err.Error())
		os.Exit(1)
	}
	logFile,err = os.OpenFile(readDir + "/access.log",os.O_WRONLY | os.O_CREATE | os.O_APPEND,0666)
	if err != nil{
		fmt.Println("create log file failed, ",err.Error())
		os.Exit(1)
	}
	logFile.WriteString(time.Now().String() + "Open log file!")
}

func writeLog(msg string) {
	info ,err := os.Stat(getogDir() + "/" + time.Now().Format("2006-01-02") + "/access.log")
	if err == nil{
		if info.IsDir(){
			fmt.Println("create log file failed, the file name is a dir")
			return
		}else{
			//路径存在，是一个文件，继续写
			logFile.WriteString(msg)
		}
	}else {
		//路径不存在
		if logFile !=nil{
			logFile.Close()
		}
		createLogFile()
		logFile.WriteString(msg)
	}
}

func Close()  {
	if logFile != nil{
		logFile.Close()
	}
	close(logInputChan)
}