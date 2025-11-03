package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

func (lt *LoadTester) printLiveStats(ctx context.Context, ticker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			total := atomic.LoadInt64(&lt.totalRequests)
			success := atomic.LoadInt64(&lt.successRequests)
			failed := atomic.LoadInt64(&lt.failedRequests)

			if total > 0 {
				successRate := float64(success) / float64(total) * 100
				failRate := float64(failed) / float64(total) * 100

				fmt.Printf("\r%s", strings.Repeat(" ", 80))
				fmt.Printf("\rRequests: %d | Erfolg: %d (%.1f%%) | Fehler: %d (%.1f%%)",
					total, success, successRate, failed, failRate)
			}
		}
	}
}

func (lt *LoadTester) printFinalStats() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("FINALE STATISTIKEN")
	fmt.Println(strings.Repeat("=", 60))

	total := atomic.LoadInt64(&lt.totalRequests)
	success := atomic.LoadInt64(&lt.successRequests)
	failed := atomic.LoadInt64(&lt.failedRequests)
	avgRT := atomic.LoadInt64(&lt.avgResponseTime)

	if total > 0 {
		avgResponseTime := avgRT / total
		successRate := float64(success) / float64(total) * 100
		failRate := float64(failed) / float64(total) * 100
		rps := float64(total) / lt.duration.Seconds()

		fmt.Printf("Zielserver: %s\n", lt.targetServer)
		fmt.Printf("Testdauer: %v\n", lt.duration)
		fmt.Printf("Gesamt Requests: %d\n", total)
		fmt.Printf("Erfolgreiche Requests: %d (%.2f%%)\n", success, successRate)
		fmt.Printf("Fehlgeschlagene Requests: %d (%.2f%%)\n", failed, failRate)
		fmt.Printf("Requests/Sekunde: %.2f\n", rps)
		fmt.Printf("Durchschnittliche Antwortzeit: %dms\n", avgResponseTime)

		errorStats := lt.getErrorStats()
		if len(errorStats) > 0 {
			fmt.Println("\nFEHLER-VERTEILUNG:")
			for errorType, count := range errorStats {
				percentage := float64(count) / float64(failed) * 100
				fmt.Printf("  %s: %d (%.1f%%)\n", errorType, count, percentage)
			}
		}
	} else {
		fmt.Println("Keine Requests verarbeitet")
	}

	fmt.Println(strings.Repeat("=", 60))
}
