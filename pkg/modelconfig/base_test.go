package modelconfig

import "testing"

func TestFormatParamCount(t *testing.T) {
	testCases := []struct {
		count    int64
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{999, "999"},
		{1000, "1K"},
		{1500, "1.5K"},
		{10000, "10K"},
		{10500, "10.5K"},
		{150000, "150K"},
		{151500, "151.5K"},
		{1000000, "1M"},
		{1500000, "1.5M"},
		{10000000, "10M"},
		{10500000, "10.5M"},
		{150000000, "150M"},
		{151500000, "151.5M"},
		{1000000000, "1B"},
		{1500000000, "1.5B"},
		{10000000000, "10B"},
		{10500000000, "10.5B"},
		{150000000000, "150B"},
		{151500000000, "151.5B"},
		{685000000000, "685B"},
		{1000000000000, "1T"},
		{1500000000000, "1.5T"},
		{1512300000000, "1.51T"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := FormatParamCount(tc.count)
			if result != tc.expected {
				t.Errorf("FormatParamCount(%d) = %s, expected %s", tc.count, result, tc.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{999, "999 B"},
		{1000, "1000 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 10, "10.00 KB"},
		{1024*1024 - 1, "1024.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1.5, "1.50 MB"},
		{1024 * 1024 * 10, "10.00 MB"},
		{1024*1024*1024 - 1, "1024.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1.5, "1.50 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{1024 * 1024 * 1024 * 1024 * 1.5, "1.50 TB"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := FormatSize(tc.bytes)
			if result != tc.expected {
				t.Errorf("FormatSize(%d) = %s, expected %s", tc.bytes, result, tc.expected)
			}
		})
	}
}
