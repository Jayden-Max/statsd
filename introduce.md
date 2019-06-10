##STATSD原理
监听UDP(默认)或TCP的守护程序，根据简单的协议收集statsd客户端发送来的数据，聚合之后，定时推送给后端，如：graphite和influxdb等，
再通过grafana等展示。

statsd系统包括三部分：客户端(client)、服务器(server)和后端(backend)。
 - 客户端植入于应用代码中，将相应的metrics上报给statsd server。
 - statsd server聚合这些metrics之后，定时发送给backend。
 - backend则负责存储这些时间序列，并通过适当的图表工具展示。


###metric的协议格式
`
<metricname>:<value>|<type>[|@sample_rate]
`

>value: metric的值，通常是数字

>type: metric的类型，通常有timer、counter、gauge和set四种

>sample_rate: 如果数据上报量过大，很容易溢满statsd。所以适当的降低采样，减少server负载。
这个频率容易误解，需要解释一下，客户端减少数据上报的频率，然后在发送的数据中加入采样频率，如:0.1。
statsd server收到上报的数据之后，如cnt=10，得知此数据是采样的数据，然后flush的时候，按采样频率恢复数据
来发送给backend，即flush的时候，数据为cnt=10/0.1=100，而不是容易误解的10*0.1=1。

statsd 默认是10s执行一次flush，直白点：10s发送一次数据给server。metric上报时，每次flush
之后，就会重置为0(gauge是保存原有值)。


也就是对于一个metric来说，只要想好他的名字以及对应的类型，然后发实际的数据给StatsD。

##Metric Types
###1. Counting
步进，通常的计数功能，StatsD会将收到的counter value累加，然后在flush的时候输出，并且重新清零。
所以用counter就能非常方便的查看一段实际某个操作的频率，譬如对于一个HTTP的服务来说，我们可以使用
counter来统计request的次数，finish这个request的次数以及fail的次数。

###2. Gauges
数值，Gauge在下次flush的时候不会清零的，另外，gauge通常是在client进行统计好再发给StatsD的，
比如：`capacity:100|g`这样的gauge，即使我们发送多次，在StatsD里面，也只会保存100，不会像
counter那样进行累加。

但是，我们可以通过显示的加入符号来让StatsD帮我们进行累加，比如：

```gauge
capacity:+100|g    // 原有值加100
capacity:-100|g    // 原有值减100

```

假设原来capacity gauge的值为100，经过上面的操作之后，gauge仍然为100

如果我们需要记录当前的总用户数，或者CPU，Memory的usage，使用gauge就是一个不错的选择。

###3. Sets
set用来计算某个metric unique事件的个数，比如对于一个接口，可能我们想知道有多少个user访问了，我们可以这样：

```sets
request:1|s
request:2|s
request:1|s

```
StatsD就会展示这个request metric只有1，2两个用户访问了。

###4. Timing
timing，顾名思义，就是记录某个操作的耗时，比如：
`foo:100|ms`

上面的例子中，完成foo这个操作花费了100ms，但仅仅是记录这个操作的耗时，并不能让我们很好的知道当前系统的情况，所以通常，timing都是跟
histogram一起来使用的。

在StatsD里面，配置histogram很简单，例如：

`histogram: [ { metric: '', bins: [10, 100, 1000, 'inf']} ]`

在上面的例子中，我们开启了histogram，这个histogram的bin的间隔是[-inf, 10ms),[10ms - 100ms),[100ms - 1000ms),以及[1000ms, +inf)，
如果一个timing落在了某个bin里面，相应的bin的计数就加1，比如：
```timing
foo:1|ms
foo:100|ms
foo:1|ms
foo:1000|ms

```

那么StatsD在console就会显示：
`histogram: { bin_10: 2, bin_100: 0, bin_1000: 1, bin_inf: 1}`

##notice some point
* UDP虽然很快，但仍然可能会因为发送buffer满block当前进程，建议设置成noblock，对于metric来说，其实我们并不在意丢几个包。
* 埋点是一个辛苦活，太多或者太少的metric其实都没啥用。
* metric并不是万能的，它只是一个系统的汇总统计，有时候我们还需要借助log，flamegraph等其他方式来进行系统问题排查。

##名词解释
1. metric
是一种系统监控变量，通过监控metric的变化，就能知道当前系统的运行的状况。Metric的方案有很多，比如著名的prometheus、statsD等，也可以自己
造轮子，毕竟通用的metric types也就那么几种，用好了足够用来监控系统。

2. flush



[link1](http://blog.gezhiqiang.com/2017/01/25/statsd-summary/)
[link2](https://www.jianshu.com/p/2b0aa5898dd7)


