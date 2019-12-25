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

//删除任务接口
//POST /job/delete name = job1
func handleJobDelete(response http.ResponseWriter, request *http.Request) {
	var (
		err    error
		name   string
		oldJob *common.Job
		bytes  []byte
	)

	//1.解析post表单
	if err = request.ParseForm(); err != nil {
		goto ERR
	}

	//删除的任务名
	name = request.PostForm.Get("name")

	//删除任务
	if oldJob, err = G_jobMgr.DeleteJob(name); err != nil {
		goto ERR
	}
	//正常应答
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

//列举所有crontab任务
func handleJobList(response http.ResponseWriter, request *http.Request) {
	var (
		jobList []*common.Job
		err     error
		bytes   []byte
	)
	//获取任务列表
	if jobList, err = G_jobMgr.ListJob(); err != nil {
		goto ERR
	}

	//正常应答
	if bytes, err = common.BuildResponse(0, "success", jobList); err == nil {
		response.Write(bytes)
	}
	return

ERR:
	//返回异常应答
	if bytes, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		response.Write(bytes)
	}

}

//强制杀死某个任务
//POST /job/kill name = job1
func handleJobKill(response http.ResponseWriter, request *http.Request) {
	var (
		err   error
		name  string
		bytes []byte
	)
	//1.解析post表单
	if err = request.ParseForm(); err != nil {
		goto ERR
	}
	//要杀死的任务名
	name = request.Form.Get("name")

	//杀死任务
	if err = G_jobMgr.KillJob(name); err != nil {
		goto ERR
	}
	//正常应答
	if bytes, err = common.BuildResponse(0, "success", nil); err == nil {
		response.Write(bytes)
	}
	return

ERR:
	//返回异常应答
	if bytes, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		response.Write(bytes)
	}

}

//查询任务日志
func handleJobLog(response http.ResponseWriter, request *http.Request) {
	var (
		err        error
		name       string
		skipParam  string //从第几条开始
		limitParam string //返回多少条
		skip       int64
		limit      int64
		logArr     []*common.JobLog
		bytes      []byte
	)
	//解析GET参数
	if err = request.ParseForm(); err != nil {
		goto ERR
	}

	//获取请求参数 /job/log?name=job1&skip=0&limit=10
	name = request.Form.Get("name")
	skipParam = request.Form.Get("skip")
	limitParam = request.Form.Get("limit")
	if skip, err = strconv.ParseInt(skipParam, 10, 64); err != nil {
		skip = 0
	}
	if limit, err = strconv.ParseInt(limitParam, 10, 64); err != nil {
		limit = 20
	}

	if logArr, err = G_logMgr.ListLog(name, skip, limit); err != nil {
		goto ERR
	}

	//正常应答
	if bytes, err = common.BuildResponse(0, "success", logArr); err == nil {
		response.Write(bytes)
	}
	return

ERR:
	//返回异常应答
	if bytes, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		response.Write(bytes)
	}

}

// 获取健康worker节点列表
func handleWorkerList(resp http.ResponseWriter, req *http.Request) {
	var (
		workerArr []string
		err       error
		bytes     []byte
	)

	if workerArr, err = G_workerMgr.ListWorkers(); err != nil {
		goto ERR
	}

	// 正常应答
	if bytes, err = common.BuildResponse(0, "success", workerArr); err == nil {
		resp.Write(bytes)
	}
	return

ERR:
	if bytes, err = common.BuildResponse(-1, err.Error(), nil); err == nil {
		resp.Write(bytes)
	}
}

//初始化服务
func InitApiServer() (err error) {
	var (
		mux           *http.ServeMux
		listener      net.Listener
		httpServer    *http.Server
		staticDir     http.Dir
		staticHandler http.Handler
	)
	//配置路由
	mux = http.NewServeMux()
	mux.HandleFunc("/job/save", handleJobSave)
	mux.HandleFunc("/job/delete", handleJobDelete)
	mux.HandleFunc("/job/list", handleJobList)
	mux.HandleFunc("/job/kill", handleJobKill)
	mux.HandleFunc("/job/log", handleJobLog)
	mux.HandleFunc("/worker/list", handleWorkerList)

	//静态文件目录
	staticDir = http.Dir(G_config.Webroot)
	staticHandler = http.FileServer(staticDir)
	mux.Handle("/", http.StripPrefix("/", staticHandler))

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
