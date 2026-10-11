package model

import "time"

// SummaryPeriod 是一份摘要覆盖的周期，也是群组的摘要频率。
type SummaryPeriod string

const (
	SummaryPeriodDay   SummaryPeriod = "day"
	SummaryPeriodWeek  SummaryPeriod = "week"
	SummaryPeriodMonth SummaryPeriod = "month"
)

const periodDateLayout = "2006-01-02"

// NormalizeSummaryPeriod 把未知取值归为每日，兼容旧数据与旧客户端。
func NormalizeSummaryPeriod(period SummaryPeriod) SummaryPeriod {
	switch period {
	case SummaryPeriodWeek, SummaryPeriodMonth:
		return period
	default:
		return SummaryPeriodDay
	}
}

// IsValidSummaryPeriod 判断请求中的频率是否受支持。
func IsValidSummaryPeriod(period SummaryPeriod) bool {
	return period == SummaryPeriodDay || period == SummaryPeriodWeek || period == SummaryPeriodMonth
}

// PeriodStartDate 返回 date 所在周期的第一天；每周从周一开始，每月从 1 日开始。
func PeriodStartDate(period SummaryPeriod, date string) (string, error) {
	day, err := time.Parse(periodDateLayout, date)
	if err != nil {
		return "", err
	}
	return periodStart(NormalizeSummaryPeriod(period), day).Format(periodDateLayout), nil
}

// PeriodEndDate 返回从 start 开始的周期的最后一天（含）。
func PeriodEndDate(period SummaryPeriod, start string) (string, error) {
	day, err := time.Parse(periodDateLayout, start)
	if err != nil {
		return "", err
	}
	return periodEnd(NormalizeSummaryPeriod(period), day).Format(periodDateLayout), nil
}

// PeriodDays 返回从 start 开始的周期天数。
func PeriodDays(period SummaryPeriod, start string) (int, error) {
	day, err := time.Parse(periodDateLayout, start)
	if err != nil {
		return 0, err
	}
	end := periodEnd(NormalizeSummaryPeriod(period), day)
	return int(end.Sub(day).Hours()/24) + 1, nil
}

// SummaryPeriodFor 返回 date 应使用的周期。生效日期之前沿用上一频率；
// 上一频率的周期若会跨过生效日期，这几天按每日生成，保证新旧周期既不重叠也不留空。
func (c Chat) SummaryPeriodFor(date string) SummaryPeriod {
	day, err := time.Parse(periodDateLayout, date)
	if err != nil {
		return SummaryPeriodDay
	}
	return c.summaryPeriodFor(day)
}

func (c Chat) summaryPeriodFor(day time.Time) SummaryPeriod {
	current := NormalizeSummaryPeriod(c.SummaryFrequency)
	effective, err := time.Parse(periodDateLayout, c.SummaryFrequencyEffectiveDate)
	if err != nil || !day.Before(effective) {
		return current
	}
	previous := NormalizeSummaryPeriod(c.PreviousSummaryFrequency)
	if periodEnd(previous, periodStart(previous, day)).Before(effective) {
		return previous
	}
	return SummaryPeriodDay
}

// SummaryPeriodStart 返回 date 所在的摘要周期及其第一天。
func (c Chat) SummaryPeriodStart(date string) (string, SummaryPeriod, error) {
	day, err := time.Parse(periodDateLayout, date)
	if err != nil {
		return "", SummaryPeriodDay, err
	}
	period := c.summaryPeriodFor(day)
	return periodStart(period, day).Format(periodDateLayout), period, nil
}

// LatestEndedPeriod 返回截至 lastDay（含）最近一个已经结束的周期的第一天和周期类型。
func (c Chat) LatestEndedPeriod(lastDay string) (string, SummaryPeriod, error) {
	day, err := time.Parse(periodDateLayout, lastDay)
	if err != nil {
		return "", SummaryPeriodDay, err
	}
	period := c.summaryPeriodFor(day)
	start := periodStart(period, day)
	if periodEnd(period, start).After(day) {
		day = start.AddDate(0, 0, -1)
		period = c.summaryPeriodFor(day)
		start = periodStart(period, day)
	}
	return start.Format(periodDateLayout), period, nil
}

// ChangeSummaryFrequency 切换摘要频率：当前这一期按原频率完成，
// 新频率从此后第一个完整周期开始生效，today 为摘要时区中的今天。频率未变时保留尚未生效的切换。
func (c *Chat) ChangeSummaryFrequency(next SummaryPeriod, today string) error {
	next = NormalizeSummaryPeriod(next)
	day, err := time.Parse(periodDateLayout, today)
	if err != nil {
		return err
	}
	if next == NormalizeSummaryPeriod(c.SummaryFrequency) {
		return nil
	}
	current := c.summaryPeriodFor(day)
	boundary := periodEnd(current, periodStart(current, day)).AddDate(0, 0, 1)
	effective := periodStart(next, boundary)
	if effective.Before(boundary) {
		effective = periodEnd(next, effective).AddDate(0, 0, 1)
	}
	c.PreviousSummaryFrequency = current
	c.SummaryFrequency = next
	c.SummaryFrequencyEffectiveDate = effective.Format(periodDateLayout)
	return nil
}

// FillPeriodEnd 按周期补全 PeriodEnd，读取摘要后调用。
func (s *Summary) FillPeriodEnd() {
	s.Period = NormalizeSummaryPeriod(s.Period)
	s.PeriodEnd = s.SummaryDate
	if end, err := PeriodEndDate(s.Period, s.SummaryDate); err == nil {
		s.PeriodEnd = end
	}
}

func periodStart(period SummaryPeriod, day time.Time) time.Time {
	switch period {
	case SummaryPeriodWeek:
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	case SummaryPeriodMonth:
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return day
	}
}

func periodEnd(period SummaryPeriod, start time.Time) time.Time {
	switch period {
	case SummaryPeriodWeek:
		return start.AddDate(0, 0, 6)
	case SummaryPeriodMonth:
		return start.AddDate(0, 1, -1)
	default:
		return start
	}
}
