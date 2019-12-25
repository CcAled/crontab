package master

import (
	"context"
	"crontab-golang/src/common"
	"github.com/mongodb/mongo-go-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

//mongodb日志管理
type LogMgr struct {
	client        *mongo.Client
	logCollection *mongo.Collection
}

var (
	G_logMgr *LogMgr
)

func InitLogMgr() (err error) {
	var (
		client        *mongo.Client
		clientOptions *options.ClientOptions
	)
	clientOptions = options.Client().ApplyURI(G_config.MongodbUri).SetConnectTimeout(time.Duration(G_config.MongodbConnectTimeout) * time.Millisecond)
	if client, err = mongo.Connect(context.TODO(), clientOptions); err != nil {
		return
	}
	//选择db和collection
	G_logMgr = &LogMgr{
		client:        client,
		logCollection: client.Database("cron").Collection("log"),
	}
	return
}

//查询列表
func (logMgr *LogMgr) ListLog(name string, skip int64, limit int64) (logArr []*common.JobLog, err error) {
	var (
		filter  *common.JobLogFilter
		cursor  *mongo.Cursor
		logSort *common.SortLogByStartTime
		jobLog  *common.JobLog
		findOpt *options.FindOptions
	)
	//len(logArr)
	logArr = make([]*common.JobLog, 0)

	//过滤条件
	filter = &common.JobLogFilter{JobName: name}

	//按照任务开始时间倒排
	logSort = &common.SortLogByStartTime{SortOrder: -1}

	findOpt = &options.FindOptions{
		Sort:  logSort,
		Skip:  &skip,
		Limit: &limit,
	}

	//查询
	if cursor, err = logMgr.logCollection.Find(context.TODO(), filter, findOpt); err != nil {
		return
	}
	//延迟释放游标
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		jobLog = &common.JobLog{}
		//反序列化bson
		if err = cursor.Decode(jobLog); err != nil {
			continue
		}
		logArr = append(logArr, jobLog)
	}
	logSort = logSort

	return
}
