package common

import (
	"encoding/json"
	"github.com/gorhill/cronexpr"
	"strings"
	"time"
)

//定时任务
type Job struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	CronExpr string `json:"cronExpr"`
}

//HTTP接口应答
type Response struct {
	Errno int         `json:"errno"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data"`
}

//变化事件
type JobEvent struct {
	EventType int //SAVE,DELETE
	Job       *Job
}

//任务调度计划
type JobSchedulePlan struct {
	Job      *Job
	Expr     *cronexpr.Expression
	NextTime time.Time
}

//任务执行状态
type JobExecuteInfo struct {
	Job      *Job
	PlanTime time.Time //理论上的调度时间
	RealTime time.Time //实际上的调度时间
}

//任务执行结果
type JobExecuteResult struct {
	ExecuteInfo *JobExecuteInfo //执行状态
	Output      []byte          //脚本输出
	Err         error           //脚本错误原因
	StartTime   time.Time       //启动时间
	EndTime     time.Time       //结束时间
}

//应答方法
func BuildResponse(errno int, msg string, data interface{}) (resp []byte, err error) {
	//1.定义一个response对象
	var (
		response Response
	)

	response.Errno = errno
	response.Msg = msg
	response.Data = data

	//2.序列化json
	resp, err = json.Marshal(response)
	return
}

//反序列化Job
func UnpackJob(value []byte) (ret *Job, err error) {
	var (
		job *Job
	)

	job = &Job{}
	if err = json.Unmarshal(value, job); err != nil {
		return
	}
	ret = job
	return
}

//从key中提取任务名
func ExtractJobName(jobKey string) string {
	return strings.TrimPrefix(jobKey, JOB_SAVE_DIR)
}

//变化事件构造
func BuildJobEvent(eventType int, job *Job) (jobEvent *JobEvent) {
	return &JobEvent{
		EventType: eventType,
		Job:       job,
	}
}

//构造任务执行计划
func BuildJobSchedulePlan(job *Job) (jobSchedulePlan *JobSchedulePlan, err error) {
	var (
		expr *cronexpr.Expression
	)
	//解析job的cron表达式
	if expr, err = cronexpr.Parse(job.CronExpr); err != nil {
		return
	}
	//生成调度计划
	jobSchedulePlan = &JobSchedulePlan{
		Job:      job,
		Expr:     expr,
		NextTime: expr.Next(time.Now()),
	}
	return
}

//构造执行信息函数
func BuildJobExecuteInfo(jobschedulePlan *JobSchedulePlan) (jobExecuteInfo *JobExecuteInfo) {
	jobExecuteInfo = &JobExecuteInfo{
		Job:      jobschedulePlan.Job,
		PlanTime: jobschedulePlan.NextTime,
		RealTime: time.Now(),
	}
	return
}
