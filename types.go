package main

import (
	"fmt"
	"sync"
	"time"
)

type ProxyConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
}

type Config struct {
	Proxies []ProxyConfig `json:"proxies"`
}

type LoadTester struct {
	targetServer    string
	targetHost      string
	targetPort      uint16
	concurrency     int
	requestsPerSec  int
	duration        time.Duration
	proxies         []ProxyConfig
	currentProxyIdx int64
	useRotating     bool
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

type ProxyTestResult struct {
	Proxy   ProxyConfig
	Success bool
	Error   error
}

func (pe *ProxyError) Error() string {
	return fmt.Sprintf("Proxy %s (%s): %s - %v", pe.ProxyAddr, pe.Protocol, pe.Message, pe.Err)
}
