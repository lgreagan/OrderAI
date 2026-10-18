package models

import (
	"database/sql/driver"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Date 是只精确到天的日期类型：JSON 用 yyyy-MM-dd，兼容 MySQL DATE 列。
type Date struct {
	time.Time
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format(dateLayout) + `"`), nil
}

func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		d.Time = time.Time{}
		return nil
	}
	if t, err := time.Parse(dateLayout, s); err == nil {
		d.Time = t
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// Value 让 Date 可以直接作为 SQL 参数写入 DATE 列。
func (d Date) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Time.Format(dateLayout), nil
}

func (d *Date) Scan(value any) error {
	if value == nil {
		d.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		d.Time = v
	case []byte:
		return d.parse(string(v))
	case string:
		return d.parse(v)
	}
	return nil
}

func (d *Date) parse(s string) error {
	if t, err := time.Parse(dateLayout, s); err == nil {
		d.Time = t
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}
