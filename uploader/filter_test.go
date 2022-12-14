package uploader

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/TrueCloudLab/frostfs-api-go/v2/object"
	"github.com/TrueCloudLab/frostfs-http-gw/utils"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestFilter(t *testing.T) {
	log := zap.NewNop()

	t.Run("duplicate keys error", func(t *testing.T) {
		req := &fasthttp.RequestHeader{}
		req.DisableNormalizing()
		req.Add("X-Attribute-DupKey", "first-value")
		req.Add("X-Attribute-DupKey", "second-value")
		_, err := filterHeaders(log, req)
		require.Error(t, err)
	})

	t.Run("duplicate system keys error", func(t *testing.T) {
		req := &fasthttp.RequestHeader{}
		req.DisableNormalizing()
		req.Add("X-Attribute-Neofs-DupKey", "first-value")
		req.Add("X-Attribute-Neofs-DupKey", "second-value")
		_, err := filterHeaders(log, req)
		require.Error(t, err)
	})

	req := &fasthttp.RequestHeader{}
	req.DisableNormalizing()

	req.Set("X-Attribute-Neofs-Expiration-Epoch1", "101")
	req.Set("X-Attribute-NEOFS-Expiration-Epoch2", "102")
	req.Set("X-Attribute-neofs-Expiration-Epoch3", "103")
	req.Set("X-Attribute-MyAttribute", "value")

	expected := map[string]string{
		"__NEOFS__EXPIRATION_EPOCH1": "101",
		"MyAttribute":                "value",
		"__NEOFS__EXPIRATION_EPOCH3": "103",
		"__NEOFS__EXPIRATION_EPOCH2": "102",
	}

	result, err := filterHeaders(log, req)
	require.NoError(t, err)

	require.Equal(t, expected, result)
}

func TestPrepareExpirationHeader(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	tomorrowUnix := tomorrow.Unix()
	tomorrowUnixNano := tomorrow.UnixNano()
	tomorrowUnixMilli := tomorrowUnixNano / 1e6

	epoch := "100"
	duration := "24h"
	timestampSec := strconv.FormatInt(tomorrowUnix, 10)
	timestampMilli := strconv.FormatInt(tomorrowUnixMilli, 10)
	timestampNano := strconv.FormatInt(tomorrowUnixNano, 10)

	defaultDurations := &epochDurations{
		currentEpoch:  10,
		msPerBlock:    1000,
		blockPerEpoch: 101,
	}

	msPerBlock := defaultDurations.blockPerEpoch * uint64(defaultDurations.msPerBlock)
	epochPerDay := uint64((24 * time.Hour).Milliseconds()) / msPerBlock
	if uint64((24*time.Hour).Milliseconds())%msPerBlock != 0 {
		epochPerDay++
	}

	defaultExpEpoch := strconv.FormatUint(defaultDurations.currentEpoch+epochPerDay, 10)

	for _, tc := range []struct {
		name      string
		headers   map[string]string
		durations *epochDurations
		err       bool
		expected  map[string]string
	}{
		{
			name:     "valid epoch",
			headers:  map[string]string{object.SysAttributeExpEpoch: epoch},
			expected: map[string]string{object.SysAttributeExpEpoch: epoch},
		},
		{
			name: "valid epoch, valid duration",
			headers: map[string]string{
				object.SysAttributeExpEpoch:  epoch,
				utils.ExpirationDurationAttr: duration,
			},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: epoch},
		},
		{
			name: "valid epoch, valid rfc3339",
			headers: map[string]string{
				object.SysAttributeExpEpoch: epoch,
				utils.ExpirationRFC3339Attr: tomorrow.Format(time.RFC3339),
			},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: epoch},
		},
		{
			name: "valid epoch, valid timestamp sec",
			headers: map[string]string{
				object.SysAttributeExpEpoch:   epoch,
				utils.ExpirationTimestampAttr: timestampSec,
			},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: epoch},
		},
		{
			name: "valid epoch, valid timestamp milli",
			headers: map[string]string{
				object.SysAttributeExpEpoch:   epoch,
				utils.ExpirationTimestampAttr: timestampMilli,
			},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: epoch},
		},
		{
			name: "valid epoch, valid timestamp nano",
			headers: map[string]string{
				object.SysAttributeExpEpoch:   epoch,
				utils.ExpirationTimestampAttr: timestampNano,
			},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: epoch},
		},
		{
			name:      "valid timestamp sec",
			headers:   map[string]string{utils.ExpirationTimestampAttr: timestampSec},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: defaultExpEpoch},
		},
		{
			name:      "valid duration",
			headers:   map[string]string{utils.ExpirationDurationAttr: duration},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: defaultExpEpoch},
		},
		{
			name:      "valid rfc3339",
			headers:   map[string]string{utils.ExpirationRFC3339Attr: tomorrow.Format(time.RFC3339)},
			durations: defaultDurations,
			expected:  map[string]string{object.SysAttributeExpEpoch: defaultExpEpoch},
		},
		{
			name:    "valid max uint 64",
			headers: map[string]string{utils.ExpirationRFC3339Attr: tomorrow.Format(time.RFC3339)},
			durations: &epochDurations{
				currentEpoch:  math.MaxUint64 - 1,
				msPerBlock:    defaultDurations.msPerBlock,
				blockPerEpoch: defaultDurations.blockPerEpoch,
			},
			expected: map[string]string{object.SysAttributeExpEpoch: strconv.FormatUint(uint64(math.MaxUint64), 10)},
		},
		{
			name:    "invalid timestamp sec",
			headers: map[string]string{utils.ExpirationTimestampAttr: "abc"},
			err:     true,
		},
		{
			name:    "invalid timestamp sec zero",
			headers: map[string]string{utils.ExpirationTimestampAttr: "0"},
			err:     true,
		},
		{
			name:    "invalid duration",
			headers: map[string]string{utils.ExpirationDurationAttr: "1d"},
			err:     true,
		},
		{
			name:    "invalid duration negative",
			headers: map[string]string{utils.ExpirationDurationAttr: "-5h"},
			err:     true,
		},
		{
			name:    "invalid rfc3339",
			headers: map[string]string{utils.ExpirationRFC3339Attr: "abc"},
			err:     true,
		},
		{
			name:    "invalid rfc3339 zero",
			headers: map[string]string{utils.ExpirationRFC3339Attr: time.RFC3339},
			err:     true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := prepareExpirationHeader(tc.headers, tc.durations, time.Now())
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, tc.headers)
			}
		})
	}
}
