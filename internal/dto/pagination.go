package dto

const (
	DefaultPage  int64 = 1
	DefaultLimit int64 = 10
)

// PaginationRequest 分页请求参数。
// 范围由 binding tag 校验；未传或为 0 时用默认值。
type PaginationRequest struct {
	Page  int64 `form:"page" binding:"omitempty,min=1,max=1000000"`
	Limit int64 `form:"limit" binding:"omitempty,min=1,max=100"`
}

// GetPage 获取页码，未设置时返回 DefaultPage。
func (p *PaginationRequest) GetPage() int64 {
	if p.Page < 1 {
		return DefaultPage
	}
	return p.Page
}

// GetLimit 获取每页数量，未设置时返回 DefaultLimit。
func (p *PaginationRequest) GetLimit() int64 {
	if p.Limit < 1 {
		return DefaultLimit
	}
	return p.Limit
}

// GetOffset 获取偏移量。
func (p *PaginationRequest) GetOffset() int64 {
	return (p.GetPage() - 1) * p.GetLimit()
}

// GetTotalPages 计算总页数。
func (p *PaginationRequest) GetTotalPages(total int64) int64 {
	if total == 0 {
		return 0
	}
	limit := p.GetLimit()
	pages := total / limit
	if total%limit > 0 {
		pages++
	}
	return pages
}
