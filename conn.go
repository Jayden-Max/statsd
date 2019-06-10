package statsd

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// define errors
var (
	Err_NotConnected      = errors.New("can't send stats, not connected to StatsD server")
	Err_InvalidCount      = errors.New("count is less than zero")
	Err_InvalidSampleRate = errors.New("sample rate larger than 1 or less than 0")
)

// client to send events to StatsD
type Client struct {
	addr        string        // 数据库IP地址
	prefix      string        // 前缀
	conn        net.Conn      // net 连接
	buffer      []bufferCount // buffer
	mux         sync.Mutex    // mux
	flushTicker *time.Ticker  // 定时器
}

type bufferCount struct {
	name       string
	count      int64
	sampleRate float32
}

func NewClient(addr string, prefix string) (*Client, error) {
	prefix = strings.TrimRight(prefix, ".") // 移除.后面的字符
	cli := &Client{
		addr:   addr,
		prefix: prefix,
	}

	// use udp, expire 10s
	conn, err := net.DialTimeout("udp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	cli.conn = conn
	cli.flushTicker = time.NewTicker(10 * time.Second)
	go cli.bufferSendLoop()

	return cli, nil
}

// buffer 循环发送
func (cli *Client) bufferSendLoop() {
	for range cli.flushTicker.C {
		cli.mux.Lock()
		if len(cli.buffer) == 0 {
			cli.mux.Unlock()
			continue
		}
		buffer := cli.buffer
		cli.buffer = nil
		cli.mux.Unlock()

		for idx := range buffer {
			cli.send(buffer[idx].name, buffer[idx].count, "c", buffer[idx].sampleRate)
		}
	}
}

// stat is a name
func (cli *Client) addToBuffer(stat string, count int64, sampleRate float32) {
	cli.mux.Lock()
	for i := range cli.buffer {
		if cli.buffer[i].name == stat {
			cli.buffer[i].count++
			cli.mux.Unlock()
			return
		}
	}
	cli.buffer = append(cli.buffer, bufferCount{stat, count, sampleRate})
	cli.mux.Unlock()
}

// close the UDP connection
func (cli *Client) Close() error {
	cli.flushTicker.Stop()
	if cli.conn == nil {
		return nil
	}

	return cli.conn.Close()
}

// see statsD data type. refer: https://github.com/b/statsd_spec
// or https://statsd.readthedocs.io/en/latest/types.html

// Incr a counter metric
// often used to note a special event.
func (cli *Client) IncrWithSampling(stat string, count int64, sampleRate float32) error {
	if err := checkSampleRate(sampleRate); err != nil {
		return err
	}

	if !shouldFire(sampleRate) {
		return nil // ignore this call
	}

	if err := checkCount(count); err != nil {
		return err
	}

	cli.addToBuffer(stat, count, sampleRate)
	return nil
}

// Decr a counter metric
func (cli *Client) DecrWithSampling(stat string, count int64, sampleRate float32) error {
	if err := checkSampleRate(sampleRate); err != nil {
		return err
	}

	if !shouldFire(sampleRate) {
		return nil
	}

	if err := checkCount(count); err != nil {
		return err
	}

	return cli.send(stat, -count, "c", sampleRate)
}

// Timing - Track a duration event
// The time delta must be given in milliseconds
func (cli *Client) TimingWithSampling(stat string, delta int64, sampleRate float32) error {
	if err := checkSampleRate(sampleRate); err != nil {
		return err
	}

	if !shouldFire(sampleRate) {
		return nil
	}

	return cli.send(stat, delta, "ms", sampleRate)
}

// Gauge - Gauges are a constant data type. They are not subject to averaging,
// and they don't change unless you change them. That is, once you set a gauge value,
// it will be a flat line on the graph until you change it again. If you specify
// delta to be true, that specifies that the gauge should be updated, not set.
// Due to the underlying protocol, you can't explicitly set a gauge to a negative number
// without first setting it to zero.
func (cli *Client) GaugeWithSampling(stat string, value int64, sampleRate float32) error {
	if err := checkSampleRate(sampleRate); err != nil {
		return err
	}

	if !shouldFire(sampleRate) {
		return nil
	}

	if value < 0 {
		cli.send(stat, 0, "g", 1)
	}

	return cli.send(stat, value, "g", sampleRate)
}

// send a float point value for gauge
func (cli *Client) FGaugeWithSampling(stat string, value float64, sampleRate float32) error {
	if err := checkSampleRate(sampleRate); err != nil {
		return err
	}

	if !shouldFire(sampleRate) {
		return nil
	}

	if value < 0 {
		cli.send(stat, value, "g", 1)
	}

	return cli.send(stat, value, "g", sampleRate)
}

// metric 指标
// <bucket>:<value>|<type>[|@sample_rate]
// bucket is metric的标识，可以看成一个metric的变量; 这个标识是我们定义时候传进来的函数名称 or 其他标识
// value is metric的值，通常是数字
// type is metric的类型，通常有timer、counter、gauge和set四种
// sample_rate
// write UDP packet with the statsd event
func (cli *Client) send(bucket string, value interface{}, t string, sampleRate float32) error {
	if cli.conn == nil {
		return Err_NotConnected
	}

	if cli.prefix != "" {
		bucket = fmt.Sprintf("%s.%s", cli.prefix, bucket)
	}

	metric := fmt.Sprintf("%s:%v|%s|@%f", bucket, value, t, sampleRate)

	_, err := cli.conn.Write([]byte(metric))
	return err
}

func checkCount(c int64) error {
	if c <= 0 {
		return Err_InvalidCount
	}

	return nil
}

func checkSampleRate(rate float32) error {
	if rate < 0 || rate > 1 {
		return Err_InvalidSampleRate
	}

	return nil
}

func shouldFire(sampleRate float32) bool {
	if sampleRate == 1 {
		return true
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	return r.Float32() <= sampleRate
}
