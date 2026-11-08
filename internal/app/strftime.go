package app

import (
	"fmt"
	"strings"
	"time"
)

// strftime formats a time using the strftime directives supported by GNU date.
func strftime(format string, value time.Time) string {
	var result strings.Builder

	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			result.WriteByte(format[i])
			continue
		}

		i++
		switch format[i] {
		case '%':
			result.WriteByte('%')
		case 'Y':
			result.WriteString(value.Format("2006"))
		case 'y':
			result.WriteString(value.Format("06"))
		case 'C':
			result.WriteString(fmt.Sprintf("%02d", value.Year()/100))
		case 'm':
			result.WriteString(value.Format("01"))
		case 'd':
			result.WriteString(value.Format("02"))
		case 'e':
			result.WriteString(fmt.Sprintf("%2d", value.Day()))
		case 'H':
			result.WriteString(value.Format("15"))
		case 'I':
			result.WriteString(value.Format("03"))
		case 'M':
			result.WriteString(value.Format("04"))
		case 'S':
			result.WriteString(value.Format("05"))
		case 'p':
			result.WriteString(value.Format("PM"))
		case 'a':
			result.WriteString(value.Format("Mon"))
		case 'A':
			result.WriteString(value.Format("Monday"))
		case 'b':
			result.WriteString(value.Format("Jan"))
		case 'B':
			result.WriteString(value.Format("January"))
		case 'j':
			result.WriteString(fmt.Sprintf("%03d", value.YearDay()))
		case 'u':
			weekday := int(value.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			result.WriteString(fmt.Sprintf("%d", weekday))
		case 'w':
			result.WriteString(fmt.Sprintf("%d", int(value.Weekday())))
		case 's':
			result.WriteString(fmt.Sprintf("%d", value.Unix()))
		case 'N':
			result.WriteString(fmt.Sprintf("%09d", value.Nanosecond()))
		case 'z':
			result.WriteString(value.Format("-0700"))
		case 'Z':
			result.WriteString(value.Format("MST"))
		case 'F':
			result.WriteString(value.Format("2006-01-02"))
		case 'T':
			result.WriteString(value.Format("15:04:05"))
		case 'R':
			result.WriteString(value.Format("15:04"))
		case 'D':
			result.WriteString(value.Format("01/02/06"))
		case 'n':
			result.WriteByte('\n')
		case 't':
			result.WriteByte('\t')
		default:
			result.WriteByte('%')
			result.WriteByte(format[i])
		}
	}

	return result.String()
}
