package rag

import "testing"

func TestXLSXDisplaysPreserveRawNumbersAndExplicitFormats(t *testing.T) {
	for _, tc := range []struct {
		raw, format, want, state string
		date1904                 bool
	}{
		{"1234.567", `"USD "#,##0.00`, "USD 1,234.57", "format_derived", false},
		{"99999999999999.995", "0.00", "100000000000000.00", "format_derived", false},
		{"0.125", "0.0%", "12.5%", "format_derived", false},
		{"-1234.5", "#,##0.00;[Red](#,##0.00)", "(1,234.50)", "format_derived", false},
		{"12", "00000", "00012", "format_derived", false},
		{"123456789", "0.00,,", "123.46", "format_derived", false},
		{"1234", "0.00E+00", "1.23E+03", "format_derived", false},
		{"0.009999", "0.00E+00", "1.00E-02", "format_derived", false},
		{"45292", "yyyy-mm-dd", "2024-01-01", "format_derived", false},
		{"0", "yyyy-mm-dd", "1904-01-01", "format_derived", true},
		{"60", "yyyy-mm-dd", "1900-02-29", "excel_1900_leap_day", false},
		{"61", "yyyy-mm-dd", "1900-03-01", "format_derived", false},
		{"0.5", "h:mm AM/PM", "12:00 PM", "format_derived", false},
		{"45292.5", "yyyy-mm-dd hh:mm:ss", "2024-01-01 12:00:00", "format_derived", false},
		{"45292", "[$-804]yyyy\"年\"m\"月\"d\"日\"", "45292", "unrendered", false},
		{"1.5", "# ?/?", "1.5", "unrendered", false},
		{"0", "0;-0;\"none\"", "none", "format_derived", false},
		{"17", ";;;", "", "format_hidden", false},
		{"1e9", "General", "1e9", "stored", false},
	} {
		t.Run(tc.raw+"/"+tc.format, func(t *testing.T) {
			display, state := xlsxDisplayNumber(tc.raw, tc.format, tc.date1904)
			if display != tc.want || state != tc.state {
				t.Fatalf("got %q (%s), want %q (%s)", display, state, tc.want, tc.state)
			}
		})
	}
}

func TestXLSXNumericWorkIsBounded(t *testing.T) {
	for _, raw := range []string{"NaN", "Inf", "1/3", "1e999999999", "1e-999999999", "abc"} {
		if _, ok := xlsxRational(raw); ok {
			t.Fatal("unbounded or invalid numeric input", raw)
		}
	}
}
