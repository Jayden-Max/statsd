package statsd

import (
	"fmt"
	"log"
	"sync"
	"time"
)

var config *Config
var addr string

type metricType int16

const (
	metricType_Count metricType = iota
	metricType_Gauge
	metricType_FGauge
	metricType_Timer

	default_SampleRate = 1.0
)

// config for StatsD
type Config struct {
	Host       string
	Port       int
	Project    string
	Enable     bool    // Indicate whether stats is enabled
	SampleRate float32 // Global StatsD sample rate
}

// set config
func Setup(cfg *Config) {
	config = cfg

	// if sample rate equal 0, it indicates the statsd never called
	// so we need to set a default rate
	if config.SampleRate == 0 {
		config.SampleRate = default_SampleRate
	}

	addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
}

var once sync.Once
var defaultClient *Client

func getClient() *Client {
	if config == nil {
		return nil
	}

	once.Do(func() {
		client, err := NewClient(addr, config.Project) // project 在初始化时，表明哪个服务
		if err != nil {
			panic(err)
		}
		defaultClient = client
	})

	return defaultClient
}

type metricItem struct {
	stat       string
	value      interface{}
	t          metricType
	sampleRate float32
}

var sendLoopOnce sync.Once
var sendChan chan *metricItem

// send metricItem to server
func send(stat string, value interface{}, t metricType, sampleRate float32) {
	sendAsync(stat, value, t, sampleRate)
}

func sendAsync(stat string, value interface{}, t metricType, sampleRate float32) {
	sendLoopOnce.Do(func() {
		if sendChan == nil {
			sendChan = make(chan *metricItem, 1024)
		}
		cli := getClient()
		go func() {
			for item := range sendChan {
				sendEx(cli, item.stat, item.value, item.t, item.sampleRate)
			}
		}()
	})

	select {
	case sendChan <- &metricItem{stat, value, t, sampleRate}:
	default:
		log.Printf("stat:%v,value:%v,type:%v,sampleRate:%v", stat, value, t, sampleRate)
	}
}

func sendEx(cli *Client, stat string, value interface{}, t metricType, sampleRate float32) {
	if stat == "" {
		return
	}

	switch t {
	case metricType_Count:
		if val, ok := value.(int64); ok {
			cli.IncrWithSampling(stat, val, sampleRate)
		}
	case metricType_Gauge:
		if val, ok := value.(int64); ok {
			cli.GaugeWithSampling(stat, val, sampleRate)
		}
	case metricType_FGauge:
		if val, ok := value.(float64); ok {
			cli.FGaugeWithSampling(stat, val, sampleRate)
		}
	case metricType_Timer:
		if val, ok := value.(int64); ok {
			cli.TimingWithSampling(stat, val, sampleRate)
		}
	default:
		// temporary to do nothing
	}
}

// Incr increment
func Incr(stat string, value int64) {
	IncrWithSampling(stat, value, config.SampleRate)
}

func IncrWithSampling(stat string, value int64, sampleRate float32) {
	if !config.Enable {
		return
	}

	send(stat, value, metricType_Count, sampleRate)
}

// Gauge set a constant value
func Gauge(stat string, value int64) {
	GaugeWithSampling(stat, value, config.SampleRate)
}

func GaugeWithSampling(stat string, value int64, sampleRate float32) {
	gauge(stat, value, metricType_Gauge, sampleRate)
}

// FGauge set a constant float64 point
func FGauge(stat string, value float64) {
	FGaugeWithSampling(stat, value, config.SampleRate)
}

func FGaugeWithSampling(stat string, value float64, sampleRate float32) {
	if !config.Enable {
		return
	}

	gauge(stat, value, metricType_FGauge, sampleRate)
}

func gauge(stat string, value interface{}, t metricType, sampleRate float32) {
	send(stat, value, t, sampleRate)
}

// Timing track duration of a event
func Timing(stat string, d time.Duration) {
	TimingWithSampling(stat, d, config.SampleRate)
}

func TimingWithSampling(stat string, d time.Duration, sampleRate float32) {
	if !config.Enable {
		return
	}

	// the d must be given in milliseconds
	t := d / time.Millisecond

	send(stat, int64(t), metricType_Timer, sampleRate)
}
