package worker

import (
	"context"
	"crontab-golang/src/common"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"time"
)

type JobMgr struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher
}

var (
	G_jobMgr *JobMgr
)

//初始化管理器
func InitJobMgr() (err error) {
	var (
		config  clientv3.Config
		client  *clientv3.Client
		kv      clientv3.KV
		lease   clientv3.Lease
		watcher clientv3.Watcher
	)

	//初始化配置
	config = clientv3.Config{
		Endpoints:   G_config.EtcdEndpoints,
		DialTimeout: time.Duration(G_config.EtcdDialTimeout) * time.Millisecond,
	}

	if client, err = clientv3.New(config); err != nil {
		return
	}

	//得到kv和lease的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)

	//赋值单例
	G_jobMgr = &JobMgr{
		client:  client,
		kv:      kv,
		lease:   lease,
		watcher: watcher,
	}
	//启动任务监听
	G_jobMgr.watchJobs()

	//启动监听killer
	G_jobMgr.watchKiller()
	return
}

//监听任务变化
func (jobMgr *JobMgr) watchJobs() (err error) {
	var (
		getResp           *clientv3.GetResponse
		kvpair            *mvccpb.KeyValue
		job               *common.Job
		watchStartRevison int64
		watchChan         clientv3.WatchChan
		watchResp         clientv3.WatchResponse
		watchEvent        *clientv3.Event
		jobName           string
		jobEvent          *common.JobEvent
	)
	//1.get当前目录下的所有任务，并且获知当前集群的revison
	if getResp, err = jobMgr.kv.Get(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithPrefix()); err != nil {
		return
	}
	//遍历当前的任务
	for _, kvpair = range getResp.Kvs {
		if job, err = common.UnpackJob(kvpair.Value); err == nil {
			jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
			//将这个job同步给scheduler(调度协程)
			G_scheduler.PushJobEvent(jobEvent)
		}
	}
	//2.从该revison向后监听变化事件
	//监听协程
	go func() {
		//从GET时刻开始监听变化
		watchStartRevison = getResp.Header.Revision + 1
		//监听目录的后续变化
		watchChan = jobMgr.watcher.Watch(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithRev(watchStartRevison), clientv3.WithPrefix())
		//处理监听事件
		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					if job, err = common.UnpackJob(watchEvent.Kv.Value); err != nil {
						continue
					}
					//构造一个event事件
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
				case mvccpb.DELETE:
					jobName = common.ExtractJobName(string(watchEvent.Kv.Key))
					job = &common.Job{
						Name: jobName,
					}
					//构造一个删除Event
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_DELETE, job)
				}
				//变化推送事件给schedule
				fmt.Println(*jobEvent)
				G_scheduler.PushJobEvent(jobEvent)
			}
		}

	}()

	return
}

//创建任务执行锁
func (jobMgr *JobMgr) CreateJobLock(jobName string) (jobLock *JobLock) {
	//返回一把锁
	jobLock = InitJobLock(jobName, jobMgr.kv, jobMgr.lease)
	return
}

//监听强杀任务通知
func (jobMgr *JobMgr) watchKiller() {
	var (
		watchChan  clientv3.WatchChan
		watchResp  clientv3.WatchResponse
		watchEvent *clientv3.Event
		jobEvent   *common.JobEvent
		jobName    string
		job        *common.Job
	)
	//监听/cron/killer目录
	//监听协程
	go func() {
		//监听/cron/killer目录的后续变化
		watchChan = jobMgr.watcher.Watch(context.TODO(), common.JOB_KILLER_DIR, clientv3.WithPrefix())
		//处理监听事件
		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT: //杀死任务事件
					jobName = common.ExtractKillerName(string(watchEvent.Kv.Key))
					job = &common.Job{
						Name: jobName,
					}
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_KILL, job)
					//推送事件给schedule
					G_scheduler.PushJobEvent(jobEvent)
				case mvccpb.DELETE: //killer标记过期，自动删除
				}

			}
		}

	}()
	return
}
