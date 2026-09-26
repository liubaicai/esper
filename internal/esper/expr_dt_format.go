package esper

import (
	"fmt"

	"strings"
	"time"
)

// This file implements Esper's format() datetime chain method: a terminal
// reformat that renders a date-time expression as a string. Unlike the calop
// transforms it is not rep-preserving — the result type is always
// Expression[string].
//
// Java splits format() into two forges:
//
//   - format() with no arguments renders through the default formatter of the
//     input representation family: legacy reps (long/Date/Calendar) use a new
//     SimpleDateFormat() (SHORT/SHORT date+time under the default locale),
//     LocalDateTime uses DateTimeFormatter.ISO_DATE_TIME, and ZonedDateTime
//     uses DateTimeFormatter.ISO_ZONED_DATE_TIME.
//   - format(pattern) accepts a constant java.text.SimpleDateFormat or
//     java.time.format.DateTimeFormatter pattern string.
//
// Go has exactly two modeled representations (int64 epoch-millis, which
// covers the legacy family, and time.Time, which covers both Java-8 reps).
// The Java-8 split is therefore exposed as two Go functions,
// DateTimeFormatISO / DateTimeFormatISOZoned, and the caller picks the one
// matching the modeled Java representation.

// dateTimeEraSentinel marks a translated Java 'G' era field inside a Go
// layout. Go layouts cannot render eras, so the formatter post-processes the
// rendered string and substitutes the era derived from the civil year. The
// sentinel is emitted only as a dedicated pattern segment — quoted literals
// never reach a layout string, so they cannot inject it.
const dateTimeEraSentinel = "\x00ERA\x00"

// dateTimeFormatSegment is one renderable piece: either a Go layout fragment
// (rendered through time.Format) or a verbatim literal (Java single-quoted
// text). Keeping literals out of layouts prevents time.Format from
// re-tokenizing them (a literal 'Jan' would render the month).
type dateTimeFormatSegment struct {
	text    string
	literal bool
}

const (
	dateTimeLegacyFormatDefault = "1/2/06, 3:04 PM"
	dateTimeLegacyDateInstance  = "Jan 2, 2006"
	dateTimeFormatISO           = "2006-01-02T15:04:05.999999999"
	dateTimeFormatISOZoned      = "2006-01-02T15:04:05.999999999Z07:00"
)

func layoutSegment(text string) dateTimeFormatSegment { return dateTimeFormatSegment{text: text} }
func literalSegment(text string) dateTimeFormatSegment {
	return dateTimeFormatSegment{text: text, literal: true}
}

func renderDateTimeSegments(t time.Time, location *time.Location, zoned bool, segments []dateTimeFormatSegment) string {
	var b strings.Builder
	for _, segment := range segments {
		if segment.literal {
			b.WriteString(segment.text)
			continue
		}
		rendered := t.Format(segment.text)
		if strings.Contains(rendered, dateTimeEraSentinel) {
			era := "AD"
			if t.Year() < 1 {
				era = "BC"
			}
			rendered = strings.ReplaceAll(rendered, dateTimeEraSentinel, era)
		}
		b.WriteString(rendered)
	}
	if zoned {
		b.WriteString("[" + location.String() + "]")
	}
	return b.String()
}

// DateTimeFormatDefault renders the legacy-family default format
// (SimpleDateFormat's SHORT/SHORT default under the pinned en_US locale,
// e.g. "5/30/02, 9:00 AM"). Null or missing input produces Null.
func DateTimeFormatDefault[V int64 | time.Time](value Expression[V]) Expression[string] {
	return dateTimeFormat[V]("date-time-format-default", value, []dateTimeFormatSegment{layoutSegment(dateTimeLegacyFormatDefault)}, false, "default")
}

// DateTimeFormatDateInstance renders Java SimpleDateFormat.getDateInstance()
// under the pinned en_US locale (medium date "MMM d, yyyy" with an
// unpadded day, e.g. "May 5, 2002"). Null or missing input produces Null.
func DateTimeFormatDateInstance[V int64 | time.Time](value Expression[V]) Expression[string] {
	return dateTimeFormat[V]("date-time-format-date-instance", value, []dateTimeFormatSegment{layoutSegment(dateTimeLegacyDateInstance)}, false, "date-instance")
}

