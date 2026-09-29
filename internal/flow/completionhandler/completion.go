package completionhandler

import "sync"

type CompletionHandler struct {
	mu       sync.Mutex
	err      error
	parties  int
	resolved int
	done     bool
	running  bool
	resolve  func()
	reject   func(error)
}

func New(resolve func(), reject func(error)) *CompletionHandler {
	return &CompletionHandler{
		parties:  0,
		resolved: 0,
		resolve:  resolve,
		reject:   reject,
		running:  false,
		done:     false,
	}
}

func (h *CompletionHandler) Register() (func(), func(err error)) {
	h.mu.Lock()
	h.parties = h.parties + 1
	h.mu.Unlock()
	return func() {
			h.complete(nil)
		}, func(err error) {
			h.complete(err)
		}
}

func (h *CompletionHandler) complete(err error) {
	h.mu.Lock()
	if err != nil && h.err == nil {
		h.err = err
	}
	if err == nil {
		h.resolved++
	}
	resolve, reject, rejectErr := h.handleLocked()
	h.mu.Unlock()
	if reject != nil {
		reject(rejectErr)
	} else if resolve != nil {
		resolve()
	}
}

func (h *CompletionHandler) handleLocked() (func(), func(error), error) {
	if h.running && !h.done {
		if h.err != nil {
			h.done = true
			return nil, h.reject, h.err
		} else if h.resolved == h.parties {
			h.done = true
			return h.resolve, nil, nil
		}
	}
	return nil, nil, nil
}

func (h *CompletionHandler) Execute() {
	h.mu.Lock()
	h.running = true
	resolve, reject, err := h.handleLocked()
	h.mu.Unlock()
	if reject != nil {
		reject(err)
	} else if resolve != nil {
		resolve()
	}
}
