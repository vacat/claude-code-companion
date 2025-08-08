package tagging

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// TaggerPipeline 负责管理和执行所有tagger
type TaggerPipeline struct {
	taggers []Tagger
	timeout time.Duration
	mu      sync.RWMutex
}

// NewTaggerPipeline 创建新的tagger管道
func NewTaggerPipeline(timeout time.Duration) *TaggerPipeline {
	return &TaggerPipeline{
		taggers: make([]Tagger, 0),
		timeout: timeout,
	}
}

// AddTagger 添加一个tagger到管道中
func (tp *TaggerPipeline) AddTagger(tagger Tagger) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	tp.taggers = append(tp.taggers, tagger)
}

// SetTaggers 设置管道中的所有tagger
func (tp *TaggerPipeline) SetTaggers(taggers []Tagger) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	tp.taggers = make([]Tagger, len(taggers))
	copy(tp.taggers, taggers)
}

// ProcessRequest 处理HTTP请求，并发执行所有tagger进行标记
func (tp *TaggerPipeline) ProcessRequest(req *http.Request) (*TaggedRequest, error) {
	tp.mu.RLock()
	taggers := make([]Tagger, len(tp.taggers))
	copy(taggers, tp.taggers)
	tp.mu.RUnlock()

	// 预处理请求体 - 读取并缓存，然后重新设置给request
	var cachedBody []byte
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			cachedBody = body
			// 重新设置请求体，这样后续代理请求不会受影响
			req.Body = io.NopCloser(bytes.NewReader(body))
			
			// 将缓存的请求体设置到context中，供tagger使用
			ctx := context.WithValue(req.Context(), "cached_body", cachedBody)
			req = req.WithContext(ctx)
		}
	}

	// 创建上下文，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), tp.timeout)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var tags []string
	var tagSet map[string]bool // 用于快速检查标签重复
	var results []TaggerResult

	tagSet = make(map[string]bool)

	// 并发执行所有tagger
	for _, tagger := range taggers {
		wg.Add(1)
		go func(t Tagger) {
			defer wg.Done()
			
			start := time.Now()
			matched, err := t.ShouldTag(req)
			duration := time.Since(start)
			
			// 创建结果记录
			result := TaggerResult{
				TaggerName: t.Name(),
				Tag:        t.Tag(),
				Matched:    matched,
				Error:      err,
				Duration:   duration,
			}
			
			mu.Lock()
			results = append(results, result)
			
			// 如果匹配成功且没有错误，添加tag（去重）
			if matched && err == nil {
				// 使用map快速检查tag是否已存在
				tag := t.Tag()
				if !tagSet[tag] {
					tagSet[tag] = true
					tags = append(tags, tag)
				}
			}
			mu.Unlock()
		}(tagger)
	}

	// 等待所有tagger完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有tagger执行完成
	case <-ctx.Done():
		// 超时，但不影响已完成的结果
		break
	}

	return &TaggedRequest{
		OriginalRequest: req,
		Tags:           tags,
		TaggingTime:    time.Now(),
		TaggerResults:  results,
	}, nil
}

// GetTaggers 获取当前管道中的所有tagger
func (tp *TaggerPipeline) GetTaggers() []Tagger {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	
	taggers := make([]Tagger, len(tp.taggers))
	copy(taggers, tp.taggers)
	return taggers
}

// GetTimeout 获取管道超时时间
func (tp *TaggerPipeline) GetTimeout() time.Duration {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	
	return tp.timeout
}

// SetTimeout 设置管道超时时间
func (tp *TaggerPipeline) SetTimeout(timeout time.Duration) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	tp.timeout = timeout
}