// DateTimeFormatISO renders the input through the ISO_DATE_TIME formatter
// (Java LocalDateTime.format(): "2002-05-30T09:00:00", optional fraction).
// Null or missing input produces Null.
func DateTimeFormatISO[V int64 | time.Time](value Expression[V]) Expression[string] {
	return dateTimeFormat[V]("date-time-format-iso", value, []dateTimeFormatSegment{layoutSegment(dateTimeFormatISO)}, false, "ISO_DATE_TIME")
}

// DateTimeFormatISOZoned renders the input through the ISO_ZONED_DATE_TIME
// formatter (Java ZonedDateTime.format():
// "2002-05-30T09:00:00Z[UTC]" in UTC). Null or missing input produces Null.
func DateTimeFormatISOZoned[V int64 | time.Time](value Expression[V]) Expression[string] {
	return dateTimeFormat[V]("date-time-format-iso-zoned", value, []dateTimeFormatSegment{layoutSegment(dateTimeFormatISOZoned)}, true, "ISO_ZONED_DATE_TIME")
}

// DateTimeFormatPattern renders the input through a constant Java
// SimpleDateFormat/DateTimeFormatter pattern. Supported pattern letters are
// G y u M d E a h H m s S z Z X x with Java's run-length semantics where Go
// layouts can express them; single-quoted literals ('at') render verbatim,
// and ” is an apostrophe. An unknown letter, an unsupported run length, or
// malformed quoting is a build-time configuration error, matching Java's
// constant-format validation. Null or missing input produces Null.
func DateTimeFormatPattern[V int64 | time.Time](value Expression[V], pattern string) Expression[string] {
	segments, err := dateTimeJavaPatternToSegments(pattern)
	if err != nil {
		return dateTimeFormatError[V](fmt.Sprintf("invalid date-time format pattern '%s': %s", pattern, errorMessage(err)))
	}
	return dateTimeFormat[V]("date-time-format-pattern", value, segments, false, fmt.Sprintf("'%s'", pattern))
}

func dateTimeFormatError[V int64 | time.Time](message string) Expression[string] {
	node := &exprNode{
		kind:               "date-time-format",
		typ:                typeOf[string](),
		description:        "date-time-format(invalid)",
		configurationError: message,
	}
	return typedExpr[string]{n: node, fn: func(ctx EvalContext) Value { return Null() }}
}

func dateTimeFormat[V int64 | time.Time](kind string, value Expression[V], segments []dateTimeFormatSegment, zoned bool, detail string) Expression[string] {
	var children []*exprNode
	if value != nil {
		children = []*exprNode{value.node()}
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[string](),
		description: fmt.Sprintf("%s(%s,%s)", kind, expressionDescription(value), detail),
		children:    children,
	}
	if value == nil || value.node() == nil {
		node.configurationError = fmt.Sprintf("%s requires a date-time operand", kind)
	} else {
		validateDateTimeOperand(node, "value", value)
	}
	return typedExpr[string]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		instant, ok := dateTimeCalOpSplit(ctx, value.eval(ctx))
		if !ok {
			return Null()
		}
		t := time.UnixMilli(instant.millis).In(instant.location)
		return Present(renderDateTimeSegments(t, instant.location, zoned, segments))
	}}
}

