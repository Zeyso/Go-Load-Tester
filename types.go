package main

import (
	"fmt"
	"sync"
	"time"
)

type LoadTester struct {
	targetServer    string
	targetHost      string
	targetPort      uint16
	concurrency     int
	requestsPerSec  int
	duration        time.Duration
	proxies         []ProxyConfig
	useRotating     bool
	currentProxyIdx int64

	totalRequests   int64
	successRequests int64
	failedRequests  int64
	avgResponseTime int64
	errorStats      map[string]int64
	errorMutex      sync.RWMutex
}

type ProxyError struct {
	ProxyAddr string
	Protocol  string
	Message   string
	Err       error
}

func (e *ProxyError) Error() string {
	return fmt.Sprintf("proxy error [%s://%s]: %s: %v",
		e.Protocol, e.ProxyAddr, e.Message, e.Err)
}

type ProxyTestResult struct {
	Proxy   ProxyConfig
	Success bool
	Error   error
}
