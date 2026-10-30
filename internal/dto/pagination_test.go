package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginationRequest_Defaults(t *testing.T) {
	tests := []struct {
		name       string
		req        PaginationRequest
		wantPage   int64
		wantLimit  int64
		wantOffset int64
	}{
		{
			name:       "未设置",
			wantPage:   DefaultPage,
			wantLimit:  DefaultLimit,
			wantOffset: 0,
		},
		{
			name:       "自定义",
			req:        PaginationRequest{Page: 2, Limit: 20},
			wantPage:   2,
			wantLimit:  20,
			wantOffset: 20,
		},
		{
			name:       "零值用默认",
			req:        PaginationRequest{Page: 0, Limit: 0},
			wantPage:   DefaultPage,
			wantLimit:  DefaultLimit,
			wantOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantPage, tt.req.GetPage())
			assert.Equal(t, tt.wantLimit, tt.req.GetLimit())
			assert.Equal(t, tt.wantOffset, tt.req.GetOffset())
		})
	}
}

func TestPaginationRequest_GetTotalPages(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		limit     int64
		wantPages int64
	}{
		{name: "整除", total: 100, limit: 10, wantPages: 10},
		{name: "有余数", total: 95, limit: 10, wantPages: 10},
		{name: "总数为0", total: 0, limit: 10, wantPages: 0},
		{name: "limit 未设置用默认", total: 10, limit: 0, wantPages: 1},
		{name: "总数小于limit", total: 5, limit: 10, wantPages: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PaginationRequest{Limit: tt.limit}
			assert.Equal(t, tt.wantPages, req.GetTotalPages(tt.total))
		})
	}
}
