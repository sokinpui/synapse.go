package broker

import (
	"sync"

	"github.com/sokinpui/synapse.go/internal/models"
	"github.com/sokinpui/synapse.go/model"
)

type MemoryBroker struct {
	tasks         chan *models.GenerationTask
	subscribers   map[string]chan model.StreamChunk
	cancellations map[string]chan struct{}
	mu            sync.RWMutex
}

func NewMemoryBroker(bufferSize int) *MemoryBroker {
	return &MemoryBroker{
		tasks:         make(chan *models.GenerationTask, bufferSize),
		subscribers:   make(map[string]chan model.StreamChunk),
		cancellations: make(map[string]chan struct{}),
	}
}

func (b *MemoryBroker) Enqueue(task *models.GenerationTask) {
	b.tasks <- task
}

func (b *MemoryBroker) Dequeue() <-chan *models.GenerationTask {
	return b.tasks
}

func (b *MemoryBroker) Subscribe(id string) chan model.StreamChunk {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan model.StreamChunk, 100)
	b.subscribers[id] = ch
	return ch
}

func (b *MemoryBroker) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}

	if cancelCh, ok := b.cancellations[id]; ok {
		close(cancelCh)
		delete(b.cancellations, id)
	}
}

func (b *MemoryBroker) Publish(id string, msg model.StreamChunk) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if ch, ok := b.subscribers[id]; ok {
		ch <- msg
	}
}

func (b *MemoryBroker) SignalCancel(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.cancellations[id]; !ok {
		b.cancellations[id] = make(chan struct{})
	}
	close(b.cancellations[id])
	delete(b.cancellations, id)
}

func (b *MemoryBroker) IsCancelled(id string) <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.cancellations[id]; ok {
		return ch
	}

	ch := make(chan struct{})
	b.cancellations[id] = ch
	return ch
}