// dateTimeJavaPatternToSegments translates the supported subset of Java
// SimpleDateFormat/DateTimeFormatter pattern letters to layout/literal
// segments. 'G' is emitted as a dedicated era layout segment. Letters and
// run lengths Go cannot express (unpadded hour-of-day, sub-millisecond
// counts, Java-only offset/run semantics, era-independent year 'Y',
// standalone 'L', day-of-year, week fields) produce the same build-time
// configuration error Java's validation would raise for an invalid pattern.
func dateTimeJavaPatternToSegments(pattern string) ([]dateTimeFormatSegment, error) {
	var segments []dateTimeFormatSegment
	var layout strings.Builder
	flushLayout := func() {
		if layout.Len() > 0 {
			segments = append(segments, layoutSegment(layout.String()))
			layout.Reset()
		}
	}
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		if c == '\'' {
			// literal or escaped apostrophe
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				flushLayout()
				segments = append(segments, literalSegment("'"))
				i += 2
				continue
			}
			end := strings.IndexByte(pattern[i+1:], '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated literal quote")
			}
			flushLayout()
			segments = append(segments, literalSegment(pattern[i+1:i+1+end]))
			i += end + 2
			continue
		}
		if !dateTimeJavaPatternLetter(c) {
			layout.WriteByte(c)
			i++
			continue
		}
		j := i
		for j < len(pattern) && pattern[j] == c {
			j++
		}
		run := j - i
		i = j
		switch c {
		case 'G':
			layout.WriteString(dateTimeEraSentinel)
		case 'y', 'u':
			switch run {
			case 2:
				layout.WriteString("06")
			case 4:
				layout.WriteString("2006")
			default:
				return nil, fmt.Errorf("unsupported run length %d for letter %q (Java pads years to exactly 2 or 4+ digits; Go has only '06' and '2006')", run, string(c))
			}
		case 'M':
			switch {
			case run == 1:
				layout.WriteString("1")
			case run == 2:
				layout.WriteString("01")
			case run == 3:
				layout.WriteString("Jan")
			case run == 4:
				layout.WriteString("January")
			default:
				return nil, fmt.Errorf("unsupported run length %d for letter 'M' (Java render-narrow has no Go equivalent)", run)
			}
		case 'd':
			switch {
			case run == 1:
				layout.WriteString("2")
			case run == 2:
				layout.WriteString("02")
			default:
				return nil, fmt.Errorf("unsupported run length %d for letter 'd'", run)
			}
		case 'E':
			if run >= 4 {
				layout.WriteString("Monday")
			} else {
				layout.WriteString("Mon")
			}
		case 'a':
			layout.WriteString("PM")
		case 'h':
			if run == 1 {
				layout.WriteString("3")
			} else {
				layout.WriteString("03")
			}
		case 'H':
			if run == 1 {
				return nil, fmt.Errorf("unsupported letter 'H' run length 1 (Java renders an unpadded hour; Go layouts cannot express it)")
			}
			layout.WriteString("15")
		case 'm':
			if run == 1 {
				layout.WriteString("4")
			} else {
				layout.WriteString("04")
			}
		case 's':
			if run == 1 {
				layout.WriteString("5")
			} else {
				layout.WriteString("05")
			}
		case 'S':
			if run != 3 {
				return nil, fmt.Errorf("unsupported run length %d for letter 'S' (Java renders a variable-width fraction; Go supports exactly SSS)", run)
			}
			layout.WriteString("000")
		case 'z':
			if run >= 4 {
				return nil, fmt.Errorf("unsupported run length %d for letter 'z' (Java renders the full zone name; Go layouts only abbreviate)", run)
			}
			layout.WriteString("MST")
		case 'Z':
			layout.WriteString("-0700")
		case 'X':
			switch run {
			case 1:
				layout.WriteString("Z07")
			case 2:
				layout.WriteString("Z0700")
			case 3:
				layout.WriteString("Z07:00")
			default:
				return nil, fmt.Errorf("unsupported run length %d for letter 'X'", run)
			}
		case 'x':
			switch run {
			case 1:
				layout.WriteString("-07")
			case 2:
				layout.WriteString("-0700")
			case 3:
				layout.WriteString("-07:00")
			default:
				return nil, fmt.Errorf("unsupported run length %d for letter 'x'", run)
			}
		default:
			return nil, fmt.Errorf("unsupported letter %q", string(c))
		}
	}
	flushLayout()
	return segments, nil
}

func dateTimeJavaPatternLetter(c byte) bool {
	switch c {
	case 'G', 'y', 'Y', 'u', 'D', 'M', 'L', 'd', 'Q', 'q', 'E', 'e', 'c',
		'a', 'B', 'h', 'H', 'K', 'k', 'm', 's', 'S', 'A', 'n', 'N',
		'z', 'O', 'X', 'x', 'Z', 'p', 'V', 'w', 'W', 'F', 'j':
		return true
	}
	return false
}
