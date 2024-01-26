package taskgroup

import (
	"errors"
	"fmt"
	"sync"
)

type TaskGroup struct {
	pipeline    int
	cache       int
	processChan []chan func()
	sync.WaitGroup
	exit chan struct{}
}

// NewTaskGroup 注意,初始化workgroup对象后,以及start了,不需要再次调用start方法
func NewTaskGroup(cacheSize, pipleSize int) *TaskGroup {
	ret := &TaskGroup{pipeline: pipleSize, cache: cacheSize, exit: make(chan struct{})}
	ret.start()
	return ret
}

var ErrWorkGroupStoped = errors.New("work group has been stopped")
var ErrorWorkChanFull = errors.New("work group channel has fulled")

func (f *TaskGroup) SendTask(sharding uint64, fun func()) error {
	index := sharding % uint64(f.pipeline)
	channel := f.processChan[int(index)]
	select {
	case <-f.exit:
		return ErrWorkGroupStoped
	default:
		channel <- fun
	}
	return nil
}

func (f *TaskGroup) SendStripTask(sharding uint64, fun func()) error {
	index := sharding % uint64(f.pipeline)
	select {
	case <-f.exit:
		return ErrWorkGroupStoped
	case f.processChan[int(index)] <- fun:
		return nil
	default:
		return ErrorWorkChanFull
	}
}

func (f *TaskGroup) start() {
	f.processChan = make([]chan func(), f.pipeline)
	for i := 0; i < f.pipeline; i++ {
		workChan := make(chan func(), f.cache)
		f.processChan[i] = workChan
		go f.process(i)
	}
}

func (f *TaskGroup) process(index int) {
	f.Add(1)
	defer f.Done()

	workChan := f.processChan[index]
	defer func() { f.processChan[index] = nil }()

	for {
		task, ok := <-workChan
		if !ok {
			return
		}
		fun := func() {
			defer func() {
				if e := recover(); e != nil {
					fmt.Printf("TaskGroup %d worker occur error, err:%v\n", index, e)
				}
			}()
			task()
		}
		fun()
	}

}

func (f *TaskGroup) Stop() {
	select {
	case <-f.exit:
		return
	default:
	}
	close(f.exit)
	for _, workChan := range f.processChan {
		close(workChan)
	}
	f.Wait()
}
