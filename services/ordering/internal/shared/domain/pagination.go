package domain

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Pagination is an offset-based pagination value object.
type Pagination struct {
	Page     int
	PageSize int
}

// NewPagination clamps page (min 1) and pageSize (1..100, default 20).
func NewPagination(page, pageSize int) Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return Pagination{Page: page, PageSize: pageSize}
}

func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }
func (p Pagination) Limit() int  { return p.PageSize }
