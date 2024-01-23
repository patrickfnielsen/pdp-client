package pdp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"github.com/patrickfnielsen/pdp-client/internal/util"
)

const (
	minRetryDelay               = time.Millisecond * 100
	defaultMinDelaySeconds      = int64(1)
	defaultMaxDelaySeconds      = int64(10)
	defaultBufferChunkSizeBytes = int64(32768) // 32KB limit
	defaultBufferSizeLimitBytes = int64(0)     // unlimited
)

type DecisionLogConfig struct {
	ConsoleLog           bool
	HTTPLog              bool
	BufferChunkSizeBytes *int64
	BufferSizeLimitBytes *int64
	MinDelaySeconds      *int64
	MaxDelaySeconds      *int64
	Endpoint             string
	EndpointTimeout      int
	BearerToken          string
}

func (c *DecisionLogConfig) validateAndInjectDefaults() error {
	min := defaultMinDelaySeconds
	max := defaultMaxDelaySeconds

	// reject bad min/max values
	if c.MaxDelaySeconds != nil && c.MinDelaySeconds != nil {
		if *c.MaxDelaySeconds < *c.MinDelaySeconds {
			return fmt.Errorf("max reporting delay must be >= min reporting delay in decision_logs")
		}
		min = *c.MinDelaySeconds
		max = *c.MaxDelaySeconds
	} else if c.MaxDelaySeconds == nil && c.MinDelaySeconds != nil {
		return fmt.Errorf("reporting configuration missing 'max_delay_seconds' in decision_logs")
	} else if c.MinDelaySeconds == nil && c.MaxDelaySeconds != nil {
		return fmt.Errorf("reporting configuration missing 'min_delay_seconds' in decision_logs")
	}

	// scale to seconds
	minSeconds := int64(time.Duration(min) * time.Second)
	c.MinDelaySeconds = &minSeconds

	maxSeconds := int64(time.Duration(max) * time.Second)
	c.MaxDelaySeconds = &maxSeconds

	// default the upload size limit
	uploadLimit := defaultBufferChunkSizeBytes
	if c.BufferChunkSizeBytes != nil {
		uploadLimit = *c.BufferChunkSizeBytes
	}

	c.BufferChunkSizeBytes = &uploadLimit

	// default the buffer size limit
	bufferLimit := defaultBufferSizeLimitBytes
	if c.BufferSizeLimitBytes != nil {
		bufferLimit = *c.BufferSizeLimitBytes
	}

	c.BufferSizeLimitBytes = &bufferLimit

	return nil
}

type decisionLogger struct {
	config     *DecisionLogConfig
	buffer     *logBuffer
	enc        *chunkEncoder
	httpClient *http.Client
	mtx        sync.Mutex
	stop       chan chan struct{}
}

func newLogger(config *DecisionLogConfig) (*decisionLogger, error) {
	err := config.validateAndInjectDefaults()
	if err != nil {
		return nil, err
	}

	return &decisionLogger{
		config:     config,
		stop:       make(chan chan struct{}),
		buffer:     newLogBuffer(*config.BufferSizeLimitBytes),
		enc:        newChunkEncoder(*config.BufferChunkSizeBytes),
		httpClient: defaultRoundTripperClient(config.EndpointTimeout),
	}, nil
}

func (l *decisionLogger) Start() {
	go l.loop()
}

func (l *decisionLogger) Stop(ctx context.Context) error {
	err := l.flushDecisions(ctx)

	done := make(chan struct{})
	l.stop <- done
	<-done
	return err
}

func (l *decisionLogger) Log(event DecisionResult) error {
	if l.config.ConsoleLog {
		l.logEventConsole(event)
	}

	if l.config.HTTPLog {
		l.mtx.Lock()
		l.encodeAndBufferEvent(event)
		l.mtx.Unlock()
	}

	return nil
}

func (p *decisionLogger) flushDecisions(ctx context.Context) error {
	slog.Info("flushing decision logs")
	done := make(chan bool)

	go func(ctx context.Context, done chan bool) {
		for ctx.Err() == nil {
			if _, err := p.oneShot(ctx); err != nil {
				// Wait some before retrying, but skip incrementing interval since we are shutting down
				time.Sleep(1 * time.Second)
			} else {
				done <- true
				return
			}
		}
	}(ctx, done)

	select {
	case <-done:
		slog.Info("all decisions in buffer uploaded.")
	case <-ctx.Done():
		switch ctx.Err() {
		case context.DeadlineExceeded, context.Canceled:
			return fmt.Errorf("logger stopped with decisions possibly still in buffer")
		}
	}
	return nil
}

