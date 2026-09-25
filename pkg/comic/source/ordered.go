package source

import (
	"sort"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
)

// indexedTask pairs a decoded task with its position in the sorted name list.
type indexedTask struct {
	id   int
	task epubimageloader.Task
}

// pumpOrdered re-emits tasks in ascending Id order and closes out once results
// is closed. Decode workers finish in arbitrary order, so without this the
// stream a loader returns would depend on worker scheduling and the sort order
// its first pass computed would be lost.
func pumpOrdered(results <-chan indexedTask, out chan<- epubimageloader.Task) {
	defer close(out)

	next := 0
	pending := make(map[int]epubimageloader.Task)
	for r := range results {
		pending[r.id] = r.task
		for {
			t, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			next++
			out <- t
		}
	}

	// A worker can stop early (cancelled context, archive error), leaving holes
	// in the id sequence: flush what arrived instead of dropping it.
	if len(pending) == 0 {
		return
	}
	ids := make([]int, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		out <- pending[id]
	}
}
