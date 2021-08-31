package parentlesscheck

import (
	"github.com/zilionixx/zilion-base/eventcheck/queuedcheck"
	"github.com/zilionixx/zilion-base/hash"
	"github.com/zilionixx/zilion-base/inter/dag"
)

type Checker struct {
	callback Callback
}

type LightCheck func(dag.Event) error

type HeavyCheck interface {
	Enqueue(tasks []queuedcheck.EventTask, onValidated func(ee []queuedcheck.EventTask)) error
}

type Callback struct {
	// FilterInterested returns only event which may be requested.
	OnlyInterested func(ids hash.Events) hash.Events

	HeavyCheck HeavyCheck
	LightCheck LightCheck
}

func New(callback Callback) *Checker {
	return &Checker{
		callback: callback,
	}
}

// Enqueue tries to fill gaps the fetcher's future import queue.
func (c *Checker) Enqueue(tasks []queuedcheck.EventTask, checked func(ee []queuedcheck.EventTask)) error {
	passed := make([]queuedcheck.EventTask, 0, len(tasks))

	for _, e := range tasks {
		if len(c.callback.OnlyInterested(hash.Events{e.Event().ID()}))  == 0 {
			checked([]queuedcheck.EventTask{e})
			continue
		}

		err := c.callback.LightCheck(e.Event())
		if err != nil {
			checked([]queuedcheck.EventTask{e})
			continue
		}
		passed = append(passed, e)
	}

	return c.callback.HeavyCheck.Enqueue(passed, checked)
}
