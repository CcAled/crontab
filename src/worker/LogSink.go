package worker

import (
	"context"
	"crontab-golang/src/common"
	"github.com/mongodb/mongo-go-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

//mongoDB存储日志
type LogSink struct {
	client         *mongo.Client
	logCollection  *mongo.Collection
	logChan        chan *common.JobLog
	autoCommitChan chan *common.LogBatch
}

var (
	G_logSink *LogSink
)

func InitLogSink() (err error) {
	var (
		client        *mongo.Client
		clientOptions *options.ClientOptions
	)
	clientOptions = options.Client().ApplyURI(G_config.MongodbUri).SetConnectTimeout(time.Duration(G_config.MongodbConnectTimeout) * time.Millisecond)
	if client, err = mongo.Connect(context.TODO(), clientOptions); err != nil {
		return
	}
	//选择db和collection
	G_logSink = &LogSink{
		client:         client,
		logCollection:  client.Database("cron").Collection("log"),
		logChan:        make(chan *common.JobLog, 1000),
		autoCommitChan: make(chan *common.LogBatch, 1000),
	}

	//启动mongodb处理协程
	go G_logSink.writeLoop()
	return
}

//批量写入日志
func (logSink *LogSink) saveLogs(batch *common.LogBatch) {
	logSink.logCollection.InsertMany(context.TODO(), batch.Logs)
}

//日志存储协程
func (logSink *LogSink) writeLoop() {
	var (
		log          *common.JobLog
		logBatch     *common.LogBatch
		commitTimer  *time.Timer
		timeoutBatch *common.LogBatch
	)
	for {
		select {
		case log = <-logSink.logChan:
			//把log写入mongodb中,批量插入，防止网络往返时间过长
			if logBatch == nil {
				logBatch = &common.LogBatch{}
				//让这个批次超时自动提交
				time.AfterFunc(time.Duration(G_config.JobLogCommitTimeout)*time.Millisecond, func(batch *common.LogBatch) func() {
					return func() {
						//发出超时通知，不直接提交batch，防止发生并发操作
						logSink.autoCommitChan <- batch
					}
				}(logBatch))
			}
			//把新日志追加到批次中
			logBatch.Logs = append(logBatch.Logs, log)

			//如果批次慢了，立即发送
			if len(logBatch.Logs) > G_config.JobLogBatchSize {
				//发送日志
				logSink.saveLogs(logBatch)
				//清空logbatch
				logBatch = nil
				//取消定时器
				commitTimer.Stop()
			}
		case timeoutBatch = <-logSink.autoCommitChan: //过期的批次
			//判断过期批次是否仍旧是当前批次（否则会有bug：当1秒过期时提交并且取消定时器时）
			if timeoutBatch != logBatch {
				continue //跳过已经被提交的批次
			}
			//把批次写入mongo
			logSink.saveLogs(timeoutBatch)
			//清空logbatch
			logBatch = nil
		}
	}
}

//发送日志api
func (logSink *LogSink) Append(jobLog *common.JobLog) {
	select {
	case logSink.logChan <- jobLog:
	default:
		//队列慢了丢弃
	}
}
