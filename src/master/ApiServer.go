package master

import (
	"crontab-golang/src/common"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"
)

//任务http接口
type ApiServer struct {
	httpServer *http.Server
}

//单例对象
var (
	G_apiServer *ApiServer
)

//保存任务接口
//POST
func handleJobSave(response http.ResponseWriter, request *http.Request) {
	var (
		err     error
		postjob string
		job     common.Job
		oldJob  *common.Job
		bytes   []byte
	)
	//1.解析post表单
	if err = request.ParseForm(); err != nil {
		goto ERR
	}
	//2.取表单中的JOB字段
	postjob = request.PostForm.Get("job")
	//3.反序列化Job
	if err = json.Unmarshal([]byte(postjob), &job); err != nil {
		goto ERR
	}
	//4.保存到etcd
	if oldJob, err = G_jobMgr.SaveJob(&job); err != nil {
		goto ERR
	}
	//5.返回正常应答{"error":0,"msg":"",data:{...}}
	if bytes, err = common.BuildResponse(0, "success", oldJob); err == nil {
		response.Write(bytes)
	}
	return
ERR:
	//6.返回异常应答
	if bytes, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		response.Write(bytes)
	}
}

//初始化服务
func InitApiServer() (err error) {
	var (
		mux        *http.ServeMux
		listener   net.Listener
		httpServer *http.Server
	)
	//配置路由
	mux = http.NewServeMux()
	mux.HandleFunc("/job/save", handleJobSave)

	//启动TCP监听
	if listener, err = net.Listen("tcp", ":"+strconv.Itoa(G_config.ApiPort)); err != nil {
		return
	}

	httpServer = &http.Server{
		ReadTimeout:  time.Duration(G_config.ApiReadTimeOut) * time.Millisecond,
		WriteTimeout: time.Duration(G_config.ApiWriteTimeOut) * time.Millisecond,
		Handler:      mux,
	}

	G_apiServer = &ApiServer{
		httpServer,
	}

	//启动了服务端
	go httpServer.Serve(listener)

	return
}
