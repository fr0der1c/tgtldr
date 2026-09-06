package model

import "time"

// SummaryRetryDelay 返回单群摘要与每日总览共用的阶梯等待时间，最长为十分钟。
func SummaryRetryDelay(retryNumber int) time.Duration {
	delays := [...]time.Duration{time.Minute, 3 * time.Minute, 5 * time.Minute, 10 * time.Minute}
	if retryNumber < 1 {
		retryNumber = 1
	}
	if retryNumber > len(delays) {
		retryNumber = len(delays)
	}
	return delays[retryNumber-1]
}
