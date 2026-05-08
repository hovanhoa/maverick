package date

import "time"

func ParseDate(date string) (*time.Time, error) {
	parseFormats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, _2 Jan 2006 15:04:05 -0700",
		"_2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		"Mon, Jan _2, 2006 at 3:04 PM",
		"Mon, Jan 02, 2006 at 3:04 PM",
		"Mon, Jan _2, 2006 at 03:04 PM",
		"Mon, Jan 02, 2006 at 03:04 PM",
		"Mon, _2 Jan 2006 3:04 PM",
		"Mon, 02 Jan 2006 3:04 PM",
		"Mon, _2 Jan 2006 03:04 PM",
		"Mon, 02 Jan 2006 03:04 PM",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	var sentAt time.Time
	var err error
	for _, parseFormat := range parseFormats {
		sentAt, err = time.Parse(parseFormat, date)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	return &sentAt, nil
}
