package timer

import (
	"context"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	m.Run()
}

func checkoutTime() timeFunc {
	return func(args timeArgs) {
		if !(NowTime() == args[0]) {
			t := args[1].(*testing.T)
			t.Error("not time")
		}
	}
}

func TestTime(t *testing.T) {
	dfs := make(chan *delayFunc, 10)
	h := NewHashTimer(context.Background(), dfs)
	h.Run()
	curTime := NowTime()
	t.Run("123", func(t *testing.T) {
		for i := 0; i < 500000; i++ {
			d := genS(curTime, i)
			h.AddTimer(d, NewDelayFunc(checkoutTime(), timeArgs{d, &t}))
		}
		tt := time.After(10*time.Second)
		for {
			select {
				case df := <- dfs:
					df.Call()
				case <- tt:
					return
			}
		}
	})
}

func genS(u int64, i int) int64{
	return u + int64(i%10)
}