func (l *decisionLogger) doOneShot(ctx context.Context) error {
	uploaded, err := l.oneShot(ctx)

	l.mtx.Lock()
	defer l.mtx.Unlock()

	if err != nil {
		slog.Error("failed to upload decision logs", slog.String("error", err.Error()))
	} else if uploaded {
		slog.Info("decision logs uploaded successfully")
	} else {
		slog.Debug("log upload queue was empty.")
	}
	return err
}

func (l *decisionLogger) oneShot(ctx context.Context) (ok bool, err error) {
	// Make a local copy of the encoder and buffer and create
	// a new encoder and buffer. This is needed as locking the buffer for
	// the upload duration will block policy evaluation and result in
	// increased latency for clients
	l.mtx.Lock()
	oldChunkEnc := l.enc
	oldBuffer := l.buffer
	l.buffer = newLogBuffer(*l.config.BufferSizeLimitBytes)
	l.enc = newChunkEncoder(*l.config.BufferChunkSizeBytes)
	l.mtx.Unlock()

	// Along with uploading the compressed events in the buffer
	// to the remote server, flush any pending compressed data to the
	// underlying writer and add to the buffer.
	chunk, err := oldChunkEnc.Flush()
	if err != nil {
		return false, err
	}

	for _, ch := range chunk {
		l.bufferChunk(oldBuffer, ch)
	}

	if oldBuffer.Len() == 0 {
		return false, nil
	}

	for bs := oldBuffer.Pop(); bs != nil; bs = oldBuffer.Pop() {
		if err == nil {
			err = l.uploadChunk(ctx, bs)
		}
		if err != nil {
			l.mtx.Lock()
			l.bufferChunk(l.buffer, bs)
			l.mtx.Unlock()
		}
	}

	return err == nil, err
}

func (l *decisionLogger) loop() {
	ctx, cancel := context.WithCancel(context.Background())
	var retry int

	for {
		var delay time.Duration
		var waitC chan struct{}
		err := l.doOneShot(ctx)

		if err == nil {
			min := float64(*l.config.MinDelaySeconds)
			max := float64(*l.config.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		} else {
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*l.config.MaxDelaySeconds), retry)
		}

		slog.Debug("waiting before next upload/retry.", slog.Duration("delay", delay))

		waitC = make(chan struct{})
		go func() {
			select {
			case <-time.After(delay):
				if err != nil {
					retry++
				} else {
					retry = 0
				}
				close(waitC)
			case <-ctx.Done():
			}
		}()

		select {
		case <-waitC:
		case done := <-l.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}
}

func (l *decisionLogger) encodeAndBufferEvent(event DecisionResult) {
	result, err := l.enc.Write(event)
	if err != nil {
		slog.Error("log encoding failed", slog.String("error", err.Error()))
		return
	}
	for _, chunk := range result {
		l.bufferChunk(l.buffer, chunk)
	}
}

func (l *decisionLogger) bufferChunk(buffer *logBuffer, bs []byte) {
	dropped := buffer.Push(bs)
	if dropped > 0 {
		slog.Warn("Dropped chunks from buffer. Reduce reporting interval or increase buffer size.", slog.Int("chunks", dropped))
	}
}

func (l *decisionLogger) logEventConsole(event DecisionResult) {
	slog.Info("decision log", slog.Any("decision", event))
}

func (l *decisionLogger) uploadChunk(ctx context.Context, data []byte) error {
	body := bytes.NewReader(data)
	request, err := http.NewRequestWithContext(ctx, "POST", l.config.Endpoint, body)
	if err != nil {
		return err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Content-Encoding", "gzip")
	request.Header.Add("Authorization", fmt.Sprintf("bearer %v", l.config.BearerToken))

	resp, err := l.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("log upload failed: %w", err)
	}

	defer closeHttp(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("log upload invalid status code: %d", resp.StatusCode)
	}

	return nil
}

func defaultRoundTripperClient(timeout int) *http.Client {
	// Ensure we use a http.Transport with proper settings: the zero values are not
	// a good choice, as they cause leaking connections:
	// https://github.com/golang/go/issues/19620

	// copy, we don't want to alter the default client's Transport
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = time.Duration(timeout) * time.Second

	c := *http.DefaultClient
	c.Transport = tr
	return &c
}

func closeHttp(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return
		}
		resp.Body.Close()
	}
}
