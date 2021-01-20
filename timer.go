package timer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	TIMER_READY = iota
	TIMER_RUN
	TIMER_STOP
)

type timeArgs []interface{}
type timeFunc func(timeArgs)

func NowTime() int64{
	t := time.Now()
	return t.Unix()
}

type delayFunc struct {
	f timeFunc
	args timeArgs
}

type Timer struct {
	// 执行时间
	unixTime int64
	// 执行函数
	df *delayFunc
	// tid
	tid uint32
}

type hashTimer struct {
	// 每个Timer分配一个唯一的tid
	tid uint32
	// 时间->tid->timer
	timers map[int64]map[uint32]*Timer
	// 上次执行到的时间
	preTime int64
	// ctx
	ctx context.Context
	cancel context.CancelFunc
	// 消息channel
	dfC chan *delayFunc
	// mutex
	mutex sync.Mutex
	// state
	State int
	// 提供给调用者判断其是否终止的通道
	Done chan struct{}
	// tid -> time
	tidMapTime map[uint32]int64

}

// 调用者需要提供ctx 以及接受消息函数的通道
func NewHashTimer(ctx context.Context, dfC chan *delayFunc) *hashTimer {
	ctx1, cancel := context.WithCancel(ctx)
	return &hashTimer{
		timers: make(map[int64]map[uint32]*Timer, 10),
		ctx: ctx1,
		cancel: cancel,
		tidMapTime: make(map[uint32]int64, 10),
		dfC: dfC,
		State: TIMER_READY,
	}
}

func NewDelayFunc(callBackFunc timeFunc, args timeArgs) *delayFunc {
	return &delayFunc{
		f:    callBackFunc,
		args: args,
	}
}

func (d *delayFunc)Call() (err error){
	defer func() {
		if derr := recover(); derr != nil {
			err = fmt.Errorf("delay func call err = %v", derr)
		}
	}()
	d.f(d.args)
	return err
}

func NewTimer(unixTime int64, tid uint32, df *delayFunc) *Timer {
	t := &Timer{
		unixTime: unixTime,
		tid: tid,
		df: df,
	}
	return t
}

func (h *hashTimer)AddTimer(unixTime int64, df *delayFunc) (uint32, error) {
	if h.preTime >= unixTime {
		return 0, errors.New("add timer error, pretime >= unixTime")
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if _, ok := h.timers[unixTime]; !ok {
		h.timers[unixTime] = make(map[uint32]*Timer, 10)
	}
	h.timers[unixTime][h.tid] = NewTimer(unixTime, h.tid, df)
	h.tidMapTime[h.tid] = unixTime
	h.tid++
	return h.tid-1, nil
}

func (h *hashTimer)StopTimer(tid uint32) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if unixTime, ok := h.tidMapTime[tid]; !ok {
		return fmt.Errorf("stopTimer error, cannot find tid = %d", h.tid)
	} else {
		delete(h.tidMapTime, tid)
		delete(h.timers[unixTime], tid)
	}
	return nil
}

func (h *hashTimer)Run() {
	if h.State == TIMER_STOP {
		return
	}
	h.preTime = NowTime()
	h.State = TIMER_RUN
	go tick(h)
}

func (h *hashTimer)Stop() {
	if h.State == TIMER_STOP {
		return
	}
	h.State = TIMER_STOP
	h.cancel()
	h.Done <- struct{}{}
}

func tick(h *hashTimer) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.State == TIMER_STOP {
		return
	}
	currTime := NowTime()
	tchan := time.After(1 * time.Second)
	for i := h.preTime + 1; i <= currTime; i++ {
		if m, ok := h.timers[i]; ok {
			for _, t := range m {
				select {
					case h.dfC <- t.df:
						delete(h.tidMapTime, t.tid)
					default:
						h.Stop()
						return
				}
			}
		}
		delete(h.timers, i)
	}
	h.preTime = currTime
	go func () {
		select {
			case <-tchan:
				tick(h)
			case <-h.ctx.Done():
				h.Stop()
				return
		}
	}()
}