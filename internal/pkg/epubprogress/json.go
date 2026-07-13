package epubprogress

import (
	"encoding/json"
	"sync/atomic"
)

type jsonprogress struct {
	o       Options
	e       *json.Encoder
	current atomic.Int64
}

func (p *jsonprogress) Add(num int) error {
	p.current.Add(int64(num))
	return p.e.Encode(map[string]any{
		"type": "epubprogress",
		"data": map[string]any{
			"epubprogress": map[string]any{
				"current": p.current.Load(),
				"total":   p.o.Max,
			},
			"steps": map[string]any{
				"current": p.o.CurrentJob,
				"total":   p.o.TotalJob,
			},
			"description": p.o.Description,
		},
	})
}

func (p *jsonprogress) Close() error {
	return nil
}
