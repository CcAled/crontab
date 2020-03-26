# 分布式crontab

## 部署方式
修改master.json和worker.json的etcd和mongodb的地址，并将master.json和worker.json放在相应的工作目录下

## 主要框架
/master<br>
/worker<br>
/common<br>

### 架构功能
1.利用etcd同步全量任务列表到所有worker节点<br>
2.每个worker独立调度全量任务，无需与master产生直接RPC<br>
3.各个worker利用分布式锁抢占，解决并发调度相同任务的问题<br>

### master功能
1.给web后台提供http API,用于管理job<br>
2.web后台前端页面为bootstrap + jquery<br>

### worker功能
1.从etcd中把job同步到内存<br>
2.实现调度模块，基于cron表达式调度N个job<br>
3.实现执行模块，并发的执行多个job<br>
4.对job分布式锁，防止集群并发(在etcd中抢占分布式乐观锁：/cron/lock/任务名，抢占成功通过command类执行shell任务，捕获command输出并等待子进程结束，将执行结果投递给调度协程)<br>
5.把执行日志保存到mongodb<br>

### 调度协程
1.监听任务变更event，更新内存中维护的任务列表<br>
2.检查任务cron表达式，扫描到期任务，交给执行协程运行<br>
3.监听任务控制event，强制中断正在执行中的子进程<br>
4.监听任务执行result，更新内存中任务状态，投递执行日志<br>

### 日志协程
1.监听调度发来的执行日志，放入一个batch中<br>
2.对新batch启动定时器，超时未满自动提交<br>
3.若batch被放满，那么立刻提交，并取消自动提交定时器<br>

