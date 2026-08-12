package query

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestExtractPaginationParams_Defaults(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.DefaultLimit, params.Limit)
	assert.Equal(t, DefaultConfig.DefaultOffset, params.Offset)
}

func TestExtractPaginationParams_CustomValues(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?limit=50&offset=100", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, 50, params.Limit)
	assert.Equal(t, 100, params.Offset)
}

func TestExtractPaginationParams_LimitCappedAtMax(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?limit=1000", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.MaxLimit, params.Limit)
}

func TestExtractPaginationParams_InvalidLimit_UsesDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?limit=invalid", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.DefaultLimit, params.Limit)
}

func TestExtractPaginationParams_NegativeLimit_UsesDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?limit=-5", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.DefaultLimit, params.Limit)
}

func TestExtractPaginationParams_NegativeOffset_UsesDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?offset=-10", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.DefaultOffset, params.Offset)
}

func TestExtractPaginationParams_InvalidOffset_UsesDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?offset=abc", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.DefaultOffset, params.Offset)
}

func TestExtractPaginationParams_CustomConfig(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?limit=500", nil)

	config := PaginationConfig{
		DefaultLimit:  10,
		MaxLimit:      50,
		DefaultOffset: 0,
	}

	params := ExtractPaginationParamsWithConfig(c, config)

	// Should cap at custom max (50), not default (100)
	assert.Equal(t, 50, params.Limit)
}

func TestExtractPaginationParams_OnlyLimit(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?limit=30", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, 30, params.Limit)
	assert.Equal(t, 0, params.Offset)
}

func TestExtractPaginationParams_OnlyOffset(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data?offset=50", nil)

	params := ExtractPaginationParams(c)

	assert.Equal(t, DefaultConfig.DefaultLimit, params.Limit)
	assert.Equal(t, 50, params.Offset)
}

func TestMaxLimitHelper_BelowMax(t *testing.T) {
	result := MaxLimitHelper(50, 100)
	assert.Equal(t, 50, result)
}

func TestMaxLimitHelper_AboveMax(t *testing.T) {
	result := MaxLimitHelper(200, 100)
	assert.Equal(t, 100, result)
}

func TestMaxLimitHelper_ExactlyMax(t *testing.T) {
	result := MaxLimitHelper(100, 100)
	assert.Equal(t, 100, result)
}

func TestMaxLimitHelper_ZeroRequested(t *testing.T) {
	result := MaxLimitHelper(0, 100)
	assert.Equal(t, DefaultConfig.DefaultLimit, result)
}

func TestMaxLimitHelper_NegativeRequested(t *testing.T) {
	result := MaxLimitHelper(-5, 100)
	assert.Equal(t, DefaultConfig.DefaultLimit, result)
}

// Standard pagination flow test
func TestStandardPaginationFlow(t *testing.T) {
	testCases := []struct {
		name           string
		queryString    string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "Page 1",
			queryString:    "?limit=10&offset=0",
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "Page 2",
			queryString:    "?limit=10&offset=10",
			expectedLimit:  10,
			expectedOffset: 10,
		},
		{
			name:           "Page 3",
			queryString:    "?limit=10&offset=20",
			expectedLimit:  10,
			expectedOffset: 20,
		},
		{
			name:           "Large limit capped",
			queryString:    "?limit=1500&offset=0",
			expectedLimit:  1000, // capped at max limit
			expectedOffset: 0,
		},
		{
			name:           "No pagination params",
			queryString:    "",
			expectedLimit:  20, // default
			expectedOffset: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/api/data"+tc.queryString, nil)

			params := ExtractPaginationParams(c)

			assert.Equal(t, tc.expectedLimit, params.Limit)
			assert.Equal(t, tc.expectedOffset, params.Offset)
		})
	}
}
